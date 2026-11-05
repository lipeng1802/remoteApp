package installeddevice

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/clientruntime"
	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/trustinstall"
	"remoteapp.local/selfhost-poc/trustprofile"
)

func TestDesktopCommandPolicyIsExactAndDefaultRemainsFixed(t *testing.T) {
	for _, scope := range []invite.DesktopPermission{invite.DesktopView, invite.DesktopControl} {
		for _, b := range []invite.Body{{Action: "register"}, {Action: "incoming"}, {Action: "invite", Permission: scope}, {Action: "claim", Permission: scope, TargetCode: "target", Code: "12345678"}, {Action: "approve", Permission: scope, RequestID: "request", Allow: true}, {Action: "approve", RequestID: "request"}, {Action: "status", RequestID: "request"}, {Action: "revoke", GrantID: "grant"}} {
			r := Request{Op: "device", Device: b}
			if !ValidDesktopSupervisedCommand(r, scope) {
				t.Fatal("valid desktop request rejected", b.Action)
			}
			if b.Permission != "" && ValidSupervisedCommand(r) {
				t.Fatal("default silently upgraded")
			}
			for _, bad := range []Request{
				{Op: r.Op, Device: b, VaultScope: "injected"}, {Op: r.Op, Device: b, ExpectedPublic: "injected"}, {Op: r.Op, Device: b, Network: invite.NetworkRequestBody{Action: "state"}},
			} {
				if ValidDesktopSupervisedCommand(bad, scope) {
					t.Fatal("identity/network selector admitted")
				}
			}
			for _, mutate := range []func(*invite.Body){func(b *invite.Body) { b.Nonce = "injected" }, func(b *invite.Body) { b.IssuedAt = 1 }, func(b *invite.Body) { b.Challenge = "injected" }, func(b *invite.Body) { b.NodeKey = "injected" }} {
				x := r
				mutate(&x.Device)
				if ValidDesktopSupervisedCommand(x, scope) {
					t.Fatal("signing/proof selector admitted")
				}
			}
		}
		for _, action := range []string{"invite", "claim", "approve"} {
			for _, other := range []invite.DesktopPermission{"", "clipboard", "fixed-payload", invite.DesktopView, invite.DesktopControl} {
				b := invite.Body{Action: action, Permission: other}
				if action == "claim" {
					b.TargetCode, b.Code = "target", "12345678"
				}
				if action == "approve" {
					b.Allow, b.RequestID = true, "request"
				}
				if ValidDesktopSupervisedCommand(Request{Op: "device", Device: b}, scope) != (other == scope) {
					t.Fatal("permission inferred from peer", action)
				}
			}
		}
		for _, op := range []string{"stop", "open", "start_controller", "stop_fault", "product_count", "maintenance_prepare"} {
			if ValidDesktopSupervisedCommand(Request{Op: op}, scope) {
				t.Fatal("lifecycle became business")
			}
		}
		g := invite.Grant{Claims: invite.Claims{Permission: string(scope)}, Signature: "fixture"}
		for _, op := range []string{"provision", "network"} {
			r := Request{Op: op, Network: invite.NetworkRequestBody{Grant: g}}
			if op == "network" {
				r.Network.Action = "grant_clock"
			}
			if !ValidDesktopSupervisedCommand(r, scope) {
				t.Fatal("scoped grant structure rejected")
			}
			r.Network.Grant.Claims.Permission = "fixed-payload"
			if ValidDesktopSupervisedCommand(r, scope) {
				t.Fatal("fixed grant reached desktop entry")
			}
		}
	}
}

