package streamgate

import (
	"context"
	"errors"
	"net"
	"remoteapp.local/selfhost-poc/nodeproof"
	"remoteapp.local/selfhost-poc/session"
	"time"
)

// All attempts share DialOverlay's original three-second context and fixed
// signed destination. Only an explicit TCP refusal can await listener startup.
// This performs no enrollment/approval, network fallback or identity bypass.
func dialUntilListening(ctx context.Context, dial func(context.Context) (net.Conn, error), active func(context.Context) bool) (net.Conn, error) {
	for {
		if ctx.Err() != nil || !active(ctx) {
			return nil, session.ErrDenied
		}
		conn, e := dial(ctx)
		if e == nil && conn != nil && ctx.Err() == nil && active(ctx) {
			return conn, nil
		}
		if conn != nil {
			conn.Close()
		}
		if e == nil || !errors.Is(e, nodeproof.ErrOverlayNotListening) {
			return nil, session.ErrDenied
		}
		timer := time.NewTimer(50 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, session.ErrDenied
		case <-timer.C:
		}
	}
}
