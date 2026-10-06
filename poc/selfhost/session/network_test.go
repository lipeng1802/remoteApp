package session

import (
	"context"
	"strings"
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/invite"
)

func TestSignedStateExpiresAndRevokeReplayCannotRestore(t *testing.T) {
	f := setup(t)
	b := invite.NetworkBinding{TargetNode: "nodekey:" + strings.Repeat("a", 64), ControllerNode: "nodekey:" + strings.Repeat("b", 64), TargetIP: "100.120.0.1", ControllerIP: "100.120.0.2"}
	g := f.server.Grant
	n, e := f.s.NetworkState(g, b)
	if e != nil {
		t.Fatal(e)
	}
	c := NewStateCache(f.s.PublicKey(), g, b, f.clock)
	if !c.Update(n) || !c.Check(context.Background(), g, g.Claims.Target, g.Claims.Controller) {
		t.Fatal("valid state rejected")
	}
	tampered := n
	tampered.Claims.Binding.ControllerNode = b.TargetNode
	if c.Update(tampered) {
		t.Fatal("binding tamper accepted")
	}
	f.execute(f.owner, invite.Body{Action: "revoke", GrantID: g.Claims.ID})
	d, e := f.s.NetworkState(g, b)
	if e != nil || d.Claims.Active {
		t.Fatal("revoke not signed")
	}
	if !c.Update(d) || c.Update(n) || c.Check(context.Background(), g, g.Claims.Target, g.Claims.Controller) {
		t.Fatal("revocation replay")
	}
	if err := f.s.Close(); err != nil {
		t.Fatal(err)
	}
	f.s = f.open()
	d, e = f.s.NetworkState(g, b)
	if e != nil || d.Claims.Active {
		t.Fatal("restart revived grant")
	}
	f.now.Add(4)
	if invite.VerifyNetworkState(f.s.PublicKey(), n, f.clock()) {
		t.Fatal("lease expired but valid")
	}
}
func TestAbsoluteLeaseExpiryCannotBeExtendedByReceipt(t *testing.T) {
	f := setup(t)
	b := invite.NetworkBinding{TargetNode: "nodekey:" + strings.Repeat("a", 64), ControllerNode: "nodekey:" + strings.Repeat("b", 64), TargetIP: "100.120.0.1", ControllerIP: "100.120.0.2"}
	g := f.server.Grant
	n, e := f.s.NetworkState(g, b)
	if e != nil {
		t.Fatal(e)
	}
	c := NewStateCache(f.s.PublicKey(), g, b, f.clock)
	f.now.Add(2)
	if !c.Update(n) {
		t.Fatal("valid late receipt")
	}
	f.now.Add(2)
	if c.Update(n) || c.Check(context.Background(), g, g.Claims.Target, g.Claims.Controller) {
		t.Fatal("receipt extended TTL")
	}
	n.Claims.ExpiresAt += int64(time.Hour)
	if c.Update(n) {
		t.Fatal("tampered expiry accepted")
	}
}

func TestSignedStateControlsRealActiveConnection(t *testing.T) {
	for _, kind := range []string{"expiry", "revoke"} {
		t.Run(kind, func(t *testing.T) {
			f := setup(t)
			b := invite.NetworkBinding{TargetNode: "nodekey:" + strings.Repeat("a", 64), ControllerNode: "nodekey:" + strings.Repeat("b", 64), TargetIP: "100.120.0.1", ControllerIP: "100.120.0.2"}
			g := f.server.Grant
			n, e := f.s.NetworkState(g, b)
			if e != nil {
				t.Fatal(e)
			}
			c := NewStateCache(f.s.PublicKey(), g, b, f.clock)
			if !c.Update(n) {
				t.Fatal("initial state")
			}
			r := f.run(c.Check)
			tick(t, r)
			if kind == "expiry" {
				f.now.Add(4)
			} else {
				f.execute(f.owner, invite.Body{Action: "revoke", GrantID: g.Claims.ID})
				n, e = f.s.NetworkState(g, b)
				if e != nil || !c.Update(n) {
					t.Fatal("revocation state")
				}
			}
			ended(t, r)
		})
	}
}

func TestExpiredGrantReceivesOnlyInactiveState(t *testing.T) {
	f := setup(t)
	b := invite.NetworkBinding{TargetNode: "nodekey:" + strings.Repeat("a", 64), ControllerNode: "nodekey:" + strings.Repeat("b", 64), TargetIP: "100.120.0.1", ControllerIP: "100.120.0.2"}
	g := f.server.Grant
	f.now.Add(301)
	n, e := f.s.NetworkState(g, b)
	if e != nil || n.Claims.Active || !invite.VerifyNetworkState(f.s.PublicKey(), n, f.clock()) {
		t.Fatal("expired grant did not receive authenticated inactive state")
	}
	g.Claims.ExpiresAt++
	if _, e = f.s.NetworkState(g, b); e != invite.ErrDenied {
		t.Fatal("tampered expired grant accepted")
	}
}