func TestDesktopResultsBindActionScopeAndExactShape(t *testing.T) {
	for _, scope := range []invite.DesktopPermission{invite.DesktopView, invite.DesktopControl} {
		req := strings.Repeat("a", 32)
		g := invite.Grant{Claims: invite.Claims{Permission: string(scope)}}
		for _, tc := range []struct {
			b invite.Body
			r invite.Result
		}{
			{invite.Body{Action: "register"}, invite.Result{Status: "registered", DeviceID: strings.Repeat("a", 64), DeviceCode: strings.Repeat("b", 12)}},
			{invite.Body{Action: "invite", Permission: scope}, invite.Result{Status: "inviting", InviteID: req, Code: "12345678", Permission: scope}},
			{invite.Body{Action: "claim", Permission: scope, TargetCode: "target", Code: "12345678"}, invite.Result{Status: "pending", RequestID: req, Permission: scope}},
			{invite.Body{Action: "incoming"}, invite.Result{Status: "pending", RequestID: req, ControllerID: strings.Repeat("c", 64), Permission: scope}},
			{invite.Body{Action: "approve", Permission: scope, RequestID: req, Allow: true}, invite.Result{Status: "approved", RequestID: req, Permission: scope, Grant: &g}},
			{invite.Body{Action: "approve", RequestID: req}, invite.Result{Status: "denied", RequestID: req, Permission: scope}},
			{invite.Body{Action: "status", RequestID: req}, invite.Result{Status: "revoked", RequestID: req, Permission: scope}},
			{invite.Body{Action: "status", RequestID: req}, invite.Result{Status: "expired", RequestID: req, Permission: scope}},
		} {
			if !ValidDesktopDeviceResult(scope, tc.b, tc.r) {
				t.Fatal("correct reply rejected", tc.b.Action)
			}
			x := tc.r
			x.Retirement = &invite.NodeRetirement{}
			if ValidDesktopDeviceResult(scope, tc.b, x) {
				t.Fatal("mixed receipt admitted")
			}
			if tc.r.Permission != "" {
				x = tc.r
				x.Permission = ""
				if ValidDesktopDeviceResult(scope, tc.b, x) {
					t.Fatal("missing scope admitted")
				}
				x.Permission = "clipboard"
				if ValidDesktopDeviceResult(scope, tc.b, x) {
					t.Fatal("foreign scope admitted")
				}
			}
			if tc.b.RequestID != "" {
				x = tc.r
				x.RequestID = "foreign"
				if ValidDesktopDeviceResult(scope, tc.b, x) {
					t.Fatal("foreign request admitted")
				}
			}
		}
		if ValidDesktopDeviceResult(scope, invite.Body{Action: "approve", RequestID: req}, invite.Result{Status: "approved", RequestID: req, Permission: scope, Grant: &g}) {
			t.Fatal("denial became approval")
		}
		if !ValidDesktopDeviceResult(scope, invite.Body{Action: "incoming"}, invite.Result{Status: "idle"}) {
			t.Fatal("idle UI state rejected")
		}
	}
}

func TestDesktopRunnerInvalidCompositionNeverOpensSigner(t *testing.T) {
	var opened atomic.Int32
	open := func() (clientruntime.Signer, error) { opened.Add(1); return nil, invite.ErrDenied }
	factory := func(context.Context) (EnrollmentNode, error) { t.Fatal("node opened"); return nil, invite.ErrDenied }
	work := func(context.Context, *SessionRuntime) error { t.Fatal("work started"); return nil }
	for _, scope := range []invite.DesktopPermission{"", "clipboard", invite.DesktopControl} {
		run := CoordinatedDesktopSession(trustinstall.Release{}, open, make(chan Request), func(any) error { return nil }, factory, work, scope)
		if run(context.Background(), trustprofile.Profile{}) == nil {
			t.Fatal("unconfigured release admitted")
		}
	}
	if run := CoordinatedDesktopSessionViaTestTunnel(trustinstall.Release{}, open, nil, nil, factory, work, invite.DesktopView, 443); run(context.Background(), trustprofile.Profile{}) == nil {
		t.Fatal("uncompiled tunnel admitted")
	}
	if opened.Load() != 0 {
		t.Fatal("invalid composition touched signer")
	}
	release, profile := desktopHTTPSFixture(t)
	for _, scope := range []invite.DesktopPermission{"", "clipboard"} {
		if run := CoordinatedDesktopSession(release, open, make(chan Request), func(any) error { return nil }, factory, work, scope); run(context.Background(), profile) == nil {
			t.Fatal("invalid scope on valid release admitted")
		}
	}
	for _, missingWork := range []bool{true, false} {
		f, w := factory, work
		if missingWork {
			w = nil
		} else {
			f = nil
		}
		if run := CoordinatedDesktopSession(release, open, make(chan Request), func(any) error { return nil }, f, w, invite.DesktopView); run(context.Background(), profile) == nil {
			t.Fatal("partial desktop composition admitted")
		}
	}
	if opened.Load() != 0 {
		t.Fatal("invalid desktop entry acquired signer")
	}
}

