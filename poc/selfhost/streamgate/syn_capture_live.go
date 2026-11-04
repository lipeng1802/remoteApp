package streamgate

import (
	"context"
	"net/netip"
	"sync"
	"time"

	"tailscale.com/client/local"
)

// beginSYNTrace opens the SDK stream before dialing. The SDK can buffer the
// capture header until traffic arrives, so a missed frame is inconclusive.
func beginSYNTrace(ctx context.Context, lc *local.Client, controller, target netip.Addr) func() synEvidence {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	openDeadline := time.AfterFunc(500*time.Millisecond, cancel)
	stream, err := lc.StreamDebugCapture(ctx)
	openDeadline.Stop()
	if err != nil {
		cancel()
		return func() synEvidence { return synEvidence{} }
	}
	closeStream := sync.OnceFunc(func() { stream.Close() })
	stop := context.AfterFunc(ctx, closeStream)
	done := make(chan synEvidence, 1)
	go func() {
		done <- readSYNEvidence(stream, controller, target, false)
	}()
	return sync.OnceValue(func() synEvidence {
		cancel()
		stop()
		closeStream()
		return <-done
	})
}
