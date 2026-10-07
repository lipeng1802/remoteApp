package invite

import (
	"bytes"
	"encoding/json"
	"io"

	"remoteapp.local/selfhost-poc/privatepipe"
)

// Decode a single bounded HTTP JSON document, preserving support for pretty
// JSON while rejecting duplicate/case-alias keys at every nested object.
func decodeHTTP(r io.Reader, value any) error {
	b, e := io.ReadAll(r)
	if e != nil {
		return e
	}
	var compact bytes.Buffer
	if e = json.Compact(&compact, b); e != nil {
		return e
	}
	p := privatepipe.NewReader(bytes.NewReader(append(compact.Bytes(), '\n')))
	if e = p.Read(value); e != nil {
		return e
	}
	var extra struct{}
	if p.Read(&extra) != io.EOF {
		return privatepipe.ErrProtocol
	}
	return nil
}
