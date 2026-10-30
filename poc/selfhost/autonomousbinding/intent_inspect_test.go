package autonomousbinding

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestIntentInspectionUsesExclusiveReadLeaseWithoutMutation(t *testing.T) {
	r := fixtureRecord()
	raw, _ := json.Marshal(creationIntent{1, "identity_creation_intent", r.ReleaseScope, r.VaultScope})
	j := &intentJournal{raw: raw}
	opened := 0
	factory := func(scope string, create, test bool) (Journal, error) {
		opened++
		want, _ := Scope(r.ReleaseScope)
		if scope != want || create || !test {
			t.Fatal("inspection changed namespace/mode")
		}
		return j, nil
	}
	found, e := CheckCreationIntentWithJournal(r.ReleaseScope, true, factory)
	if e != nil || !found || !j.closed || j.calls != 0 || opened != 1 || !bytes.Equal(j.raw, raw) {
		t.Fatal("inspection mutated or failed to close", e)
	}
}
func TestIntentClassificationRejectsForeignCorruptAndNoncanonical(t *testing.T) {
	r := fixtureRecord()
	v := creationIntent{1, "identity_creation_intent", r.ReleaseScope, r.VaultScope}
	raw, _ := json.Marshal(v)
	for _, bad := range [][]byte{append(append([]byte(nil), raw...), ' '), []byte(`{"version":1,"kind":"identity_creation_intent"}`), append(raw, []byte("\n{}")...)} {
		if _, e := isCreationIntent(bad, r.ReleaseScope); e == nil {
			t.Fatal("corruption accepted")
		}
	}
	if _, e := isCreationIntent(raw, r.VaultScope); e == nil {
		t.Fatal("foreign intent accepted")
	}
	final, _ := json.Marshal(r)
	if found, e := isCreationIntent(final, r.ReleaseScope); e != nil || found {
		t.Fatal("final binding mislabeled")
	}
}

type failedInspectionClose struct{ *intentJournal }

func (j failedInspectionClose) Close() error { j.intentJournal.Close(); return ErrBinding }
func TestIntentInspectionUnknownCloseNeverReturnsProof(t *testing.T) {
	r := fixtureRecord()
	raw, _ := json.Marshal(creationIntent{1, "identity_creation_intent", r.ReleaseScope, r.VaultScope})
	j := &intentJournal{raw: raw}
	found, e := CheckCreationIntentWithJournal(r.ReleaseScope, true, func(string, bool, bool) (Journal, error) { return failedInspectionClose{j}, nil })
	if found || e == nil || j.calls != 0 || !j.closed {
		t.Fatal("unknown close accepted")
	}
}

func TestIntentInspectionRejectsMissingFactory(t *testing.T) {
	if found, e := CheckCreationIntentWithJournal(fixtureRecord().ReleaseScope, true, nil); found || e == nil {
		t.Fatal("nil factory accepted")
	}
}
