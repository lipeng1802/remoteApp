// Package autonomousbinding owns the immutable local identity selected during
// explicit autonomous-product installation. Runtime peers and parent frames
// cannot select another vault or enrollment namespace.
package autonomousbinding

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"runtime"
	"sync"

	"remoteapp.local/selfhost-poc/enrollmentstore"
	"remoteapp.local/selfhost-poc/nativejournal"
	"remoteapp.local/selfhost-poc/privatepipe"
)

var ErrBinding = errors.New("autonomous_binding_unavailable")

type Record struct {
	Version         int    `json:"version"`
	ReleaseScope    string `json:"release_scope"`
	VaultScope      string `json:"vault_scope"`
	PublicKey       string `json:"public_key"`
	EnrollmentScope string `json:"enrollment_scope"`
}

type Journal interface {
	Load() ([]byte, error)
	Commit([]byte, []byte) error
	Close() error
}
type JournalFactory func(scope string, create, test bool) (Journal, error)

type Store struct {
	j       Journal
	record  Record
	raw     []byte
	closed  bool
	release func()
}

// Installer reserves the immutable native namespace before any identity or
// node-directory side effect. A failed install leaves the native pending marker
// in place, so retry cannot silently select a fresh identity or namespace.
type Installer struct {
	intent                  []byte
	intentAttempted, broken bool
	vaultScope              string
	j                       Journal
	releaseScope            string
	closed                  bool
	release                 func()
}

var held = struct {
	sync.Mutex
	scopes map[string]bool
}{scopes: map[string]bool{}}

func canonical(value string, size int) bool {
	b, err := hex.DecodeString(value)
	return err == nil && len(b) == size && hex.EncodeToString(b) == value
}

// Scope is independent from both the trust journal and enrollment journal,
// while remaining stable across signed epoch upgrades in the same channel.
func Scope(releaseScope string) (string, error) {
	if !canonical(releaseScope, 16) {
		return "", ErrBinding
	}
	h := sha256.Sum256([]byte("remoteapp-autonomous-install-binding-v1\n" + releaseScope))
	return hex.EncodeToString(h[:16]), nil
}

func valid(record Record) bool {
	if record.Version != 1 || !canonical(record.ReleaseScope, 16) || !canonical(record.VaultScope, 16) || !canonical(record.PublicKey, 32) || !canonical(record.EnrollmentScope, 16) {
		return false
	}
	want, err := enrollmentstore.Scope(record.ReleaseScope, record.VaultScope, record.PublicKey)
	return err == nil && want == record.EnrollmentScope
}

func decode(raw []byte) (Record, error) {
	var record Record
	reader := privatepipe.NewReader(bytes.NewReader(append(append([]byte(nil), raw...), '\n')))
	if reader.Read(&record) != nil || !valid(record) {
		return Record{}, ErrBinding
	}
	var extra struct{}
	if reader.Read(&extra) != io.EOF {
		return Record{}, ErrBinding
	}
	canonicalRaw, _ := json.Marshal(record)
	if !bytes.Equal(raw, canonicalRaw) {
		return Record{}, ErrBinding
	}
	return record, nil
}

func newStore(j Journal, expectedRelease string, create *Record) (*Store, error) {
	if j == nil || !canonical(expectedRelease, 16) {
		return nil, ErrBinding
	}
	store := &Store{j: j}
	if create != nil {
		if !valid(*create) || create.ReleaseScope != expectedRelease {
			return nil, ErrBinding
		}
		raw, _ := json.Marshal(*create)
		if j.Commit(nil, raw) != nil {
			return nil, ErrBinding
		}
		store.raw = raw
	}
	raw, err := j.Load()
	if err != nil || (len(store.raw) > 0 && !bytes.Equal(raw, store.raw)) {
		return nil, ErrBinding
	}
	record, err := decode(raw)
	if err != nil || record.ReleaseScope != expectedRelease {
		return nil, ErrBinding
	}
	store.raw, store.record = append([]byte(nil), raw...), record
	return store, nil
}

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

func acquire(releaseScope string, create, test bool, factory JournalFactory) (Journal, func(), error) {
	nativeScope, err := Scope(releaseScope)
	if err != nil || factory == nil {
		return nil, nil, err
	}
	id := nativeScope + "/production"
	if test {
		id = nativeScope + "/test"
	}
	held.Lock()
	if held.scopes[id] {
		held.Unlock()
		return nil, nil, ErrBinding
	}
	held.scopes[id] = true
	held.Unlock()
	runtime.LockOSThread()
	release := func() {
		runtime.UnlockOSThread()
		held.Lock()
		delete(held.scopes, id)
		held.Unlock()
	}
	j, err := factory(nativeScope, create, test)
	if err != nil {
		release()
		return nil, nil, ErrBinding
	}
	return j, release, nil
}

