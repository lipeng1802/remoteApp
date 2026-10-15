package autonomousbinding

import (
	"bytes"
	"strings"
	"testing"

	"remoteapp.local/selfhost-poc/enrollmentstore"
	"remoteapp.local/selfhost-poc/nativejournal"
)

type memoryJournal struct {
	raw                  []byte
	closed, fail, mutate bool
}

func (m *memoryJournal) Load() ([]byte, error) {
	if m.closed || m.raw == nil {
		return nil, ErrBinding
	}
	raw := append([]byte(nil), m.raw...)
	if m.mutate {
		raw = append(raw, ' ')
	}
	return raw, nil
}
func (m *memoryJournal) Commit(expected, raw []byte) error {
	if m.closed || m.fail || expected != nil || m.raw != nil {
		return ErrBinding
	}
	m.raw = append([]byte(nil), raw...)
	return nil
}
func (m *memoryJournal) Close() error { m.closed = true; return nil }

func fixtureRecord() Record {
	release := strings.Repeat("1", 32)
	vault := strings.Repeat("2", 32)
	public := strings.Repeat("3", 64)
	enrollment, _ := enrollmentstore.Scope(release, vault, public)
	return Record{Version: 1, ReleaseScope: release, VaultScope: vault, PublicKey: public, EnrollmentScope: enrollment}
}

func TestImmutableBindingCreateReopenAndLiveMutation(t *testing.T) {
	record := fixtureRecord()
	j := &memoryJournal{}
	store, err := newStore(j, record.ReleaseScope, &record)
	if err != nil || store.Record() != record || store.Check() != nil {
		t.Fatal("create binding", err)
	}
	original := append([]byte(nil), j.raw...)
	j.mutate = true
	if store.Check() == nil || !bytes.Equal(j.raw, original) {
		t.Fatal("live mutation accepted or repaired")
	}
	j.mutate = false
	reopened, err := newStore(j, record.ReleaseScope, nil)
	if err != nil || reopened.Record() != record {
		t.Fatal("reopen binding", err)
	}
}

func TestBindingRejectsReplacementCorruptionAndUncertainCreate(t *testing.T) {
	record := fixtureRecord()
	bad := record
	bad.PublicKey = strings.Repeat("4", 64)
	for name, candidate := range map[string]Record{"release": {Version: 1}, "derived": bad} {
		t.Run(name, func(t *testing.T) {
			j := &memoryJournal{}
			if _, err := newStore(j, record.ReleaseScope, &candidate); err == nil {
				t.Fatal("invalid binding created")
			}
		})
	}
	j := &memoryJournal{fail: true}
	if _, err := newStore(j, record.ReleaseScope, &record); err == nil {
		t.Fatal("failed commit accepted")
	}
	j = &memoryJournal{raw: []byte(`{"version":1}`)}
	before := append([]byte(nil), j.raw...)
	if _, err := newStore(j, record.ReleaseScope, nil); err == nil || !bytes.Equal(j.raw, before) {
		t.Fatal("corrupt binding repaired")
	}
	if _, err := Scope("bad"); err == nil || nativejournal.Digest(before) == "" {
		t.Fatal("invalid release scope accepted")
	}
}
