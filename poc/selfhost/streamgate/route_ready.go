package streamgate

import (
	"context"
	"remoteapp.local/selfhost-poc/nodeproof"
	"time"
)

// Wait only for initial authenticated netmap visibility, before any dial. This
// never retries a failed dial, changes the signed destination, or resets the
// original connection budget. Unknown queries and conflicting identity fail.
func waitInitialRoute(ctx context.Context, query func(context.Context) nodeproof.RouteStatus, active func(context.Context) bool) bool {
	for {
		if ctx.Err() != nil || !active(ctx) {
			return false
		}
		state := query(ctx)
		if ctx.Err() != nil || !active(ctx) || !state.QueryOK || !state.Running {
			return false
		}
		if state.PeerKnown && !state.IPMatches {
			return false
		}
		if state.Ready {
			return true
		}
		timer := time.NewTimer(50 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return false
		case <-timer.C:
		}
	}
}