func Begin(releaseScope string, test bool) (*Installer, error) {
	return BeginWithJournal(releaseScope, test, nativeFactory)
}

func BeginWithJournal(releaseScope string, test bool, factory JournalFactory) (*Installer, error) {
	j, release, err := acquire(releaseScope, true, test, factory)
	if err != nil {
		return nil, err
	}
	return &Installer{j: j, releaseScope: releaseScope, release: release}, nil
}

func (i *Installer) Commit(vaultScope, public string) (*Store, error) {
	if i == nil || i.closed || i.j == nil || i.broken || (i.intentAttempted && vaultScope != i.vaultScope) {
		return nil, ErrBinding
	}
	enrollmentScope, err := enrollmentstore.Scope(i.releaseScope, vaultScope, public)
	if err != nil {
		return nil, ErrBinding
	}
	record := Record{Version: 1, ReleaseScope: i.releaseScope, VaultScope: vaultScope, PublicKey: public, EnrollmentScope: enrollmentScope}
	var store *Store
	if i.intentAttempted {
		raw, _ := json.Marshal(record)
		i.broken = true // unknown CAS receipts must never permit another commit
		if i.j.Commit(i.intent, raw) != nil {
			return nil, ErrBinding
		}
		store, err = newStore(i.j, i.releaseScope, nil)
	} else {
		store, err = newStore(i.j, i.releaseScope, &record)
	}
	if err != nil {
		return nil, err
	}
	store.release = i.release
	i.closed, i.j, i.release = true, nil, nil
	return store, nil
}

func (i *Installer) Close() error {
	if i == nil || i.closed || i.j == nil {
		return ErrBinding
	}
	i.closed = true
	err := i.j.Close()
	i.j = nil
	if i.release != nil {
		i.release()
		i.release = nil
	}
	if err != nil {
		return ErrBinding
	}
	return nil
}

func Initialize(releaseScope, vaultScope, public string, test bool) (*Store, error) {
	installer, err := Begin(releaseScope, test)
	if err != nil {
		return nil, err
	}
	store, err := installer.Commit(vaultScope, public)
	if err != nil {
		_ = installer.Close()
	}
	return store, err
}

func Open(releaseScope string, test bool) (*Store, error) {
	return OpenWithJournal(releaseScope, test, nativeFactory)
}

func OpenWithJournal(releaseScope string, test bool, factory JournalFactory) (*Store, error) {
	j, release, err := acquire(releaseScope, false, test, factory)
	if err != nil {
		return nil, err
	}
	store, err := newStore(j, releaseScope, nil)
	if err != nil {
		j.Close()
		release()
		return nil, err
	}
	store.release = release
	return store, nil
}

// CleanupTest removes only the exact derived TEST namespace. Production has no
// delete/reset API and a pending or corrupt production install remains closed.
func CleanupTest(releaseScope string) error {
	return CleanupTestWith(releaseScope, nativejournal.CleanupTest)
}

func CleanupTestWith(releaseScope string, cleanup func(string) error) error {
	nativeScope, err := Scope(releaseScope)
	if err != nil || cleanup == nil || cleanup(nativeScope) != nil {
		return ErrBinding
	}
	return nil
}

func (s *Store) Record() Record { return s.record }

func (s *Store) Check() error {
	if s == nil || s.closed || s.j == nil {
		return ErrBinding
	}
	raw, err := s.j.Load()
	if err != nil || !bytes.Equal(raw, s.raw) {
		return ErrBinding
	}
	record, err := decode(raw)
	if err != nil || record != s.record {
		return ErrBinding
	}
	return nil
}

func (s *Store) Close() error {
	if s == nil || s.closed {
		return ErrBinding
	}
	s.closed = true
	err := s.j.Close()
	if s.release != nil {
		s.release()
		s.release = nil
	}
	if err != nil {
		return ErrBinding
	}
	return nil
}

// ReserveIdentity must succeed before calling the OS vault. This is an intent,
// NOT evidence that an identity exists or permission to reopen/delete one.
// The final runtime binding remains unavailable until the exact CAS completes.
func (i *Installer) ReserveIdentity(vaultScope string) error {
	if i == nil || i.closed || i.j == nil || i.intentAttempted || !canonical(vaultScope, 16) {
		return ErrBinding
	}
	i.intentAttempted = true
	i.broken = true
	intent := struct {
		Version      int    `json:"version"`
		Kind         string `json:"kind"`
		ReleaseScope string `json:"release_scope"`
		VaultScope   string `json:"vault_scope"`
	}{1, "identity_creation_intent", i.releaseScope, vaultScope}
	raw, _ := json.Marshal(intent)
	if i.j.Commit(nil, raw) != nil {
		return ErrBinding
	}
	got, e := i.j.Load()
	if e != nil || !bytes.Equal(raw, got) {
		return ErrBinding
	}
	i.intent = raw
	i.vaultScope = vaultScope
	i.broken = false
	return nil
}
