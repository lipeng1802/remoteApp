package invite

import (
	"context"
	"strings"
	"testing"
	"time"
)

type dynamicDriver struct{ *memoryDriver }

func (d *dynamicDriver) Observe(ctx context.Context, r ProvisionRecord) (ProvisionObservation, error) {
	o, e := d.memoryDriver.Observe(ctx, r)
	o.IP = "100.120.0.42"
	if r.Device == r.Grant.Claims.Controller {
		o.IP = "100.120.0.93"
	}
	return o, e
}

func TestProvisionNetworkStateUsesVerifiedDynamicPair(t *testing.T) {
	f, g, b, base := provisionFixture(t)
	b.Driver = &dynamicDriver{base}
	if _, e := b.NetworkState(g); e != ErrDenied {
		t.Fatal("state before registration")
	}
	for i, key := range []string{"a", "b"} {
		app := f.owner
		if i == 1 {
			app = f.client
		}
		id := enroll(t, f, g, b, app, key)
		if b.Bind(context.Background(), id) != nil {
			t.Fatal("bind")
		}
		if i == 0 {
			if _, e := b.NetworkState(g); e != ErrDenied {
				t.Fatal("one-sided binding")
			}
		}
	}
	n, e := b.NetworkState(g)
	if e != nil || n.Claims.Active {
		t.Fatal("bound but unapplied policy authorized")
	}
	if b.Reconcile(context.Background()) != nil {
		t.Fatal("reconcile")
	}
	n, e = b.NetworkState(g)
	want := NetworkBinding{"nodekey:" + strings.Repeat("a", 64), "nodekey:" + strings.Repeat("b", 64), "100.120.0.42", "100.120.0.93"}
	if e != nil || !n.Claims.Active || n.Claims.Binding != want || !VerifyNetworkState(f.s.PublicKey(), n, f.now) {
		t.Fatal("dynamic verified pair not signed")
	}
	if n.Claims.ExpiresAt-n.Claims.IssuedAt != int64(3*time.Second) {
		t.Fatal("lease widened")
	}
	g.Claims.Controller = g.Claims.Target
	if _, e := b.NetworkState(g); e != ErrDenied {
		t.Fatal("tampered grant accepted")
	}
}

func TestProvisionNetworkStateFailClosed(t *testing.T) {
	for _, mode := range []string{"revoke", "expiry", "rollback", "policy", "health", "storage", "restart", "cleanup"} {
		t.Run(mode, func(t *testing.T) {
			f, g, b, d := provisionFixture(t)
			ids := bindPair(t, f, g, b)
			switch mode {
			case "revoke":
				f.exec(t, f.owner, Body{Action: "revoke", GrantID: g.Claims.ID})
			case "expiry":
				f.now = f.now.Add(301 * time.Second)
			case "rollback":
				f.now = f.now.Add(-time.Second)
			case "policy":
				d.failPolicy = true
				if b.Reconcile(context.Background()) == nil {
					t.Fatal("policy failure")
				}
			case "health":
				b.Driver = &proofFailureDriver{memoryDriver: d, unhealthy: true}
			case "storage":
				f.s.broken = true
			case "restart":
				f.restart(t)
				b = &ProvisionBackend{Store: f.s, Driver: d}
			case "cleanup":
				if f.s.ProvisionCleanup(ids[0], false) != nil {
					t.Fatal("cleanup")
				}
			}
			n, e := b.NetworkState(g)
			if e == nil && n.Claims.Active {
				t.Fatal("failure authorized state")
			}
			if mode == "restart" {
				if e != nil {
					t.Fatal("restart should sign inactive existing binding")
				}
				if b.Reconcile(context.Background()) != nil {
					t.Fatal("recover")
				}
				n, e = b.NetworkState(g)
				if e != nil || !n.Claims.Active {
					t.Fatal("reconciled restart not recovered")
				}
			}
		})
	}
}

func TestDynamicBindingScope(t *testing.T) {
	b := NetworkBinding{"nodekey:" + strings.Repeat("a", 64), "nodekey:" + strings.Repeat("b", 64), "100.120.0.254", "100.120.0.17"}
	if !validBinding(b) {
		t.Fatal("valid dynamic pair")
	}
	for _, ip := range []string{"100.120.0.0", "100.120.0.255", "100.121.0.1", "127.0.0.1", "::ffff:100.120.0.17", "100.120.00.17", b.TargetIP} {
		v := b
		v.ControllerIP = ip
		if validBinding(v) {
			t.Fatal("unsafe binding accepted")
		}
	}
	b.ControllerNode = "nodekey:" + strings.Repeat("0", 64)
	if validBinding(b) {
		t.Fatal("zero node key")
	}
}

func TestProvisionNetworkStateSharedPolicyFence(t *testing.T) {
	f, g, b, d := provisionFixture(t)
	bindPair(t, f, g, b)
	other := &ProvisionBackend{Store: f.s, Driver: d}
	if other.Reconcile(context.Background()) != nil {
		t.Fatal("other reconcile")
	}
	d.failPolicy = true
	if b.Reconcile(context.Background()) == nil {
		t.Fatal("failure missing")
	}
	n, e := other.NetworkState(g)
	if e != nil || n.Claims.Active {
		t.Fatal("other wrapper bypassed shared fence")
	}
}

func TestProvisionNetworkStateRejectsAmbiguousRecords(t *testing.T) {
	f, g, b, _ := provisionFixture(t)
	ids := bindPair(t, f, g, b)
	// Defense in depth for an inconsistent trusted ledger: do not select an
	// arbitrary map entry and sign it. Normal PlanProvision already forbids this.
	f.s.mu.Lock()
	r := f.s.state.Provisions[ids[0]]
	f.s.state.Provisions["duplicate"] = r
	f.s.mu.Unlock()
	if _, e := b.NetworkState(g); e != ErrDenied {
		t.Fatal("ambiguous binding signed")
	}
}
