// Package enrollmentstore owns a fixed per-account node directory and an OS
// credential journal. Runtime Open never initializes, repairs or resets it.
package enrollmentstore

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/netip"
	"runtime"
	"strings"
	"sync"

	"remoteapp.local/selfhost-poc/nativejournal"
	"remoteapp.local/selfhost-poc/privatepipe"
)

var ErrStorage = errors.New("enrollment_storage_unavailable")

type Journal interface {
	Load() ([]byte, error)
	Commit([]byte, []byte) error
	Close() error
}
type JournalFactory func(scope string, create, test bool) (Journal, error)
type directory interface {
	Path() string
	Check(bool) error // empty=true before the first side effect
	Close() error
	ClearTestMaterials() error
}
type Record struct {
	Version int    `json:"version"`
	Scope   string `json:"scope"`
	Phase   string `json:"phase"`
	NodeKey string `json:"node_key,omitempty"`
	Address string `json:"address,omitempty"`
}
type Store struct {
	j              Journal
	d              directory
	raw            []byte
	record         Record
	failed, closed bool
	release        func()
	test           bool
}

var held = struct {
	sync.Mutex
	scopes map[string]bool
}{scopes: map[string]bool{}}

func canonical(s string, n int) bool {
	b, e := hex.DecodeString(s)
	return e == nil && len(b) == n && hex.EncodeToString(b) == s
}

// Scope is stable across grants, restart, epoch upgrades and executable paths.
// All inputs are trusted installation/identity values, not a peer's device ID.
func Scope(channelScope, vaultScope, public string) (string, error) {
	if !canonical(channelScope, 16) || !canonical(vaultScope, 16) || !canonical(public, 32) {
		return "", ErrStorage
	}
	h := sha256.Sum256([]byte("remoteapp-enrollment-store-v1\n" + channelScope + "\n" + vaultScope + "\n" + public))
	return hex.EncodeToString(h[:16]), nil
}

// Initialize is explicit installer/TEST provisioning, never a fallback in Open.
// Same-goroutine Close is required: Windows mutex ownership is thread-affine.
type nativeJournal struct{ *nativejournal.Store }

func (j nativeJournal) Commit(expected, next []byte) error {
	digest := ""
	if expected != nil {
		digest = nativejournal.Digest(expected)
	}
	return j.Store.Commit(digest, next)
}

func nativeFactory(scope string, create, test bool) (Journal, error) {
	j, err := nativejournal.Open(scope, create, test)
	if err != nil {
		return nil, err
	}
	return nativeJournal{j}, nil
}

func Initialize(scope string, test bool) (*Store, error) {
	return open(scope, test, true, nativeFactory)
}
func Open(scope string, test bool) (*Store, error) {
	return open(scope, test, false, nativeFactory)
}
func InitializeWithJournal(scope string, test bool, factory JournalFactory) (*Store, error) {
	return open(scope, test, true, factory)
}
func OpenWithJournal(scope string, test bool, factory JournalFactory) (*Store, error) {
	return open(scope, test, false, factory)
}

func open(scope string, test, create bool, factory JournalFactory) (*Store, error) {
	if !canonical(scope, 16) || factory == nil {
		return nil, ErrStorage
	}
	id := scope + "/production"
	if test {
		id = scope + "/test"
	}
	held.Lock()
	if held.scopes[id] {
		held.Unlock()
		return nil, ErrStorage
	}
	held.scopes[id] = true
	held.Unlock()
	runtime.LockOSThread()
	release := func() { runtime.UnlockOSThread(); held.Lock(); delete(held.scopes, id); held.Unlock() }
	j, e := factory(scope, create, test)
	if e != nil {
		release()
		return nil, ErrStorage
	}
	d, e := openDirectory(scope, test, create)
	if e != nil {
		j.Close()
		release()
		return nil, ErrStorage
	}
	s, e := newStore(j, d, scope, create)
	if e != nil {
		d.Close()
		j.Close()
		release()
		return nil, e
	}
	s.release = release
	s.test = test
	return s, nil
}

