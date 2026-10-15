package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"testing"

	"remoteapp.local/selfhost-poc/autonomousbinding"
	"remoteapp.local/selfhost-poc/enrollmentstore"
	"remoteapp.local/selfhost-poc/trustinstall"
	"remoteapp.local/selfhost-poc/trustprofile"
)

type testJournal struct {
	raw    *[]byte
	closed bool
}

func (j *testJournal) Load() ([]byte, error) {
	if j.closed || j.raw == nil || *j.raw == nil {
		return nil, errors.New("missing")
	}
	return append([]byte(nil), (*j.raw)...), nil
}
func (j *testJournal) Commit(expected, next []byte) error {
	if j.closed || j.raw == nil || !bytes.Equal(expected, *j.raw) {
		return errors.New("commit")
	}
	*j.raw = append([]byte(nil), next...)
	return nil
}
func (j *testJournal) Close() error { j.closed = true; return nil }

func randomScope(t *testing.T, size int) string {
	t.Helper()
	raw := make([]byte, size)
	if _, err := rand.Read(raw); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(raw)
}

func TestDiagnosisStagesDoNotRepairOrSelectIdentity(t *testing.T) {
	release, vault, public := randomScope(t, 16), randomScope(t, 16), randomScope(t, 32)
	enrollment, err := enrollmentstore.Scope(release, vault, public)
	if err != nil {
		t.Fatal(err)
	}
	bindingScope, _ := autonomousbinding.Scope(release)
	record := autonomousbinding.Record{Version: 1, ReleaseScope: release, VaultScope: vault, PublicKey: public, EnrollmentScope: enrollment}
	bindingRaw, _ := json.Marshal(record)
	raws := map[string][]byte{bindingScope: bindingRaw}
	factory := trustinstall.RecordFactory(func(scope string, create, test bool) (trustinstall.Journal, error) {
		if create || !test {
			return nil, errors.New("unexpected create")
		}
		raw, ok := raws[scope]
		if !ok {
			return nil, errors.New("missing")
		}
		copyRaw := append([]byte(nil), raw...)
		return &testJournal{raw: &copyRaw}, nil
	})
	for name, states := range map[string]map[string]trustprofile.JournalState{
		"binding_pending":  {bindingScope: trustprofile.JournalPending},
		"binding_unusable": {bindingScope: trustprofile.JournalUnusable},
		"enrollment_missing": {
			bindingScope: trustprofile.JournalCommitted, enrollment: trustprofile.JournalAbsent,
		},
		"enrollment_pending": {
			bindingScope: trustprofile.JournalCommitted, enrollment: trustprofile.JournalPending,
		},
		"enrollment_unusable": {
			bindingScope: trustprofile.JournalCommitted, enrollment: trustprofile.JournalUnusable,
		},
	} {
		t.Run(name, func(t *testing.T) {
			identityCalls := 0
			state := trustinstall.RecordState(func(scope string, test bool) (trustprofile.JournalState, error) {
				if !test {
					t.Fatal("production probe")
				}
				return states[scope], nil
			})
			got, err := diagnose(release, 7, true, factory, state, func(autonomousbinding.Record) error {
				identityCalls++
				return nil
			})
			if err != nil || got.Status != "maintenance_required" || got.Stage != name || got.Diagnostic != 1 || got.TrustEpoch != 7 || got.PublicKey != "" || identityCalls != 0 {
				t.Fatalf("got %+v calls=%d err=%v", got, identityCalls, err)
			}
		})
	}
	absent, err := diagnose(release, 7, true, factory, func(string, bool) (trustprofile.JournalState, error) {
		return trustprofile.JournalAbsent, nil
	}, func(autonomousbinding.Record) error { t.Fatal("identity probe"); return nil })
	if err != nil || absent.Status != "not_installed" || absent.Stage != "" || absent.Diagnostic != 1 || absent.TrustEpoch != 7 || absent.PublicKey != "" {
		t.Fatal("absent", absent, err)
	}
	if _, err = diagnose(release, 0, true, factory, nil, func(autonomousbinding.Record) error { return nil }); err == nil {
		t.Fatal("zero trust epoch accepted")
	}
}

func TestDiagnosisUsableAndIdentityUnavailable(t *testing.T) {
	release, vault, public := randomScope(t, 16), randomScope(t, 16), randomScope(t, 32)
	enrollment, _ := enrollmentstore.Scope(release, vault, public)
	bindingScope, _ := autonomousbinding.Scope(release)
	bindingRaw, _ := json.Marshal(autonomousbinding.Record{Version: 1, ReleaseScope: release, VaultScope: vault, PublicKey: public, EnrollmentScope: enrollment})
	raws := map[string][]byte{bindingScope: bindingRaw}
	factory := trustinstall.RecordFactory(func(scope string, create, test bool) (trustinstall.Journal, error) {
		raw := raws[scope]
		return &testJournal{raw: &raw}, nil
	})
	enrollmentFactory := enrollmentstore.JournalFactory(func(scope string, create, test bool) (enrollmentstore.Journal, error) {
		raw := raws[scope]
		journal := &testJournal{raw: &raw}
		return journal, nil
	})
	state, err := enrollmentstore.InitializeWithJournal(enrollment, true, enrollmentFactory)
	if err != nil {
		t.Fatal(err)
	}
	raw := state.State()
	raws[enrollment], _ = json.Marshal(raw)
	if state.Close() != nil {
		t.Fatal("close fixture")
	}
	states := trustinstall.RecordState(func(string, bool) (trustprofile.JournalState, error) { return trustprofile.JournalCommitted, nil })
	got, err := diagnose(release, 11, true, factory, states, func(record autonomousbinding.Record) error {
		if record.PublicKey != public {
			t.Fatal("wrong identity")
		}
		return nil
	})
	if err != nil || got.Status != "usable" || got.PublicKey != "" || got.EnrollmentState != "idle" || got.Diagnostic != 1 || got.TrustEpoch != 11 {
		t.Fatal("usable", got, err)
	}
	got, err = diagnose(release, 11, true, factory, states, func(autonomousbinding.Record) error { return errors.New("locked") })
	if err != nil || got.Status != "maintenance_required" || got.Stage != "identity_unavailable" || got.PublicKey != "" || got.Diagnostic != 1 || got.TrustEpoch != 11 {
		t.Fatal("identity unavailable", got, err)
	}
	cleanup, err := enrollmentstore.OpenWithJournal(enrollment, true, enrollmentFactory)
	if err != nil || cleanup.CleanupTestAfterJoinWith(func(string) error { return nil }) != nil {
		t.Fatal("cleanup", err)
	}
}
