package trustprofile

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestRemovalTransactionPreservesAuthorityAndFencesEveryPhase(t *testing.T) {
	_, j, a, _, _ := installationFixture(t)
	s, err := OpenInstallation(a, &j, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.BeginMaintenance(); err != nil {
		t.Fatal(err)
	}
	tx, err := OpenRemovalTransaction(a, &j, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	original, _, _ := readInstallation(a, &j, time.Now)
	digest := strings.Repeat("a", 64)
	if tx.MarkFilesRemoved(digest) == nil {
		t.Fatal("completion before bind")
	}
	for _, phase := range []string{"pending", "deleting", "files_removed"} {
		if phase == "deleting" {
			err = tx.BindRemovalPlan(digest)
		}
		if phase == "files_removed" {
			err = tx.MarkFilesRemoved(digest)
		}
		if err != nil {
			t.Fatal(err)
		}
		state, e := tx.State()
		if e != nil || state.State != phase {
			t.Fatal(state, e)
		}
		if _, e = OpenInstallation(a, &j, time.Now); e != ErrMaintenance {
			t.Fatal("reopened active", phase, e)
		}
		current, _, e := readInstallation(a, &j, time.Now)
		if e != nil || current.Profile != original.Profile || current.Epoch != original.Epoch || current.Anchor != original.Anchor || current.AcceptedAt != original.AcceptedAt || current.Maintenance.ID != original.Maintenance.ID {
			t.Fatal("authority changed")
		}
		if phase != "pending" && tx.BindRemovalPlan(strings.Repeat("b", 64)) == nil {
			t.Fatal("changed plan accepted")
		}
	}
	before := append([]byte(nil), j.raw...)
	if tx.MarkFilesRemoved(digest) != nil || tx.BindRemovalPlan(digest) != nil || !bytes.Equal(before, j.raw) {
		t.Fatal("non-idempotent completion")
	}
}

func TestRemovalTransactionUncertainCommitRequiresFreshReopen(t *testing.T) {
	for _, complete := range []bool{false, true} {
		_, j, a, _, _ := installationFixture(t)
		s, _ := OpenInstallation(a, &j, time.Now)
		s.BeginMaintenance()
		tx, _ := OpenRemovalTransaction(a, &j, time.Now)
		d := strings.Repeat("c", 64)
		if complete {
			if err := tx.BindRemovalPlan(d); err != nil {
				t.Fatal(err)
			}
		}
		j.commitErr, j.uncertain = true, true
		var err error
		if complete {
			err = tx.MarkFilesRemoved(d)
		} else {
			err = tx.BindRemovalPlan(d)
		}
		if err == nil {
			t.Fatal("lost receipt accepted")
		}
		j.commitErr = false
		if _, err = tx.State(); err == nil {
			t.Fatal("uncertain owner reused")
		}
		fresh, err := OpenRemovalTransaction(a, &j, time.Now)
		if err != nil {
			t.Fatal(err)
		}
		state, _ := fresh.State()
		want := "deleting"
		if complete {
			want = "files_removed"
		}
		if state.State != want || state.PlanSHA256 != d {
			t.Fatal(state)
		}
	}
}