func decode(raw []byte, scope string) (Record, error) {
	var r Record
	reader := privatepipe.NewReader(bytes.NewReader(append(append([]byte(nil), raw...), '\n')))
	if reader.Read(&r) != nil || r.Version != 1 || r.Scope != scope {
		return r, ErrStorage
	}
	// Canonical encoding also rejects trailing data and alternate field shapes.
	b, _ := json.Marshal(r)
	if !bytes.Equal(b, raw) {
		return r, ErrStorage
	}
	switch r.Phase {
	case "idle", "attempted":
		if r.NodeKey != "" || r.Address != "" {
			return r, ErrStorage
		}
	case "registered":
		if !validNode(r.NodeKey) || !validIP(r.Address) {
			return r, ErrStorage
		}
	default:
		return r, ErrStorage
	}
	return r, nil
}
func validNode(s string) bool {
	return strings.HasPrefix(s, "nodekey:") && canonical(strings.TrimPrefix(s, "nodekey:"), 32) && s != "nodekey:"+strings.Repeat("0", 64)
}
func validIP(s string) bool {
	ip, e := netip.ParseAddr(s)
	return e == nil && ip.String() == s && netip.MustParsePrefix("100.120.0.0/24").Contains(ip) && s != "100.120.0.0" && s != "100.120.0.255"
}

func newStore(j Journal, d directory, scope string, create bool) (*Store, error) {
	s := &Store{j: j, d: d}
	if d.Check(create) != nil {
		return nil, ErrStorage
	}
	if create {
		s.record = Record{Version: 1, Scope: scope, Phase: "idle"}
		if e := s.commit(s.record); e != nil {
			return nil, e
		}
	} else {
		b, e := j.Load()
		if e != nil {
			return nil, ErrStorage
		}
		r, e := decode(b, scope)
		if e != nil {
			return nil, e
		}
		s.record, s.raw = r, b
		if r.Phase == "idle" && d.Check(true) != nil {
			return nil, ErrStorage
		}
	}
	return s, nil
}
func (s *Store) commit(r Record) error {
	b, _ := json.Marshal(r)
	var expected []byte
	if len(s.raw) > 0 {
		expected = s.raw
	}
	if s.j.Commit(expected, b) != nil {
		s.failed = true
		return ErrStorage
	}
	got, e := s.j.Load()
	if e != nil || !bytes.Equal(got, b) {
		s.failed = true
		return ErrStorage
	}
	s.raw, s.record = got, r
	return nil
}
func (s *Store) Check() error {
	if s.closed || s.failed || s.d.Check(false) != nil {
		s.failed = true
		return ErrStorage
	}
	b, e := s.j.Load()
	if e != nil || !bytes.Equal(b, s.raw) {
		s.failed = true
		return ErrStorage
	}
	return nil
}

// Begin commits attempted and verifies the native receipt BEFORE Prepare/enroll.
// No transition back to idle exists, including after normal stop or grant expiry.
func (s *Store) Begin() error {
	if s.Check() != nil || s.record.Phase != "idle" || s.d.Check(true) != nil {
		return ErrStorage
	}
	r := s.record
	r.Phase = "attempted"
	return s.commit(r)
}
func (s *Store) Registered(key, address string) error {
	if s.Check() != nil || s.record.Phase != "attempted" || !validNode(key) || !validIP(address) {
		return ErrStorage
	}
	r := s.record
	r.Phase = "registered"
	r.NodeKey = key
	r.Address = address
	return s.commit(r)
}
func (s *Store) NodeDir() string { return s.d.Path() }
func (s *Store) State() Record   { return s.record }

// Isolated validation only, after the caller has actually joined its node.
// Leaves empty non-secret directories; no production cleanup/reset capability.
func (s *Store) CleanupTestAfterJoin() error {
	return s.CleanupTestAfterJoinWith(nativejournal.CleanupTest)
}

func (s *Store) CleanupTestAfterJoinWith(cleanup func(string) error) error {
	if !s.test || s.Check() != nil || s.d.ClearTestMaterials() != nil {
		return ErrStorage
	}
	scope := s.record.Scope
	if s.Close() != nil {
		return ErrStorage
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if cleanup == nil || cleanup(scope) != nil {
		return ErrStorage
	}
	return nil
}

// Caller must join its node first. Close releases, but NEVER deletes material.
func (s *Store) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true
	e := s.d.Close()
	if next := s.j.Close(); e == nil {
		e = next
	}
	if s.release != nil {
		s.release()
		s.release = nil
	}
	if e != nil {
		return ErrStorage
	}
	return nil
}
