//go:build linux || darwin

package broker

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/invite"
)

func desktopSupervisorFixture(t *testing.T, d *supervisorDriver) (*NetworkSupervisor, *invite.Store, *liveDependency) {
	t.Helper()
	s, st, v := startDesktopSupervisorFixture(t, d)
	waitSupervisor(t, s.Ready)
	return s, st, v
}

func startDesktopSupervisorFixture(t *testing.T, d invite.ProvisionDriver) (*NetworkSupervisor, *invite.Store, *liveDependency) {
	t.Helper()
	dir, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	st, e := invite.OpenDesktopAuthorization(filepath.Join(dir, "authority"), time.Now)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { st.Close() })
	b, e := headscaleBackend(st, d, true)
	if e != nil {
		t.Fatal(e)
	}
	v := &liveDependency{done: make(chan struct{})}
	v.live.Store(true)
	s, e := StartNetworkSupervisor(context.Background(), b, v, 100*time.Millisecond)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if s.Stop(ctx) != nil {
			t.Error("desktop supervisor did not join")
		}
	})
	return s, st, v
}

type firstCycleDriver struct {
	supervisorDriver
	writes    atomic.Int32
	entered   chan struct{}
	release   chan struct{}
	failCycle bool
}

func (d *firstCycleDriver) ReplaceRules(ctx context.Context, rules []invite.NetworkRule) error {
	// The initial reconciliation makes two writes. Hold the first write
	// of the next reconciliation, which runs in the maintenance worker.
	if d.writes.Add(1) == 3 {
		close(d.entered)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-d.release:
		}
		if d.failCycle {
			return invite.ErrDenied
		}
	}
	return d.supervisorDriver.ReplaceRules(ctx, rules)
}

func TestDesktopSupervisorStartupWaitsForFirstPolicyCycle(t *testing.T) {
	for _, tc := range []struct {
		name      string
		failCycle bool
	}{
		{name: "delayed_success"},
		{name: "first_cycle_failure", failCycle: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := &firstCycleDriver{
				entered: make(chan struct{}), release: make(chan struct{}), failCycle: tc.failCycle,
			}
			s, _, _ := startDesktopSupervisorFixture(t, d)
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			select {
			case <-d.entered:
			case <-ctx.Done():
				t.Fatal("startup maintenance did not reach the policy driver")
			}
			if s.Ready() {
				t.Fatal("policy projection is still in flight")
			}
			result := make(chan bool, 1)
			go func() { result <- s.WaitReady(ctx) }()
			select {
			case <-result:
				t.Fatal("startup returned before the first policy cycle finished")
			case <-time.After(20 * time.Millisecond):
			}
			canceled, stop := context.WithCancel(ctx)
			stop()
			if s.WaitReady(canceled) {
				t.Fatal("canceled waiter reported ready")
			}
			close(d.release)
			select {
			case ready := <-result:
				if ready == tc.failCycle {
					t.Fatalf("startup ready = %t with first cycle failure = %t", ready, tc.failCycle)
				}
			case <-ctx.Done():
				t.Fatal("startup did not observe the first policy result")
			}
		})
	}
}

func desktopNonce(t *testing.T) string {
	t.Helper()
	var b [16]byte
	if _, e := rand.Read(b[:]); e != nil {
		t.Fatal(e)
	}
	return hex.EncodeToString(b[:])
}

func TestDesktopHeadscaleCompositionRequiresExplicitMatchingStore(t *testing.T) {
	for _, desktopStore := range []bool{false, true} {
		dir, e := filepath.EvalSymlinks(t.TempDir())
		if e != nil {
			t.Fatal(e)
		}
		open := invite.Open
		if desktopStore {
			open = invite.OpenDesktopAuthorization
		}
		st, e := open(filepath.Join(dir, "authority"), time.Now)
		if e != nil {
			t.Fatal(e)
		}
		for _, selected := range []bool{!desktopStore, desktopStore} {
			d := &supervisorDriver{}
			b, e := headscaleBackend(st, d, selected)
			if e == nil {
				v := &liveDependency{done: make(chan struct{})}
				v.live.Store(true)
				s, startErr := StartNetworkSupervisor(context.Background(), b, v, 100*time.Millisecond)
				if (startErr == nil) != (selected == desktopStore) {
					t.Fatal("mode inferred from Store")
				}
				if s != nil {
					ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
					if s.Stop(ctx) != nil || !s.DenyAllConfirmed() {
						t.Fatal("matching mode did not join/withdraw")
					}
					cancel()
				}
			} else if selected == desktopStore {
				t.Fatal("matching mode rejected")
			}
			if selected != desktopStore && d.zero.Load() != 0 {
				t.Fatal("wrong mode invoked privileged rule writer")
			}
		}
		st.Close()
	}
	if s, e := StartDesktopHeadscaleNetworkSupervisor(context.Background(), nil, nil, "/", time.Second); s != nil || e == nil {
		t.Fatal("unowned runtime accepted")
	}
}

