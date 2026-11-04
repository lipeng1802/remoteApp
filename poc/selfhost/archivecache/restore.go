// Package archivecache reconstructs TEST transport bytes. It grants no trust,
// loads no profile, opens no native identity and never executes a cached file.
package archivecache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

var ErrArchive = errors.New("archive_cache_rejected")

type File struct {
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}
type Target struct {
	File
	// Source 0 is the literal stream, source N is Sources[N-1].
	Steps []Step `json:"steps"`
}
type Step [3]int64

func (s *Step) UnmarshalJSON(raw []byte) error {
	var values []int64
	if json.Unmarshal(raw, &values) != nil || len(values) != 3 {
		return ErrArchive
	}
	copy(s[:], values)
	return nil
}

type Plan struct {
	Version int      `json:"version"`
	Block   int64    `json:"block"`
	Literal File     `json:"literal"`
	Sources []File   `json:"sources"`
	Targets []Target `json:"targets"`
}

const maximum = 256 << 20

func regular(p string) (int64, error) {
	s, err := os.Lstat(p)
	if err != nil || !s.Mode().IsRegular() {
		return 0, ErrArchive
	}
	return s.Size(), nil
}
func path(root, relative string, create bool) (string, error) {
	if relative == "" || strings.ContainsAny(relative, "\\:") || strings.HasPrefix(relative, "_") {
		return "", ErrArchive
	}
	parts := strings.Split(relative, "/")
	p := root
	for i, part := range parts {
		if part == "" || part == "." || part == ".." || strings.Trim(part, " .") != part {
			return "", ErrArchive
		}
		device := strings.ToUpper(strings.SplitN(part, ".", 2)[0])
		if device == "CON" || device == "PRN" || device == "AUX" || device == "NUL" || len(device) == 4 && (strings.HasPrefix(device, "COM") || strings.HasPrefix(device, "LPT")) && device[3] >= '1' && device[3] <= '9' {
			return "", ErrArchive
		}
		for _, c := range part {
			if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '.' || c == '-' || c == '_') {
				return "", ErrArchive
			}
		}
		p = filepath.Join(p, part)
		if i == len(parts)-1 {
			break
		}
		s, err := os.Lstat(p)
		if os.IsNotExist(err) && create {
			err = os.Mkdir(p, 0700)
			if err == nil {
				s, err = os.Lstat(p)
			}
		}
		if err != nil || !s.IsDir() || s.Mode()&os.ModeSymlink != 0 {
			return "", ErrArchive
		}
	}
	return p, nil
}
func rootOK(root string) bool {
	if !filepath.IsAbs(root) || filepath.Clean(root) != root || filepath.Dir(root) == root {
		return false
	}
	for p := root; ; p = filepath.Dir(p) {
		s, err := os.Lstat(p)
		if err != nil || !s.IsDir() || s.Mode()&os.ModeSymlink != 0 {
			return false
		}
		if filepath.Dir(p) == p {
			return true
		}
	}
}
func specOK(f File, empty bool) bool {
	d, err := hex.DecodeString(f.SHA256)
	return err == nil && len(d) == 32 && hex.EncodeToString(d) == f.SHA256 && f.Size >= 0 && f.Size <= maximum && (empty || f.Size > 0)
}
func digest(f *os.File, want File) bool {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return false
	}
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(f, maximum+1))
	return err == nil && n == want.Size && hex.EncodeToString(h.Sum(nil)) == want.SHA256
}
func open(p string, want File) (*os.File, error) {
	if n, err := regular(p); err != nil || n != want.Size {
		return nil, ErrArchive
	}
	f, err := os.Open(p)
	if err != nil {
		return nil, ErrArchive
	}
	if !digest(f, want) {
		f.Close()
		return nil, ErrArchive
	}
	return f, nil
}

