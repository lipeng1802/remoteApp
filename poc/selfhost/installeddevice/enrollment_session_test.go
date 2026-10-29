package installeddevice

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/clientruntime"
	"remoteapp.local/selfhost-poc/invite"
)

func activeEnrollmentForSession(t *testing.T, inactiveAfter int32) (*enrollment, *proofNodeFixture) {
	t.Helper()
	g, grant, key, e, binding := watchedFixture(t)
	var calls atomic.Int32
	client := requestFunc(func(context.Context, invite.NetworkRequestBody) (invite.NetworkReply, error) {
		active := inactiveAfter == 0 || calls.Add(1) < inactiveAfter
		return watchState(key, grant, binding, active, g.clock()), nil
	})
	if r, err := e.startWatch(context.Background(), g, client); err != nil || !r.Passed || !r.Active {
		t.Fatal("watch activation")
	}
	e.peers = &SessionPeers{grant: grant, binding: binding, server: true}
	e.peers.runtime = &SessionRuntime{peers: e.peers}
	return e, e.node.(*proofNodeFixture)
}

func TestSessionRequiresLiveActiveWatch(t *testing.T) {
	e := &enrollment{}
	called := false
	if err := e.startSession(context.Background(), func(context.Context, *SessionRuntime) error { called = true; return nil }); err == nil || called || e.session != nil {
		t.Fatal("session started without active watch")
	}

	e, _ = activeEnrollmentForSession(t, 2)
	select {
	case <-e.watch.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("watch did not fault")
	}
	if err := e.startSession(context.Background(), func(context.Context, *SessionRuntime) error { called = true; return nil }); err == nil || called || e.session != nil {
		t.Fatal("session started after watch fault")
	}
	if err := e.closeJoined(); err != nil {
		t.Fatal(err)
	}
}

func TestSessionPeersComeOnlyFromIssuerSignedActiveBinding(t *testing.T) {
	g, grant, issuer, e, binding := watchedFixture(t)
	_, controller, _ := ed25519.GenerateKey(rand.Reader)
	controllerID := sha256.Sum256(controller.Public().(ed25519.PublicKey))
	grant.Claims.Controller = hex.EncodeToString(controllerID[:])
	grant.Signature = testSignature(issuer, "remoteapp-invite-grant-v1\n", grant.Claims)
	g.grant = grant
	client := requestFunc(func(_ context.Context, b invite.NetworkRequestBody) (invite.NetworkReply, error) {
		switch b.Action {
		case "state", "await_binding":
			return watchState(issuer, grant, binding, true, g.clock()), nil
		case "session_identity":
			now := g.clock()
			claims := invite.SessionIdentityClaims{Version: 1, Grant: grant, Binding: binding, TargetPublic: hex.EncodeToString(g.public), ControllerPublic: hex.EncodeToString(controller.Public().(ed25519.PublicKey)), Challenge: b.Challenge, IssuedAt: now.UnixNano(), ExpiresAt: now.Add(3 * time.Second).UnixNano()}
			identity := invite.SessionIdentity{Claims: claims, Signature: testSignature(issuer, "remoteapp-session-identity-v1\n", claims)}
			return invite.NetworkReply{Session: &identity}, nil
		default:
			return invite.NetworkReply{}, invite.ErrDenied
		}
	})
	if r, err := e.startWatch(context.Background(), g, client); err != nil || !r.Active {
		t.Fatal("watch")
	}
	if err := e.prepareSessionPeers(context.Background(), g, client); err != nil || e.peers == nil || !e.peers.Server() || e.peers.Binding() != binding || e.peers.Grant() != grant {
		t.Fatal("verified session peers", err)
	}
	target := e.peers.TargetPublic()
	target[0] ^= 0xff
	if hex.EncodeToString(e.peers.TargetPublic()) != hex.EncodeToString(g.public) || hex.EncodeToString(e.peers.ControllerPublic()) != hex.EncodeToString(controller.Public().(ed25519.PublicKey)) {
		t.Fatal("session peer pins are mutable")
	}
	if err := e.closeJoined(); err != nil {
		t.Fatal(err)
	}
}

