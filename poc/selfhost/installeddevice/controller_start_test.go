package installeddevice

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"sync/atomic"
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/invite"
)

func TestCoordinatedControllerPreparesFreshIdentityOnlyOnStart(t *testing.T) {
	g, grant, issuer, e, binding := watchedFixture(t)
	_, target, _ := ed25519.GenerateKey(rand.Reader)
	targetID := sha256.Sum256(target.Public().(ed25519.PublicKey))
	grant.Claims.Target = hex.EncodeToString(targetID[:])
	grant.Claims.Controller = g.self
	grant.Signature = testSignature(issuer, "remoteapp-invite-grant-v1\n", grant.Claims)
	g.grant = grant
	binding.TargetNode, binding.ControllerNode = binding.ControllerNode, binding.TargetNode
	binding.TargetIP, binding.ControllerIP = binding.ControllerIP, binding.TargetIP
	originalClock := g.clock
	var offset atomic.Int64
	g.clock = func() time.Time { return originalClock().Add(time.Duration(offset.Load())) }
	var identities, states atomic.Int32
	client := requestFunc(func(_ context.Context, b invite.NetworkRequestBody) (invite.NetworkReply, error) {
		switch b.Action {
		case "state", "await_binding":
			states.Add(1)
			return watchState(issuer, grant, binding, true, g.clock()), nil
		case "session_identity":
			identities.Add(1)
			if offset.Load() < int64(4*time.Second) {
				t.Error("identity prepared before explicit start")
			}
			now := g.clock()
			claims := invite.SessionIdentityClaims{Version: 1, Grant: grant, Binding: binding, TargetPublic: hex.EncodeToString(target.Public().(ed25519.PublicKey)), ControllerPublic: hex.EncodeToString(g.public), Challenge: b.Challenge, IssuedAt: now.UnixNano(), ExpiresAt: now.Add(3 * time.Second).UnixNano()}
			identity := invite.SessionIdentity{Claims: claims, Signature: testSignature(issuer, "remoteapp-session-identity-v1\n", claims)}
			return invite.NetworkReply{Session: &identity}, nil
		default:
			return invite.NetworkReply{}, invite.ErrDenied
		}
	})
	if _, err := e.startWatch(context.Background(), g, client); err != nil {
		t.Fatal(err)
	}
	defer e.closeJoined()
	start := controllerStart{pending: true}
	var works atomic.Int32
	work := func(ctx context.Context, r *SessionRuntime) error {
		works.Add(1)
		if r.peers.Server() {
			t.Error("controller became target")
		}
		<-ctx.Done()
		return ctx.Err()
	}
	if !start.allowed(e, g, work) || identities.Load() != 0 || e.peers != nil || e.watch.gate != nil {
		t.Fatal("pending acquired session material")
	}
	// More than the entire identity lifetime elapses before the trigger; the
	// identity and gate must both be obtained from the new time, with no cache.
	offset.Store(int64(4 * time.Second))
	if err := start.run(context.Background(), e, g, client, work); err != nil {
		t.Fatal(err)
	}
	if identities.Load() != 1 || states.Load() < 2 || e.peers == nil || e.watch.gate == nil {
		t.Fatal("fresh identity/state absent")
	}
	if err := start.run(context.Background(), e, g, client, work); err == nil || identities.Load() != 1 {
		t.Fatal("duplicate triggered another identity")
	}
	e.session.Stop()
	if works.Load() != 1 {
		t.Fatal("work not joined once")
	}
}

func TestControllerStartRejectsRolePhaseAndFailedIdentityIsOneShot(t *testing.T) {
	g, _, _, e, _ := watchedFixture(t)
	work := func(context.Context, *SessionRuntime) error { return nil }
	for _, pending := range []bool{false, true} {
		s := controllerStart{pending: pending}
		if s.allowed(e, g, work) {
			t.Fatal("preprovision/target accepted")
		}
	}
	// Closed watch and missing session wiring cannot admit an identity request.
	s := controllerStart{pending: true}
	g.grant.Claims.Controller = g.self
	e.watch = &enrollmentWatch{done: make(chan struct{})}
	close(e.watch.done)
	if err := s.run(context.Background(), e, g, requestFunc(func(context.Context, invite.NetworkRequestBody) (invite.NetworkReply, error) {
		t.Fatal("identity requested after watch ended")
		return invite.NetworkReply{}, nil
	}), work); err == nil {
		t.Fatal("closed watch accepted")
	}
	if !s.attempted {
		t.Fatal("failed admission rearmed")
	}
	for _, r := range []Request{{Op: "start_controller", VaultScope: "foreign"}, {Op: "start_controller", ExpectedPublic: "foreign"}, {Op: "start_controller", Device: invite.Body{Action: "status"}}} {
		if ValidControllerStart(r) {
			t.Fatal("selector admitted")
		}
	}
	if !ValidControllerStart(Request{Op: "start_controller"}) || ValidSupervisedCommand(Request{Op: "start_controller"}) {
		t.Fatal("private command became business")
	}
}
