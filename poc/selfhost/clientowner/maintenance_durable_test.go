package clientowner

import (
	"context"
	"errors"
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/trustprofile"
)

func TestDurableMaintenanceCommitsBeforeActionAndCannotRetry(t *testing.T) {
	o, j, next := setup(t)
	if err := o.Upgrade(context.Background(), next); err != nil {
		t.Fatal(err)
	}
	calls := 0
	if err := o.MaintainDurable(context.Background(), func(context.Context, uint64) error {
		calls++
		state, err := trustprofile.ReadMaintenance(o.release.Anchor(), j, time.Now)
		if err != nil || state.Epoch != 2 || state.State != "pending" {
			t.Fatal("action before durable pending", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if calls != 1 || j.commits != 3 {
		t.Fatal("incorrect durable transition")
	}
	if o.Maintain(context.Background(), func(context.Context, uint64) error { t.Fatal("action repeated"); return nil }) == nil {
		t.Fatal("ephemeral path bypassed pending")
	}
	if err := o.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	j.mu.Lock()
	j.closed = false
	j.mu.Unlock() // model a new lease over persisted bytes
	if _, err := trustprofile.OpenInstallation(o.release.Anchor(), j, time.Now); !errors.Is(err, trustprofile.ErrMaintenance) {
		t.Fatal("fresh installation opened", err)
	}
}

func TestDurableMaintenanceLostReceiptNeverInvokesAction(t *testing.T) {
	o, j, _ := setup(t)
	o.installed, _ = trustprofile.OpenInstallation(o.release.Anchor(), &uncertainNativeCommit{Journal: j, armed: true}, time.Now)
	err := o.MaintainDurable(context.Background(), func(context.Context, uint64) error { t.Fatal("action on uncertain commit"); return nil })
	if err == nil {
		t.Fatal("uncertain maintenance succeeded")
	}
	if _, err := trustprofile.ReadMaintenance(o.release.Anchor(), j, time.Now); err != nil {
		t.Fatal("durable marker missing", err)
	}
	if o.MaintainDurable(context.Background(), func(context.Context, uint64) error { t.Fatal("uncertain retry"); return nil }) == nil {
		t.Fatal("uncertain owner reused")
	}
}

func TestDurableMaintenanceTimeoutKeepsIntentAndOnlyResumesJoin(t *testing.T) {
	o, j, _ := setup(t)
	finish := make(chan struct{})
	defer func() {
		select {
		case <-finish:
		default:
			close(finish)
		}
	}()
	h := mustStart(t, o, func(ctx context.Context, _ trustprofile.Profile) error { <-ctx.Done(); <-finish; return nil })
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	calls := 0
	action := func(context.Context, uint64) error { calls++; return nil }
	if o.MaintainDurable(ctx, action) == nil || calls != 0 {
		t.Fatal("unjoined maintenance acknowledged")
	}
	before, err := trustprofile.ReadMaintenance(o.release.Anchor(), j, time.Now)
	if err != nil || before.State != "pending" {
		t.Fatal("timeout lost durable intent", err)
	}
	if o.Maintain(context.Background(), action) == nil {
		t.Fatal("mode switch bypassed intent")
	}
	close(finish)
	_ = h.Wait(context.Background())
	if err = o.MaintainDurable(context.Background(), action); err != nil || calls != 1 {
		t.Fatal("join did not resume", err)
	}
	after, err := trustprofile.ReadMaintenance(o.release.Anchor(), j, time.Now)
	if err != nil || after != before {
		t.Fatal("join resume replaced transaction", err)
	}
}
