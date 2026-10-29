package installeddevice

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/invite"
)

func watchState(k ed25519.PrivateKey, g invite.Grant, b invite.NetworkBinding, active bool, now time.Time) invite.NetworkReply {
	n := invite.NetworkState{Claims: invite.NetworkClaims{Version: 1, Grant: g, Binding: b, Active: active, IssuedAt: now.UnixNano(), ExpiresAt: now.Add(3 * time.Second).UnixNano()}}
	n.Signature = testSignature(k, "remoteapp-network-state-v1\n", n.Claims)
	return invite.NetworkReply{State: &n}
}

func watchedFixture(t *testing.T) (*authorizationGate, invite.Grant, ed25519.PrivateKey, *enrollment, invite.NetworkBinding) {
	t.Helper()
	g, grant, key, clock := authorizationFixture(t)
	if _, err := g.request(context.Background(), clock, invite.NetworkRequestBody{Action: "grant_clock", Grant: grant}); err != nil {
		t.Fatal("clock")
	}
	n := &proofNodeFixture{enrollmentNodeFixture: nodeFixture(), ready: true}
	e := &enrollment{node: n, ticket: strings.Repeat("e", 32), key: n.key, address: n.address, attempted: true, proofCancel: func() {}}
	b := invite.NetworkBinding{TargetNode: n.key, ControllerNode: "nodekey:" + strings.Repeat("2", 64), TargetIP: n.address, ControllerIP: "100.120.0.2"}
	return g, grant, key, e, b
}

func TestEnrollmentWatchAllowsOnlyInitialActivationThenInactiveFaults(t *testing.T) {
	g, grant, key, e, binding := watchedFixture(t)
	var calls atomic.Int32
	client := requestFunc(func(ctx context.Context, b invite.NetworkRequestBody) (invite.NetworkReply, error) {
		if ctx.Err() != nil || (b.Action != "state" && b.Action != "await_binding") || b.Grant != grant || b.Ticket != "" || b.Challenge != "" {
			t.Fatal("unexpected state request")
		}
		n := calls.Add(1)
		if (n == 1 && b.Action != "await_binding") || (n > 1 && b.Action != "state") {
			t.Fatal("readiness must be initial-only; watch failures cannot wait again")
		}
		return watchState(key, grant, binding, n >= 2 && n < 4, g.clock()), nil
	})
	r, err := e.startWatch(context.Background(), g, client)
	if err != nil || !r.Passed || !r.Active || r.Status != "network_watch" || calls.Load() != 2 {
		t.Fatal("bounded inactive-to-active transition")
	}
	raw, _ := json.Marshal(r)
	for _, private := range []string{grant.Claims.ID, grant.Signature, e.ticket, e.key, e.address, "binding", "ticket", "credential"} {
		if strings.Contains(string(raw), private) {
			t.Fatal("private watch state escaped")
		}
	}
	select {
	case <-e.watch.Done():
		if !e.watch.Faulted() || calls.Load() != 4 {
			t.Fatal("inactive state did not latch terminal fault")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("watch did not terminate")
	}
	if e.closeJoined() != nil || e.node.(*proofNodeFixture).closes != 1 {
		t.Fatal("watch/proof/node not joined")
	}
}

func TestEnrollmentWatchBindingChangeAfterActivationFaults(t *testing.T) {
	g, grant, key, e, binding := watchedFixture(t)
	var calls atomic.Int32
	client := requestFunc(func(context.Context, invite.NetworkRequestBody) (invite.NetworkReply, error) {
		b := binding
		if calls.Add(1) > 1 {
			b.ControllerIP = "100.120.0.9"
		}
		return watchState(key, grant, b, true, g.clock()), nil
	})
	if r, err := e.startWatch(context.Background(), g, client); err != nil || !r.Passed {
		t.Fatal("activation")
	}
	select {
	case <-e.watch.Done():
		if !e.watch.Faulted() {
			t.Fatal("binding replacement not terminal")
		}
	case <-time.After(time.Second):
		t.Fatal("binding replacement remained active")
	}
	if e.closeJoined() != nil {
		t.Fatal("fixture close")
	}
}

func TestEnrollmentWatchExplicitStopJoinsWithoutFault(t *testing.T) {
	g, grant, key, e, binding := watchedFixture(t)
	client := requestFunc(func(context.Context, invite.NetworkRequestBody) (invite.NetworkReply, error) {
		return watchState(key, grant, binding, true, g.clock()), nil
	})
	if r, err := e.startWatch(context.Background(), g, client); err != nil || !r.Passed {
		t.Fatal("start")
	}
	if e.closeJoined() != nil || e.watch.Faulted() {
		t.Fatal("explicit stop treated as authorization fault")
	}
}

func TestEnrollmentWatchRejectsUntrustedStateBeforeWorker(t *testing.T) {
	for _, scenario := range []string{"binding", "signature", "mixed", "transport", "identity", "no_proof"} {
		t.Run(scenario, func(t *testing.T) {
			g, grant, key, e, binding := watchedFixture(t)
			if scenario == "no_proof" {
				e.proofCancel = nil
			}
			calls := 0
			client := requestFunc(func(context.Context, invite.NetworkRequestBody) (invite.NetworkReply, error) {
				calls++
				if scenario == "transport" {
					return invite.NetworkReply{}, errors.New("fixture")
				}
				r := watchState(key, grant, binding, true, g.clock())
				switch scenario {
				case "binding":
					r.State.Claims.Binding.TargetIP = "100.120.0.9"
					r.State.Signature = testSignature(key, "remoteapp-network-state-v1\n", r.State.Claims)
				case "signature":
					r.State.Signature = strings.Repeat("0", 128)
				case "mixed":
					r.Ticket = strings.Repeat("f", 32)
				case "identity":
					_, g.signer.(*testSigner).key, _ = ed25519.GenerateKey(rand.Reader)
				}
				return r, nil
			})
			if r, err := e.startWatch(context.Background(), g, client); err == nil || r.Passed || e.watch != nil {
				t.Fatal("untrusted state started worker")
			}
			if scenario == "no_proof" && calls != 0 {
				t.Fatal("missing proof queried authority")
			}
			if scenario != "no_proof" && calls != 1 {
				t.Fatal("untrusted readiness response must not be retried")
			}
			if e.closeJoined() != nil {
				t.Fatal("fixture close")
			}
		})
	}
}

func TestEnrollmentWatchInitialInactiveHasBoundedContext(t *testing.T) {
	g, grant, key, e, binding := watchedFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	client := requestFunc(func(context.Context, invite.NetworkRequestBody) (invite.NetworkReply, error) {
		return watchState(key, grant, binding, false, g.clock()), nil
	})
	start := time.Now()
	if r, err := e.startWatch(ctx, g, client); err == nil || r.Passed || e.watch != nil || time.Since(start) > time.Second {
		t.Fatal("inactive activation wait escaped caller deadline")
	}
	if e.closeJoined() != nil {
		t.Fatal("fixture close")
	}
}
