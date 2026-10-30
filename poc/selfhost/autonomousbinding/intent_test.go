package autonomousbinding

import (
	"bytes"
	"encoding/json"
	"testing"
)

type intentJournal struct {
	raw       []byte
	calls     int
	uncertain int
	closed    bool
}

func (j *intentJournal) Load() ([]byte, error) {
	if j.closed {
		return nil, ErrBinding
	}
	return append([]byte(nil), j.raw...), nil
}
func (j *intentJournal) Commit(expected, next []byte) error {
	j.calls++
	if j.closed || !bytes.Equal(expected, j.raw) {
		return ErrBinding
	}
	j.raw = append([]byte(nil), next...)
	if j.calls == j.uncertain {
		return ErrBinding
	}
	return nil
}
func (j *intentJournal) Close() error { j.closed = true; return nil }
func TestIdentityIntentPrecedesCreationAndCannotRunAsBinding(t *testing.T) {
	r := fixtureRecord()
	j := &intentJournal{}
	i := &Installer{j: j, releaseScope: r.ReleaseScope}
	if i.ReserveIdentity(r.VaultScope) != nil {
		t.Fatal("intent failed")
	}
	var raw map[string]any
	if json.Unmarshal(j.raw, &raw) != nil || raw["vault_scope"] != r.VaultScope || raw["kind"] != "identity_creation_intent" {
		t.Fatal("scope not retained")
	}
	if _, e := newStore(j, r.ReleaseScope, nil); e == nil {
		t.Fatal("intent accepted as runtime binding")
	}
	if i.ReserveIdentity(r.VaultScope) == nil || j.calls != 1 {
		t.Fatal("intent repeated")
	}
	if _, e := i.Commit("00000000000000000000000000000000", r.PublicKey); e == nil || j.calls != 1 {
		t.Fatal("foreign scope committed")
	}
	s, e := i.Commit(r.VaultScope, r.PublicKey)
	if e != nil || s.Check() != nil || s.Record() != r || j.calls != 2 {
		t.Fatal("exact CAS binding failed", e)
	}
	if s.Close() != nil {
		t.Fatal("close")
	}
}
func TestUnknownIntentAndFinalReceiptsForbidReuse(t *testing.T) {
	for _, failAt := range []int{1, 2} {
		r := fixtureRecord()
		j := &intentJournal{uncertain: failAt}
		i := &Installer{j: j, releaseScope: r.ReleaseScope}
		e := i.ReserveIdentity(r.VaultScope)
		if failAt == 1 && e == nil {
			t.Fatal("unknown intent accepted")
		}
		if failAt == 2 {
			if e != nil {
				t.Fatal(e)
			}
			if _, e = i.Commit(r.VaultScope, r.PublicKey); e == nil {
				t.Fatal("unknown final accepted")
			}
		}
		before := append([]byte(nil), j.raw...)
		calls := j.calls
		if _, e = i.Commit(r.VaultScope, r.PublicKey); e == nil || j.calls != calls || !bytes.Equal(j.raw, before) {
			t.Fatal("unknown receipt retried")
		}
		if i.Close() != nil {
			t.Fatal("lease close failed")
		}
	}
}

type uncertainReadJournal struct{ *intentJournal }

func (j uncertainReadJournal) Load() ([]byte, error) { return nil, ErrBinding }
func TestIntentReadbackFailureCannotCreateBinding(t *testing.T) {
	r := fixtureRecord()
	j := &intentJournal{}
	i := &Installer{j: uncertainReadJournal{j}, releaseScope: r.ReleaseScope}
	if i.ReserveIdentity(r.VaultScope) == nil {
		t.Fatal("unknown readback accepted")
	}
	if _, e := i.Commit(r.VaultScope, r.PublicKey); e == nil || j.calls != 1 {
		t.Fatal("binding created after unknown readback")
	}
	if i.Close() != nil {
		t.Fatal("close")
	}
}