func TestWatchFaultCancelsAndJoinsSessionBeforeNode(t *testing.T) {
	e, node := activeEnrollmentForSession(t, 2)
	started := make(chan struct{})
	cancelled := make(chan struct{})
	release := make(chan struct{})
	joined := make(chan struct{})
	if err := e.startSession(context.Background(), func(ctx context.Context, _ *SessionRuntime) error {
		close(started)
		<-ctx.Done()
		close(cancelled)
		<-release
		close(joined)
		return ctx.Err()
	}); err != nil {
		t.Fatal(err)
	}
	<-started
	select {
	case <-e.watch.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("watch did not fault")
	}
	closed := make(chan error, 1)
	go func() { closed <- e.closeJoined() }()
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("session was not cancelled")
	}
	if node.closes != 0 {
		t.Fatal("node closed before session joined")
	}
	close(release)
	select {
	case <-joined:
	case <-time.After(time.Second):
		t.Fatal("session did not join")
	}
	if err := <-closed; err != nil || node.closes != 1 {
		t.Fatal("ordered close", err, node.closes)
	}
}

func TestSessionEarlyExitIsObservableAndStillJoined(t *testing.T) {
	e, node := activeEnrollmentForSession(t, 0)
	want := errors.New("product fixture ended")
	if err := e.startSession(context.Background(), func(context.Context, *SessionRuntime) error { return want }); err != nil {
		t.Fatal(err)
	}
	select {
	case <-e.session.Done():
		if !errors.Is(e.session.Err(), want) {
			t.Fatal("session result")
		}
	case <-time.After(time.Second):
		t.Fatal("session exit not observable")
	}
	if err := e.closeJoined(); !errors.Is(err, want) || node.closes != 1 {
		t.Fatal("early session not joined", err)
	}
}

func TestSessionCloseFailureCannotHideBehindCancellation(t *testing.T) {
	e, node := activeEnrollmentForSession(t, 0)
	want := errors.New("product join failed")
	if err := e.startSession(context.Background(), func(ctx context.Context, _ *SessionRuntime) error {
		<-ctx.Done()
		return want
	}); err != nil {
		t.Fatal(err)
	}
	if err := e.closeJoined(); !errors.Is(err, want) || node.closes != 1 {
		t.Fatal("session failure hidden or node not joined", err, node.closes)
	}
}

func TestSessionRuntimeIsOpaqueAndFailsClosedWithoutNativeNode(t *testing.T) {
	e, _ := activeEnrollmentForSession(t, 0)
	runtime := e.peers.runtime
	if runtime == nil || runtime.Peers().runtime != nil || runtime.Peers().Binding() != e.binding {
		t.Fatal("opaque public session view")
	}
	if listener, err := runtime.ListenOverlay(context.Background()); err == nil || listener != nil {
		t.Fatal("listener opened without native node")
	}
	if conn, err := runtime.DialOverlay(context.Background()); err == nil || conn != nil {
		t.Fatal("dial opened without native node")
	}
	if err := e.closeJoined(); err != nil {
		t.Fatal(err)
	}
}

func TestProductSessionRejectsParentSecretCallbacks(t *testing.T) {
	e, _ := activeEnrollmentForSession(t, 0)
	runtime := e.peers.runtime
	if err := runtime.RunProduct(context.Background(), clientruntime.Product{ControllerKey: "parent-secret"}); err == nil {
		t.Fatal("parent supplied product secret accepted")
	}
	if err := runtime.RunProduct(context.Background(), clientruntime.Product{TargetReady: func(string, string) {}}); err == nil {
		t.Fatal("parent secret callback accepted")
	}
	if err := e.closeJoined(); err != nil {
		t.Fatal(err)
	}
}
