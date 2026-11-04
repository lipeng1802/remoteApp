package streamgate

import (
	"context"
	"net/netip"
	"sync"
	"time"

	"remoteapp.local/selfhost-poc/startuptrace"
	"tailscale.com/client/local"
)

// beginSYNTrace opens the SDK stream before dialing. The SDK can buffer the
// capture header until traffic arrives, so a missed frame is inconclusive.
func beginSYNTrace(ctx context.Context, lc *local.Client, controller, target netip.Addr, atTarget bool) func() synEvidence {
	lifetime := 4 * time.Second
	if atTarget {
		lifetime = 20 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, lifetime)
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
		done <- readSYNEvidence(stream, controller, target, atTarget, func(e synEvidence) {
			if atTarget {
				stage := startuptrace.TargetSYNObserved
				if e.reply {
					stage = startuptrace.TargetSYNReplyObserved
				}
				startuptrace.Emit(ctx, stage)
			}
		})
	}()
	return sync.OnceValue(func() synEvidence {
		cancel()
		stop()
		closeStream()
		return <-done
	})
}
