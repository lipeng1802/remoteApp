// Package nativejournal is an independent OS credential namespace for signed
// configuration records. It does not access existing device/pairing keys.
package nativejournal

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"remoteapp.local/selfhost-poc/privatepipe"
)

var ErrUnavailable = errors.New("native_journal_unavailable")
var ErrMissing = errors.New("native_item_missing")
var ErrFresh = errors.New("native_installation_pending")
var ErrLogon = errors.New("interactive_logon_required")

type State string

const (
	StateAbsent    State = "absent"
	StatePending   State = "pending"
	StateCommitted State = "committed"
	StateUnusable  State = "unusable"
)

const chunkSize = 2048
const maximumRecord = 8191

var pending = []byte("explicit_installation_pending_v1")

type keyValues interface {
	Read(string) ([]byte, error)
	Write(string, []byte) error
	Remove(string) error
	Empty() (bool, error)
	ClearTest() error
	Close() error
}

type Store struct {
	kv                          keyValues
	fresh, failed, closed, test bool
}

type manifest struct {
	Version int    `json:"version"`
	Digest  string `json:"digest"`
	Size    int    `json:"size"`
}

func canonical(s string, size int) bool {
	b, e := hex.DecodeString(s)
	return e == nil && len(b) == size && hex.EncodeToString(b) == s
}
func Digest(raw []byte) string                 { h := sha256.Sum256(raw); return hex.EncodeToString(h[:]) }
func partName(digest string, index int) string { return fmt.Sprintf("data/%s/%d", digest, index) }

// Open owns the native lock until Close. Caller must run on the same OS thread
// on Windows (the dedicated helper does so), and serialize operations.
func Open(scope string, create, test bool) (*Store, error) {
	if !canonical(scope, 16) {
		return nil, ErrUnavailable
	}
	kv, e := openNative(scope, test)
	if e != nil {
		return nil, e
	}
	s := &Store{kv: kv, fresh: create, test: test}
	if create {
		empty, e := kv.Empty()
		if e != nil {
			kv.Close()
			return nil, e
		}
		if !empty {
			kv.Close()
			return nil, ErrUnavailable
		}
		if e := kv.Write("manifest", pending); e != nil {
			kv.Close()
			return nil, e
		}
		b, e := kv.Read("manifest")
		if e != nil {
			kv.Close()
			return nil, e
		}
		if !bytes.Equal(b, pending) {
			kv.Close()
			return nil, ErrUnavailable
		}
	} else if _, e := s.Load(); e != nil {
		kv.Close()
		return nil, e
	}
	return s, nil
}

// CleanupTest is restricted to the separate test namespace and exact scope.
// It can clean a corrupt test record without opening/resetting production state.
func CleanupTest(scope string) error {
	if !canonical(scope, 16) {
		return ErrUnavailable
	}
	kv, e := openNative(scope, true)
	if e != nil {
		return e
	}
	defer kv.Close()
	return clearTestVerified(kv)
}

// InspectState is read-only and returns no record bytes. Non-empty state that
// cannot be fully authenticated is deliberately collapsed to unusable: callers
// must not infer whether corruption, missing chunks or an uncertain write won.
func InspectState(scope string, test bool) (State, error) {
	if !canonical(scope, 16) {
		return "", ErrUnavailable
	}
	kv, err := openNative(scope, test)
	if err != nil {
		return "", err
	}
	defer kv.Close()
	return inspectValues(kv, test)
}

func inspectValues(kv keyValues, test bool) (State, error) {
	if kv == nil {
		return "", ErrUnavailable
	}
	empty, err := kv.Empty()
	if err != nil {
		return "", err
	}
	if empty {
		return StateAbsent, nil
	}
	manifest, err := kv.Read("manifest")
	if err == nil && bytes.Equal(manifest, pending) {
		return StatePending, nil
	}
	s := &Store{kv: kv, test: test}
	if _, err = s.read(); err != nil {
		return StateUnusable, nil
	}
	return StateCommitted, nil
}

func clearTestVerified(kv keyValues) error {
	if e := kv.ClearTest(); e != nil {
		return e
	}
	empty, e := kv.Empty()
	if e != nil {
		return e
	}
	if !empty {
		return ErrUnavailable
	}
	return nil
}

// TestEmpty inspects only TEST namespace attributes under the native lease.
// It does not return payloads, initialize records or inspect production keys.
// Caller must retain Windows OS-thread affinity for this operation.
func TestEmpty(scope string) (bool, error) {
	if !canonical(scope, 16) {
		return false, ErrUnavailable
	}
	kv, e := openNative(scope, true)
	if e != nil {
		return false, e
	}
	empty, e := kv.Empty()
	if next := kv.Close(); e == nil {
		e = next
	}
	return empty, e
}

