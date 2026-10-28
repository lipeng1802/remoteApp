package clientowner

import (
	"bytes"
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/trustprofile"
)

func TestMaintenanceJoinsSealsAndPreservesJournal(t *testing.T) {
	o, j, next := setup(t)
	before := append([]byte(nil), j.raw...)
	cancelled, finish := make(chan struct{}), make(chan struct{})
	h := mustStart(t, o, func(ctx context.Context, _ trustprofile.Profile) error {
		<-ctx.Done()
		close(cancelled)
		<-finish
		return nil
	})
	var calls atomic.Int32
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	action := func(context.Context, uint64) error { calls.Add(1); return nil }
	if o.Maintain(ctx, action) == nil {
		t.Fatal("unjoined maintenance succeeded")
	}
	<-cancelled
	if calls.Load() != 0 || j.closed {
		t.Fatal("action/lease advanced before join")
	}
	if _, err := o.Start(context.Background(), func(context.Context, trustprofile.Profile) error { return nil }); err == nil {
		t.Fatal("maintenance reopened session")
	}
	if o.Upgrade(context.Background(), next) == nil {
		t.Fatal("maintenance allowed upgrade")
	}
	close(finish)
	if h.Wait(context.Background()) != nil {
		t.Fatal("join failed")
	}
	if err := o.Maintain(context.Background(), func(_ context.Context, epoch uint64) error {
		calls.Add(1)
		if epoch != 1 || j.closed || !h.State().Joined {
			t.Fatal("invalid maintenance lease")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, j.raw) || j.commits != 1 {
		t.Fatal("maintenance changed durable state")
	}
	if o.Maintain(context.Background(), action) == nil || calls.Load() != 1 {
		t.Fatal("action repeated")
	}
	if _, err := o.InstalledEpoch(context.Background()); err == nil {
		t.Fatal("sealed owner advertised usable state")
	}
	if err := o.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestMaintenanceCloseCannotOvertakeAction(t *testing.T) {
	o, j, _ := setup(t)
	entered, finish := make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- o.Maintain(context.Background(), func(context.Context, uint64) error { close(entered); <-finish; return errors.New("uncertain action") })
	}()
	<-entered
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if o.Close(ctx) == nil {
		t.Fatal("close passed running action")
	}
	j.mu.Lock()
	closed := j.closed
	j.mu.Unlock()
	if closed {
		t.Fatal("native lease released during action")
	}
	close(finish)
	if <-done == nil {
		t.Fatal("action fault ignored")
	}
	if o.Maintain(context.Background(), func(context.Context, uint64) error { t.Fatal("repeated action"); return nil }) == nil {
		t.Fatal("uncertain action reused")
	}
	if o.Close(context.Background()) != nil {
		t.Fatal("close after joined action")
	}
}

func TestMaintenanceFaultAndInvalidActionNeverRun(t *testing.T) {
	o, _, _ := setup(t)
	if o.Maintain(context.Background(), nil) == nil {
		t.Fatal("nil action")
	}
	h := mustStart(t, o, func(context.Context, trustprofile.Profile) error { return errors.New("runner fault") })
	_ = h.Wait(context.Background())
	if o.Maintain(context.Background(), func(context.Context, uint64) error { t.Fatal("fault granted maintenance"); return nil }) == nil {
		t.Fatal("fault accepted")
	}
}
