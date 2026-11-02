package streamgate

import (
	"context"
	"remoteapp.local/selfhost-poc/nodeproof"
	"testing"
	"time"
)

func TestInitialRouteWaitSharesOriginalDeadlineAndRequiresLiveAuthority(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	end, _ := ctx.Deadline()
	calls := 0
	ready := nodeproof.RouteStatus{QueryOK: true, Running: true, PeerKnown: true, IPMatches: true, Ready: true}
	query := func(c context.Context) nodeproof.RouteStatus {
		calls++
		d, _ := c.Deadline()
		if d != end {
			t.Fatal("budget replaced")
		}
		if calls == 1 {
			return nodeproof.RouteStatus{QueryOK: true, Running: true}
		}
		return ready
	}
	if !waitInitialRoute(ctx, query, func(context.Context) bool { return true }) || calls != 2 {
		t.Fatal("initial map visibility not awaited")
	}
	// InEngine=false still permits an idle peer; identity/route Ready is required.
	if ready.InEngine {
		t.Fatal("invalid fixture")
	}
	calls = 0
	if waitInitialRoute(ctx, query, func(context.Context) bool { return calls == 0 }) {
		t.Fatal("revocation during query ignored")
	}
}
func TestInitialRouteWaitRejectsUnknownConflictCancellationAndTimeout(t *testing.T) {
	for _, state := range []nodeproof.RouteStatus{{}, {QueryOK: true}, {QueryOK: true, Running: true, PeerKnown: true, IPMatches: false}} {
		calls := 0
		if waitInitialRoute(context.Background(), func(context.Context) nodeproof.RouteStatus { calls++; return state }, func(context.Context) bool { return true }) || calls != 1 {
			t.Fatal("unknown state retried")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if waitInitialRoute(ctx, func(context.Context) nodeproof.RouteStatus {
		return nodeproof.RouteStatus{QueryOK: true, Running: true}
	}, func(context.Context) bool { return true }) {
		t.Fatal("deadline ignored")
	}
	ctx2, stop := context.WithCancel(context.Background())
	stop()
	calls := 0
	if waitInitialRoute(ctx2, func(context.Context) nodeproof.RouteStatus { calls++; return nodeproof.RouteStatus{} }, func(context.Context) bool { return true }) || calls != 0 {
		t.Fatal("query after cancellation")
	}
}