func TestDesktopScopedHTTPSViaPrivateBrokerAndRetainedDeviceAuthority(t *testing.T) {
	for _, scope := range []invite.DesktopPermission{invite.DesktopView, invite.DesktopControl} {
		t.Run(string(scope), func(t *testing.T) {
			d := &supervisorDriver{}
			s, st, _ := desktopSupervisorFixture(t, d)
			path := socket(t)
			cancel, done := start(t, Config{Socket: path, PeerUID: uint32(os.Geteuid()), GroupID: uint32(os.Getegid()), Executor: s.Authority()})
			defer stop(t, cancel, done)
			h := httptest.NewTLSServer(invite.NewAdmission().Wrap(PrivateNetworkHandler(Client{Socket: path, ServerUID: uint32(os.Geteuid())}, hex.EncodeToString(st.PublicKey()))))
			defer h.Close()
			_, owner, _ := ed25519.GenerateKey(rand.Reader)
			_, peer, _ := ed25519.GenerateKey(rand.Reader)
			newClient := func(key ed25519.PrivateKey) *invite.NetworkClient {
				c, e := invite.NewNetworkClient(h.URL, h.Certificate().Raw, key)
				if e != nil {
					t.Fatal(e)
				}
				t.Cleanup(c.Close)
				return c
			}
			target, controller := newClient(owner), newClient(peer)
			call := func(c *invite.NetworkClient, b invite.Body) invite.Result {
				r, e := c.DeviceRequest(context.Background(), b)
				if e != nil {
					t.Fatal("signed device operation rejected", b.Action)
				}
				return r
			}
			targetID := call(target, invite.Body{Action: "register"})
			call(controller, invite.Body{Action: "register"})
			inv := call(target, invite.Body{Action: "invite", Permission: scope})
			pending := call(controller, invite.Body{Action: "claim", Permission: scope, TargetCode: targetID.DeviceCode, Code: inv.Code})
			if _, e := controller.DeviceRequest(context.Background(), invite.Body{Action: "approve", Permission: scope, RequestID: pending.RequestID, Allow: true}); e == nil {
				t.Fatal("self approval admitted through broker")
			}
			if _, e := target.DeviceRequest(context.Background(), invite.Body{Action: "approve", RequestID: pending.RequestID, Allow: true}); e == nil {
				t.Fatal("implicit desktop consent")
			}
			approved := call(target, invite.Body{Action: "approve", Permission: scope, RequestID: pending.RequestID, Allow: true})
			if approved.Grant == nil || approved.Permission != scope {
				t.Fatal("approved scope lost in IPC")
			}
			g := *approved.Grant
			status := call(controller, invite.Body{Action: "status", RequestID: pending.RequestID})
			if status.Grant == nil || *status.Grant != g || status.Permission != scope {
				t.Fatal("status scope lost in IPC")
			}
			startTime := time.Now()
			proof, e := controller.Request(context.Background(), invite.NetworkRequestBody{Action: "grant_clock", Grant: g, Challenge: strings.Repeat("c", 64)})
			if e != nil || proof.Proof == nil {
				t.Fatal("scoped backend not reached")
			}
			if _, e = invite.AuthorityClock(st.PublicKey(), *proof.Proof, g.Claims.ID, strings.Repeat("c", 64), startTime); e != nil {
				t.Fatal("calibration signature")
			}
			// A private frame cannot select another local mode or extend its scope.
			altered := g
			altered.Claims.Permission = "fixed-payload"
			if _, e = controller.Request(context.Background(), invite.NetworkRequestBody{Action: "grant_clock", Grant: altered, Challenge: strings.Repeat("d", 64)}); e == nil {
				t.Fatal("unsigned scope substitution")
			}
			target.Close()
			ctx, finish := context.WithTimeout(context.Background(), 3*time.Second)
			defer finish()
			if s.Stop(ctx) != nil || !s.DenyAllConfirmed() {
				t.Fatal("supervisor did not actually join/withdraw")
			}
			select {
			case <-s.Done():
			default:
				t.Fatal("false join")
			}
			if _, e = controller.Request(context.Background(), invite.NetworkRequestBody{Action: "grant_clock", Grant: g, Challenge: strings.Repeat("e", 64)}); e == nil {
				t.Fatal("network gate reopened")
			}
			independent := newClient(owner)
			if r := call(independent, invite.Body{Action: "revoke", GrantID: g.Claims.ID}); r != (invite.Result{Status: "revoked"}) {
				t.Fatal("retained independent revoke unavailable")
			}
			if r := call(controller, invite.Body{Action: "status", RequestID: pending.RequestID}); r != (invite.Result{Status: "revoked", RequestID: pending.RequestID, Permission: scope}) {
				t.Fatal("retained scoped terminal proof unavailable")
			}
		})
	}
}

