package productmaintenance

import (
	"context"
	"remoteapp.local/selfhost-poc/installeddevice"
	"remoteapp.local/selfhost-poc/invite"
	"testing"
	"time"
)

func TestQuiesceBeforeProvisionDoesNotStopIdleOwner(t *testing.T) {
	c, s, _ := controllerFixture(t, "forward")
	c.prepared = false
	c.revocation = &revokeFixture{}
	if _, err := c.Forward(context.Background(), installeddevice.Request{Op: "quiesce_session"}); err == nil {
		t.Fatal("idle owner stopped")
	}
	if _, err := c.Forward(context.Background(), installeddevice.Request{Op: "device", Device: invite.Body{Action: "register"}}); err != nil {
		t.Fatal("idle owner poisoned", err)
	}
	if s.closes.Load() != 0 {
		t.Fatal("idle signer released")
	}
}

func TestQuiesceJoinsProductRetainsRevocationAndSealsReuse(t *testing.T) {
	c, signer, _ := controllerFixture(t, "normal")
	c.prepared = false
	c.provisioned = true
	rev := &revokeFixture{}
	c.revocation = rev
	if _, err := c.Forward(context.Background(), installeddevice.Request{Op: "quiesce_session", VaultScope: "foreign"}); err == nil {
		t.Fatal("selector accepted")
	}
	for i := 0; i < 2; i++ {
		r, err := c.Forward(context.Background(), installeddevice.Request{Op: "quiesce_session"})
		if err != nil || string(r) != `{"status":"session_quiesced","joined":true,"zero_exit":true}` {
			t.Fatal("quiesce receipt", err)
		}
	}
	select {
	case <-c.child.Done():
	default:
		t.Fatal("receipt before actual exit")
	}
	if signer.closes.Load() != 0 || rev.closes != 0 {
		t.Fatal("revocation signer retired early")
	}
	if _, err := c.Count(context.Background()); err == nil {
		t.Fatal("joined product counted as active")
	}
	if _, err := c.Forward(context.Background(), installeddevice.Request{Op: "revoke_session"}); err != nil || rev.revokes != 1 {
		t.Fatal("independent revoke unavailable")
	}
	if err := c.Release(context.Background()); err != nil {
		t.Fatal(err)
	}
	if signer.closes.Load() != 1 || rev.closes != 1 {
		t.Fatal("final signer join missing")
	}
	if _, err := c.Forward(context.Background(), installeddevice.Request{Op: "quiesce_session"}); err == nil {
		t.Fatal("released owner reused")
	}
}

func TestQuiesceCannotBlessTimeoutOrFaultyExit(t *testing.T) {
	for _, mode := range []string{"slow", "nonzero", "duplicate", "extra"} {
		t.Run(mode, func(t *testing.T) {
			c, s, _ := controllerFixture(t, mode)
			c.prepared = false
			c.provisioned = true
			c.revocation = &revokeFixture{}
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
			defer cancel()
			if _, err := c.Forward(ctx, installeddevice.Request{Op: "quiesce_session"}); err == nil {
				t.Fatal("failed join accepted")
			}
			if s.closes.Load() != 0 {
				t.Fatal("signer released by failed quiesce")
			}
			c.Release(context.Background())
			if _, err := c.Forward(context.Background(), installeddevice.Request{Op: "quiesce_session"}); err == nil {
				t.Fatal("late exit rearmed receipt")
			}
		})
	}
}