func TestDesktopSessionUsesScopedGateAndKeepsControllerStartOneShot(t *testing.T) {
	for _, scope := range []invite.DesktopPermission{invite.DesktopView, invite.DesktopControl} {
		g, grant, issuer, e, binding := watchedFixture(t)
		_, target, _ := ed25519.GenerateKey(rand.Reader)
		grant.Claims.Permission = string(scope)
		grant.Claims.Controller = g.self
		id := sha256.Sum256(target.Public().(ed25519.PublicKey))
		grant.Claims.Target = hex.EncodeToString(id[:])
		grant.Signature = testSignature(issuer, "remoteapp-invite-grant-v1\n", grant.Claims)
		g.policy, _ = invite.DesktopGrantPolicy(scope)
		g.grant = grant
		binding.TargetNode, binding.ControllerNode = binding.ControllerNode, binding.TargetNode
		binding.TargetIP, binding.ControllerIP = binding.ControllerIP, binding.TargetIP
		var identities atomic.Int32
		client := requestFunc(func(_ context.Context, b invite.NetworkRequestBody) (invite.NetworkReply, error) {
			if b.Grant != grant {
				t.Error("grant changed")
			}
			if b.Action == "session_identity" {
				identities.Add(1)
				now := g.clock()
				c := invite.SessionIdentityClaims{Version: 1, Grant: grant, Binding: binding, TargetPublic: hex.EncodeToString(target.Public().(ed25519.PublicKey)), ControllerPublic: hex.EncodeToString(g.public), Challenge: b.Challenge, IssuedAt: now.UnixNano(), ExpiresAt: now.Add(3 * time.Second).UnixNano()}
				return invite.NetworkReply{Session: &invite.SessionIdentity{Claims: c, Signature: testSignature(issuer, "remoteapp-session-identity-v1\n", c)}}, nil
			}
			return watchState(issuer, grant, binding, true, g.clock()), nil
		})
		if _, err := e.startWatch(context.Background(), g, client); err != nil {
			t.Fatal(err)
		}
		defer e.closeJoined()
		start := controllerStart{pending: true}
		entered := make(chan struct{})
		work := func(ctx context.Context, r *SessionRuntime) error {
			defer close(entered)
			if r.policy != g.policy || r.gate == nil {
				t.Error("policy lost before payload")
			}
			if r.RunProduct(ctx, clientruntime.Product{}) == nil || r.RunDesktopProduct(ctx, "clipboard", clientruntime.Product{}) == nil {
				t.Error("scope upgraded")
			}
			<-ctx.Done()
			return ctx.Err()
		}
		if identities.Load() != 0 || e.peers != nil {
			t.Fatal("pre-start identity")
		}
		if err := start.run(context.Background(), e, g, client, work); err != nil {
			t.Fatal(err)
		}
		if start.run(context.Background(), e, g, client, work) == nil || identities.Load() != 1 {
			t.Fatal("duplicate identity")
		}
		e.session.Stop()
		<-entered
		if e.watch.Faulted() {
			t.Fatal("normal stop became watch failure")
		}
	}
}