func decodeManifest(b []byte) (manifest, error) {
	var m manifest
	r := privatepipe.NewReader(bytes.NewReader(append(append([]byte(nil), b...), '\n')))
	if r.Read(&m) != nil || m.Version != 1 || !canonical(m.Digest, 32) || m.Size < 1 || m.Size > maximumRecord {
		return m, ErrUnavailable
	}
	var extra struct{}
	if r.Read(&extra) != io.EOF {
		return m, ErrUnavailable
	}
	return m, nil
}

func (s *Store) read() ([]byte, error) {
	if s.closed || s.failed {
		return nil, ErrUnavailable
	}
	b, e := s.kv.Read("manifest")
	if e != nil {
		return nil, e
	}
	if s.fresh && bytes.Equal(b, pending) {
		return nil, ErrFresh
	}
	m, e := decodeManifest(b)
	if e != nil {
		return nil, e
	}
	var raw []byte
	for i := 0; len(raw) < m.Size; i++ {
		part, e := s.kv.Read(partName(m.Digest, i))
		want := min(chunkSize, m.Size-len(raw))
		if e != nil || len(part) != want {
			return nil, ErrUnavailable
		}
		raw = append(raw, part...)
	}
	if Digest(raw) != m.Digest {
		return nil, ErrUnavailable
	}
	return raw, nil
}

func (s *Store) Load() ([]byte, error) {
	b, e := s.read()
	if e != nil && e != ErrFresh {
		s.failed = true
	}
	return b, e
}

// Native chunks are immutable and verified before the single manifest update.
// No partial record can be selected. OS write failure/uncertainty latches closed.
// These API acknowledgements are not a claim of tested power-loss durability.
func (s *Store) Commit(expected string, raw []byte) (err error) {
	if s.closed || s.failed {
		return ErrUnavailable
	}
	defer func() {
		if err != nil {
			s.failed = true
		}
	}()
	old, e := s.read()
	if (e != nil && !(e == ErrFresh && expected == "")) || (e == nil && Digest(old) != expected) || len(raw) < 1 || len(raw) > maximumRecord {
		return ErrUnavailable
	}
	digest := Digest(raw)
	for i, offset := 0, 0; offset < len(raw); i++ {
		part := raw[offset:min(len(raw), offset+chunkSize)]
		name := partName(digest, i)
		prior, e := s.kv.Read(name)
		if e == ErrMissing {
			e = s.kv.Write(name, part)
		} else if e == nil && !bytes.Equal(prior, part) {
			return ErrUnavailable
		}
		if e != nil {
			return e
		}
		got, e := s.kv.Read(name)
		if e != nil || !bytes.Equal(got, part) {
			return ErrUnavailable
		}
		offset += len(part)
	}
	b, _ := json.Marshal(manifest{1, digest, len(raw)})
	if s.kv.Write("manifest", b) != nil {
		return ErrUnavailable
	}
	s.fresh = false
	got, e := s.read()
	if e != nil || !bytes.Equal(got, raw) {
		return ErrUnavailable
	}
	// Exact prior chunks only, after new manifest readback. Failed cleanup is
	// not ignored: explicit reopen can inspect the committed record afterward.
	if len(old) > 0 && Digest(old) != digest {
		for i := 0; i*chunkSize < len(old); i++ {
			if s.kv.Remove(partName(Digest(old), i)) != nil {
				return ErrUnavailable
			}
		}
	}
	return nil
}

// FaultTest never operates outside the isolated test scope. The helper enables
// this method only when explicitly launched with Test=true.
func (s *Store) FaultTest(mode string) error {
	if !s.test {
		return ErrUnavailable
	}
	b, e := s.kv.Read("manifest")
	if e != nil {
		return e
	}
	switch mode {
	case "missing":
		return s.kv.Remove("manifest")
	case "corrupt":
		return s.kv.Write("manifest", []byte("{}"))
	case "chunk":
		m, e := decodeManifest(b)
		if e != nil {
			return e
		}
		return s.kv.Remove(partName(m.Digest, 0))
	case "changedchunk":
		m, e := decodeManifest(b)
		if e != nil {
			return e
		}
		part, e := s.kv.Read(partName(m.Digest, 0))
		if e != nil || len(part) == 0 {
			return ErrUnavailable
		}
		part[0] ^= 1
		return s.kv.Write(partName(m.Digest, 0), part)
	default:
		return ErrUnavailable
	}
}

func (s *Store) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true
	return s.kv.Close()
}

func namespace(scope string, test bool) string {
	if test {
		return "RemoteApp/TrustJournal/Test/v1/" + scope + "/"
	}
	return "RemoteApp/TrustJournal/PoC/v1/" + scope + "/"
}

func validName(name string) bool {
	if name == "manifest" {
		return true
	}
	p := strings.Split(name, "/")
	return len(p) == 3 && p[0] == "data" && canonical(p[1], 32) && len(p[2]) == 1 && p[2][0] >= '0' && p[2][0] <= '3'
}
