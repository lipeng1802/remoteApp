package processowner

import (
	"context"
	"remoteapp.local/selfhost-poc/jointrace"
)

// Own runs an in-process network worker with one product child. Called INSIDE
// clientowner.Runner: neither normal cancellation nor a timeout can bypass
// product Stop's actual join before the runner returns. Child is always owned,
// including startup failures handled by the caller before calling Own.
func Own(ctx context.Context, c *Child, work func(context.Context) error) error {
	if ctx == nil || c == nil || work == nil {
		return ErrProcess
	}
	life, cancel := context.WithCancel(ctx)
	defer cancel()
	worker := make(chan error, 1)
	go func() { worker <- work(life) }()
	var result error
	ended := false
	select {
	case result = <-worker:
		ended = true
	case <-ctx.Done():
	case <-c.Exited():
		result = ErrProcess
	case <-c.readDone:
		result = ErrProcess
	}
	cancel()
	// Stop concurrently with network cancellation, never let a stuck worker
	// leave a product serving. No detached timeout or forced kill as success.
	joined := make(chan error, 1)
	go func() {
		jointrace.Emit(jointrace.ProductStopping)
		e := c.Stop(context.Background())
		if e == nil {
			jointrace.Emit(jointrace.ProductJoined)
		}
		joined <- e
	}()
	if !ended {
		if e := <-worker; result == nil {
			result = e
		}
	}
	if e := <-joined; e != nil {
		return ErrProcess
	}
	return result
}
