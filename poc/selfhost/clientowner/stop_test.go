package clientowner

import (
	"context"
	"remoteapp.local/selfhost-poc/trustprofile"
	"testing"
	"time"
)

func TestStopRetainsLeaseAndAllowsExplicitRestart(t *testing.T) {
	o, j, _ := setup(t)
	h, e := o.Start(context.Background(), func(ctx context.Context, _ trustprofile.Profile) error { <-ctx.Done(); return nil })
	if e != nil {
		t.Fatal(e)
	}
	if e = o.Stop(context.Background()); e != nil {
		t.Fatal(e)
	}
	if e = h.Wait(context.Background()); e != nil {
		t.Fatal(e)
	}
	j.mu.Lock()
	closed := j.closed
	j.mu.Unlock()
	if closed {
		t.Fatal("stop released journal")
	}
	if _, e = o.Start(context.Background(), func(ctx context.Context, _ trustprofile.Profile) error { <-ctx.Done(); return nil }); e != nil {
		t.Fatal(e)
	}
}
func TestStopTimeoutCannotHealOwner(t *testing.T) {
	o, j, _ := setup(t)
	release := make(chan struct{})
	h, e := o.Start(context.Background(), func(context.Context, trustprofile.Profile) error { <-release; return nil })
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	if o.Stop(ctx) == nil {
		t.Fatal("unjoined stop succeeded")
	}
	j.mu.Lock()
	closed := j.closed
	j.mu.Unlock()
	if closed {
		t.Fatal("released lease before join")
	}
	close(release)
	h.Wait(context.Background())
	if e = o.Stop(context.Background()); e != nil {
		t.Fatal(e)
	}
	if _, e = o.Start(context.Background(), func(context.Context, trustprofile.Profile) error { return nil }); e == nil {
		t.Fatal("timeout owner healed")
	}
}
