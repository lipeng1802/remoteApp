package trustprofile

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/privatepipe"
)

func TestDurableMaintenancePreservesAuthorityAndBlocksLegacyAdmission(t *testing.T) {
	_, j, a, _, next := installationFixture(t)
	s, _ := OpenInstallation(a, &j, time.Now)
	if err := s.Accept(context.Background(), next, quiesceFunc(func(context.Context) error { return nil })); err != nil {
		t.Fatal(err)
	}
	before := s.record
	if err := s.BeginMaintenance(); err != nil {
		t.Fatal(err)
	}
	state, err := ReadMaintenance(a, &j, time.Now)
	if err != nil || state.State != "pending" || state.Epoch != next.Claims.Epoch || len(state.Transaction) != 32 {
		t.Fatal("invalid durable maintenance", err)
	}
	after := s.record
	after.Maintenance = nil
	if before != after {
		t.Fatal("maintenance altered accepted authority")
	}
	if _, _, err = s.Current(); err == nil {
		t.Fatal("profile remained usable")
	}
	if _, err = OpenInstallation(a, &j, time.Now); !errors.Is(err, ErrMaintenance) {
		t.Fatal("reopen bypassed pending", err)
	}
	if _, err = InitializeInstallation(a, next, &j, time.Now); err == nil {
		t.Fatal("reinitialize erased pending")
	}
	if s.BeginMaintenance() == nil || s.Accept(context.Background(), next, quiesceFunc(func(context.Context) error { return nil })) == nil {
		t.Fatal("pending state advanced")
	}
	// The pre-maintenance schema's strict decoder rejects the additional field.
	var legacy struct {
		Version    int     `json:"version"`
		Anchor     string  `json:"anchor"`
		Epoch      uint64  `json:"epoch"`
		AcceptedAt int64   `json:"accepted_at"`
		Profile    Profile `json:"profile"`
	}
	if privatepipe.NewReader(bytes.NewReader(append(append([]byte(nil), j.raw...), '\n'))).Read(&legacy) == nil {
		t.Fatal("legacy schema admitted pending record")
	}
	expired := func() time.Time { return time.Unix(next.Claims.ExpiresAt+1, 0) }
	if snapshot, e := ReadMaintenance(a, &j, expired); e != nil || snapshot != state {
		t.Fatal("historical diagnostic lost floor", e)
	}
	wrong := a
	wrong.Root[0] ^= 1
	if _, e := ReadMaintenance(wrong, &j, time.Now); e == nil {
		t.Fatal("wrong root admitted")
	}
}

func TestDurableMaintenanceUncertainCommitLatchesClosed(t *testing.T) {
	for _, uncertain := range []bool{false, true} {
		_, j, a, _, _ := installationFixture(t)
		s, _ := OpenInstallation(a, &j, time.Now)
		j.commitErr = true
		j.uncertain = uncertain
		if s.BeginMaintenance() == nil {
			t.Fatal("failed commit succeeded")
		}
		if _, _, e := s.Current(); e == nil {
			t.Fatal("uncertain owner remained usable")
		}
		_, err := OpenInstallation(a, &j, time.Now)
		if uncertain && !errors.Is(err, ErrMaintenance) {
			t.Fatal("durable pending ignored", err)
		}
		if !uncertain && err != nil {
			t.Fatal("pre-commit failure corrupted record", err)
		}
	}
}

func TestDurableMaintenanceMalformedMarkersFailClosed(t *testing.T) {
	for _, marker := range []string{`null`, `{}`, `{"version":2,"state":"pending","id":"11111111111111111111111111111111"}`, `{"version":1,"state":"completed","id":"11111111111111111111111111111111"}`, `{"version":1,"state":"pending","id":"bad"}`, `{"version":1,"state":"pending","id":"11111111111111111111111111111111","path":"override"}`} {
		_, j, a, _, _ := installationFixture(t)
		var record map[string]json.RawMessage
		if json.Unmarshal(j.raw, &record) != nil {
			t.Fatal("fixture")
		}
		record["maintenance"] = json.RawMessage(marker)
		j.raw, _ = json.Marshal(record)
		if _, err := OpenInstallation(a, &j, time.Now); err == nil {
			t.Fatal("malformed marker admitted")
		}
		if _, err := ReadMaintenance(a, &j, time.Now); err == nil {
			t.Fatal("malformed marker inspected")
		}
	}
}
