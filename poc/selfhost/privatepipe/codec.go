// Package privatepipe is for inherited anonymous pipes created by the trusted
// local parent, never a TCP/named public socket. It is not peer authentication.
package privatepipe

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
)

var ErrProtocol = errors.New("private_protocol_denied")

type Reader struct {
	reader *bufio.Reader
	limit  int
}

func NewReader(r io.Reader) *Reader { return NewBoundedReader(r, 8192) }

// Larger framing is reserved for local journal blobs, not network messages.
// Existing callers retain the original 8KiB limit and strict JSON parsing.
func NewBoundedReader(r io.Reader, limit int) *Reader {
	if limit < 1 || limit > 16384 {
		limit = 8192
	}
	return &Reader{bufio.NewReaderSize(r, limit+2), limit}
}
func (r *Reader) Read(v any) error {
	line, e := r.reader.ReadSlice('\n')
	if e == io.EOF && len(line) == 0 {
		return io.EOF
	}
	if e != nil || len(line) > r.limit {
		return ErrProtocol
	}
	// encoding/json normally accepts duplicate keys. Reject at every nested
	// object before typed decoding, preventing divergent command interpretation.
	d := json.NewDecoder(bytes.NewReader(line))
	if unique(d) != nil {
		return ErrProtocol
	}
	if _, e = d.Token(); e != io.EOF {
		return ErrProtocol
	}
	d = json.NewDecoder(bytes.NewReader(line))
	d.DisallowUnknownFields()
	if d.Decode(v) != nil {
		return ErrProtocol
	}
	return nil
}
func unique(d *json.Decoder) error {
	t, e := d.Token()
	if e != nil {
		return e
	}
	delim, ok := t.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]bool{}
		for d.More() {
			k, e := d.Token()
			if e != nil {
				return e
			}
			s, ok := k.(string)
			s = strings.ToLower(s)
			if !ok || seen[s] {
				return ErrProtocol
			}
			seen[s] = true
			if unique(d) != nil {
				return ErrProtocol
			}
		}
	case '[':
		for d.More() {
			if unique(d) != nil {
				return ErrProtocol
			}
		}
	default:
		return ErrProtocol
	}
	_, e = d.Token()
	return e
}
