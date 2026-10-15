package installeddevice

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

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
	return e, e.node.(*proofNodeFixture)
}

func TestSessionRequiresLiveActiveWatch(t *testing.T) {
	e := &enrollment{}
	called := false
	if err := e.startSession(context.Background(), func(context.Context) error { called = true; return nil }); err == nil || called || e.session != nil {
		t.Fatal("session started without active watch")
	}

	e, _ = activeEnrollmentForSession(t, 2)
	select {
	case <-e.watch.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("watch did not fault")
	}
	if err := e.startSession(context.Background(), func(context.Context) error { called = true; return nil }); err == nil || called || e.session != nil {
		t.Fatal("session started after watch fault")
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
	if err := e.startSession(context.Background(), func(ctx context.Context) error {
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
	if err := e.startSession(context.Background(), func(context.Context) error { return want }); err != nil {
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
	if err := e.startSession(context.Background(), func(ctx context.Context) error {
		<-ctx.Done()
		return want
	}); err != nil {
		t.Fatal(err)
	}
	if err := e.closeJoined(); !errors.Is(err, want) || node.closes != 1 {
		t.Fatal("session failure hidden or node not joined", err, node.closes)
	}
}