func TestDesktopVerifierFaultCannotOutrunMintJoinOrLeakCredential(t *testing.T) {
	for _, scope := range []invite.DesktopPermission{invite.DesktopView, invite.DesktopControl} {
		t.Run(string(scope), func(t *testing.T) {
			d := &supervisorDriver{mintStarted: make(chan struct{}), release: make(chan struct{})}
			s, st, v := desktopSupervisorFixture(t, d)
			_, owner, _ := ed25519.GenerateKey(rand.Reader)
			_, peer, _ := ed25519.GenerateKey(rand.Reader)
			call := func(k ed25519.PrivateKey, b invite.Body) invite.Result {
				b.Nonce, b.IssuedAt = desktopNonce(t), time.Now().Unix()
				r, e := st.Execute(invite.Sign(k, b))
				if e != nil {
					t.Fatal("device fixture rejected", b.Action)
				}
				return r
			}
			id := call(owner, invite.Body{Action: "register"})
			call(peer, invite.Body{Action: "register"})
			inv := call(owner, invite.Body{Action: "invite", Permission: scope})
			pending := call(peer, invite.Body{Action: "claim", Permission: scope, TargetCode: id.DeviceCode, Code: inv.Code})
			approved := call(owner, invite.Body{Action: "approve", Permission: scope, RequestID: pending.RequestID, Allow: true})
			if approved.Grant == nil {
				t.Fatal("grant missing")
			}
			g := *approved.Grant
			intent := invite.SignProvision(owner, invite.ProvisionBody{GrantID: g.Claims.ID, NodeKey: "nodekey:" + strings.Repeat("a", 64), Nonce: desktopNonce(t), IssuedAt: time.Now().Unix()})
			req := invite.SignNetworkRequest(owner, invite.NetworkRequestBody{Action: "enroll", Grant: g, Intent: intent, Nonce: desktopNonce(t), IssuedAt: time.Now().Unix()})
			result := make(chan Reply, 1)
			go func() {
				result <- s.Authority().Execute(context.Background(), Request{Operation: "network", Network: &req})
			}()
			// Always release a failed fixture so cleanup never abandons its owner.
			released := false
			defer func() {
				if !released {
					close(d.release)
				}
			}()
			select {
			case <-d.mintStarted:
			case <-time.After(3 * time.Second):
				t.Fatal("mint not reached")
			}
			v.live.Store(false)
			close(v.done)
			waitSupervisor(t, d.canceled.Load)
			short, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
			if s.Stop(short) == nil {
				t.Fatal("in-flight Mint falsely joined")
			}
			cancel()
			select {
			case <-s.Done():
				t.Fatal("owner released before worker")
			default:
			}
			close(d.release)
			released = true
			select {
			case r := <-result:
				if r.Status == 200 || r.Network != nil {
					t.Fatal("credential escaped canceled scope")
				}
			case <-time.After(3 * time.Second):
				t.Fatal("request did not join")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if s.Stop(ctx) != nil || !s.DenyAllConfirmed() || s.Ready() {
				t.Fatal("fault cleanup/join proof missing")
			}
		})
	}
}
