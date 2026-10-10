package clientowner

import (
	"context"
	"errors"
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/trustinstall"
	"remoteapp.local/selfhost-poc/trustprofile"
)

// In-memory coordination coverage only. The native test separately exercises
// actual OS helper exclusivity and persisted epoch state.
func replacementFixture(t *testing.T) (*Owner, *journal, *int) {
	t.Helper()
	o, j, _ := setup(t)
	calls := new(int)
	o.factory = func(create bool) (trustinstall.Journal, error) {
		*calls++
		if create || !j.closed {
			return nil, trustprofile.ErrTrust
		}
		return &journal{raw: append([]byte(nil), j.raw...)}, nil
	}
	return o, j, calls
}

func retire(t *testing.T, o *Owner) {
	t.Helper()
	h := mustStart(t, o, func(context.Context, trustprofile.Profile) error { return errors.New("fault") })
	<-h.Done()
	if e := o.RetireFault(context.Background()); e != nil {
		t.Fatal(e)
	}
}

func TestReplacementRequiresSuccessfulFaultRetirement(t *testing.T) {
	for _, state := range []string{"idle", "pending", "fault_unretired", "normal_closed", "release_failed"} {
		t.Run(state, func(t *testing.T) {
			o, j, calls := replacementFixture(t)
			switch state {
			case "pending":
				release := make(chan struct{})
				h := mustStart(t, o, func(context.Context, trustprofile.Profile) error { <-release; return errors.New("fault") })
				t.Cleanup(func() { close(release); <-h.Done() })
			case "fault_unretired", "release_failed":
				h := mustStart(t, o, func(context.Context, trustprofile.Profile) error { return errors.New("fault") })
				<-h.Done()
				if state == "release_failed" {
					o.journal = failedRetirementJournal{j}
					if o.RetireFault(context.Background()) == nil {
						t.Fatal("release accepted")
					}
					o.journal = j
				}
			case "normal_closed":
				h := mustStart(t, o, func(context.Context, trustprofile.Profile) error { return nil })
				<-h.Done()
				if o.Close(context.Background()) != nil {
					t.Fatal("normal close")
				}
			}
			if replacement, e := o.OpenReplacement(context.Background()); e == nil || replacement != nil || *calls != 0 {
				t.Fatal("unretired owner opened replacement")
			}
		})
	}
}

func TestReplacementIsDifferentIdleOwnerAndOldOwnerRemainsClosed(t *testing.T) {
	o, _, calls := replacementFixture(t)
	retire(t, o)
	next, e := o.OpenReplacement(context.Background())
	if e != nil || next == o || next == nil || next.running != nil || *calls != 1 {
		t.Fatal("not fresh idle owner", e)
	}
	defer next.Close(context.Background())
	if _, e = o.Start(context.Background(), func(context.Context, trustprofile.Profile) error { t.Fatal("old runner launched"); return nil }); e == nil {
		t.Fatal("old owner revived")
	}
	h := mustStart(t, next, func(context.Context, trustprofile.Profile) error { return nil })
	if h.Wait(context.Background()) != nil {
		t.Fatal("explicit new start")
	}
}

func TestReplacementRejectsMissingCorruptStateAndFailedFactory(t *testing.T) {
	for _, state := range []string{"missing", "corrupt", "factory"} {
		t.Run(state, func(t *testing.T) {
			o, j, _ := replacementFixture(t)
			retire(t, o)
			switch state {
			case "missing":
				j.raw = nil
			case "corrupt":
				j.raw = []byte("corrupt")
			case "factory":
				o.factory = func(bool) (trustinstall.Journal, error) { return nil, errors.New("unavailable") }
			}
			if next, e := o.OpenReplacement(context.Background()); e == nil || next != nil {
				t.Fatal("invalid state repaired")
			}
			if !o.closed || !o.retiredFault {
				t.Fatal("old state mutated")
			}
		})
	}
}

func TestReplacementRejectsCanceledRequestAndExpiredInstallation(t *testing.T) {
	o, _, calls := replacementFixture(t)
	retire(t, o)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if next, e := o.OpenReplacement(ctx); e == nil || next != nil || *calls != 0 {
		t.Fatal("canceled request opened state")
	}
	o.clock = func() time.Time { return time.Now().Add(48 * time.Hour) }
	if next, e := o.OpenReplacement(context.Background()); e == nil || next != nil {
		t.Fatal("expired profile reopened")
	}
}