// Restore requires the trusted coordinator to verify the complete incoming
// archive first. Cache changes, traversal, overwrites and incomplete copies fail
// closed. An unknown/partial output is preserved and must never be retried.
func Restore(base, output, wire string) (result error) {
	defer func() {
		if result != nil {
			result = ErrArchive
		}
	}()
	if !rootOK(base) || !rootOK(output) || !rootOK(wire) || base == output || filepath.Dir(wire) != output || filepath.Base(wire) != "_transport" {
		return ErrArchive
	}
	p := filepath.Join(wire, "plan.json")
	if n, err := regular(p); err != nil || n > 4<<20 {
		return ErrArchive
	}
	f, err := os.Open(p)
	if err != nil {
		return err
	}
	defer f.Close()
	decoder := json.NewDecoder(io.LimitReader(f, (4<<20)+1))
	decoder.DisallowUnknownFields()
	var plan Plan
	if decoder.Decode(&plan) != nil || decoder.Decode(new(any)) != io.EOF || plan.Version != 1 || plan.Block != 32768 || plan.Literal.Name != "literal.bin" || !specOK(plan.Literal, true) || len(plan.Sources) > 64 || len(plan.Targets) == 0 || len(plan.Targets) > 64 {
		return ErrArchive
	}
	inputs := make([]*os.File, 0, len(plan.Sources)+1)
	defer func() {
		for _, f := range inputs {
			if f.Close() != nil {
				result = ErrArchive
			}
		}
	}()
	literal, err := open(filepath.Join(wire, "literal.bin"), plan.Literal)
	if err != nil {
		return err
	}
	inputs = append(inputs, literal)
	sizes := []int64{plan.Literal.Size}
	seen := map[string]bool{}
	var inputBytes int64
	for _, s := range plan.Sources {
		if !specOK(s, false) || !strings.HasSuffix(s.Name, ".exe") || seen[strings.ToLower(s.Name)] {
			return ErrArchive
		}
		seen[strings.ToLower(s.Name)] = true
		inputBytes += s.Size
		if inputBytes > maximum {
			return ErrArchive
		}
		p, err := path(base, s.Name, false)
		if err != nil {
			return err
		}
		f, err := open(p, s)
		if err != nil {
			return err
		}
		inputs = append(inputs, f)
		sizes = append(sizes, s.Size)
	}
	seen = map[string]bool{}
	var total int64
	// Validate the whole plan before creating any output executable.
	for _, t := range plan.Targets {
		if !specOK(t.File, false) || !strings.HasSuffix(t.Name, ".exe") || seen[strings.ToLower(t.Name)] || len(t.Steps) == 0 || len(t.Steps) > 8192 {
			return ErrArchive
		}
		seen[strings.ToLower(t.Name)] = true
		var size int64
		for _, step := range t.Steps {
			source, offset, length := step[0], step[1], step[2]
			if source < 0 || source >= int64(len(inputs)) || offset < 0 || length < 1 || length > plan.Block || offset > sizes[source]-length {
				return ErrArchive
			}
			size += length
		}
		total += size
		if size != t.Size || total > maximum {
			return ErrArchive
		}
		p, err := path(output, t.Name, true)
		if err != nil {
			return err
		}
		if _, err := os.Lstat(p); err == nil || !os.IsNotExist(err) {
			return ErrArchive
		}
	}
	buffer := make([]byte, plan.Block)
	for _, t := range plan.Targets {
		p, err := path(output, t.Name, true)
		if err != nil {
			return err
		}
		out, err := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0500)
		if err != nil {
			return err
		}
		copyErr := func() (result error) {
			defer func() {
				if out.Close() != nil {
					result = ErrArchive
				}
			}()
			for _, step := range t.Steps {
				b := buffer[:step[2]]
				if _, err := inputs[step[0]].ReadAt(b, step[1]); err != nil {
					return err
				}
				if n, err := out.Write(b); err != nil || n != len(b) {
					return ErrArchive
				}
			}
			if !digest(out, t.File) || out.Sync() != nil {
				return ErrArchive
			}
			return nil
		}()
		if copyErr != nil {
			return copyErr
		}
	}
	return nil
}
