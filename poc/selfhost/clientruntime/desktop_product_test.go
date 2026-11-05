package clientruntime

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/invite"
)

func desktopProductAPI(t *testing.T, scope invite.DesktopPermission) (*productAPI, ed25519.PublicKey) {
	t.Helper()
	a, pub := productFixture(t)
	a.c.Grant.Claims.Permission = string(scope)
	raw, _ := json.Marshal(a.c.Grant.Claims)
	a.c.Grant.Signature = hex.EncodeToString(ed25519.Sign(a.key, append([]byte("remoteapp-invite-grant-v1\n"), raw...)))
	return a, pub
}

func TestDesktopProductPrivateHandoffAndRoleScopeReadiness(t *testing.T) {
	for _, scope := range []invite.DesktopPermission{invite.DesktopView, invite.DesktopControl} {
		for _, server := range []bool{true, false} {
			a, pub := desktopProductAPI(t, scope)
			mode := "controller"
			if server {
				mode = "target"
			}
			p := Product{Spec: productSpec(t, string(scope)+"-"+mode)}
			if server {
				p.TargetReady = func(string, string) {}
			} else {
				p.ControllerKey = testHMAC
			}
			child, err := StartBoundDesktopProduct(context.Background(), scope, p, a, pub, a.c.Grant, a.c.Binding, server, time.Now, 32123, "")
			if err != nil || child == nil {
				stopProduct(t, child)
				t.Fatal("scoped private handoff rejected", err)
			}
			stopProduct(t, child)
		}
	}
}

func TestDesktopProductRejectsLegacyScopeAndLegacyReadiness(t *testing.T) {
	a, pub := desktopProductAPI(t, invite.DesktopControl)
	p := Product{Spec: productSpec(t, "controller"), ControllerKey: testHMAC}
	p.Observation = &ProductObservation{}
	observed, observedErr := StartBoundDesktopProduct(context.Background(), invite.DesktopControl, p, a, pub, a.c.Grant, a.c.Binding, false, time.Now, 32123, "")
	if observed != nil {
		stopProduct(t, observed)
	}
	if observedErr == nil || observed != nil {
		t.Fatal("legacy PONG observation admitted as desktop evidence")
	}
	p.Observation = nil
	for _, scope := range []invite.DesktopPermission{invite.DesktopView, "fixed-payload", ""} {
		child, err := StartBoundDesktopProduct(context.Background(), scope, p, a, pub, a.c.Grant, a.c.Binding, false, time.Now, 32123, "")
		if child != nil {
			stopProduct(t, child)
		}
		if err == nil || child != nil {
			t.Fatal("scope mismatch started child")
		}
	}
	if child, err := StartBoundProduct(context.Background(), p, a, pub, a.c.Grant, a.c.Binding, false, time.Now, 32123, ""); err == nil || child != nil {
		stopProduct(t, child)
		t.Fatal("legacy entry accepted desktop grant")
	}
	// A legacy child must be joined when it cannot echo the desktop permission.
	child, err := StartBoundDesktopProduct(context.Background(), invite.DesktopControl, p, a, pub, a.c.Grant, a.c.Binding, false, time.Now, 32123, "")
	if child != nil {
		stopProduct(t, child)
	}
	if err == nil {
		t.Fatal("legacy PONG readiness upgraded")
	}
	p = Product{Spec: productSpec(t, "desktop-view-target"), TargetReady: func(string, string) { t.Error("mismatched readiness released secret") }}
	child, err = StartBoundDesktopProduct(context.Background(), invite.DesktopControl, p, a, pub, a.c.Grant, a.c.Binding, true, time.Now, 0, "")
	if child != nil {
		stopProduct(t, child)
	}
	if err == nil || a.published {
		t.Fatal("wrong target scope published")
	}
}
