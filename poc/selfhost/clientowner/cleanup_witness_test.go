package clientowner

import (
	"context"
	"remoteapp.local/selfhost-poc/trustprofile"
	"testing"
)

type badCloseJournal struct{ *journal }

func (j badCloseJournal) Close() error { j.journal.Close(); return trustprofile.ErrTrust }
func TestJournalCloseWitnessSeparatesBusinessFaultAndCleanupFailure(t *testing.T) {
	for _, bad := range []bool{false, true} {
		o, j, _ := setup(t)
		if bad {
			o.journal = badCloseJournal{j}
		}
		h := mustStart(t, o, func(context.Context, trustprofile.Profile) error { return trustprofile.ErrTrust })
		_ = h.Wait(context.Background())
		if o.JournalClosedCleanly(context.Background()) {
			t.Fatal("live lease became closed")
		}
		if o.Close(context.Background()) == nil {
			t.Fatal("runner fault hidden")
		}
		if o.JournalClosedCleanly(context.Background()) == bad {
			t.Fatal("close receipt conflated with work error")
		}
	}
}
