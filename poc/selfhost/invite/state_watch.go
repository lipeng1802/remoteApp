package invite

import (
	"context"
	"crypto/ed25519"
	"sync"
	"time"
)

// Device-owned serial state polling. It never retries enrollment/Mint, never
// refreshes a lease from receipt time and cannot recover a revoked watch.
// Consumers MUST retain their absolute lease watchdog; cached state is not
// permission to continue after the watch stops or a pipe closes.
type StateWatch struct {
	mu      sync.Mutex
	latest  *NetworkState
	failed  bool
	cancel  context.CancelFunc
	done    chan struct{}
	issuer  ed25519.PublicKey
	grant   Grant
	binding NetworkBinding
	clock   func() time.Time
}

func (c *NetworkClient) StartStateWatch(ctx context.Context, issuer ed25519.PublicKey, g Grant, binding NetworkBinding, clock func() time.Time) (*StateWatch, error) {
	if ctx.Err() != nil || len(issuer) != 32 || clock == nil || !validBinding(binding) || !VerifyGrant(issuer, g, g.Claims.Target, g.Claims.Controller, clock()) {
		return nil, ErrDenied
	}
	ctx, cancel := context.WithCancel(ctx)
	w := &StateWatch{cancel: cancel, done: make(chan struct{}), issuer: append(ed25519.PublicKey(nil), issuer...), grant: g, binding: binding, clock: clock}
	go func() {
		defer close(w.done)
		for {
			r, e := c.Request(ctx, NetworkRequestBody{Action: "state", Grant: g})
			if e != nil || r.State == nil || r.State.Claims.Grant != g || r.State.Claims.Binding != binding || !VerifyGrant(w.issuer, g, g.Claims.Target, g.Claims.Controller, clock()) || !VerifyNetworkState(w.issuer, *r.State, clock()) {
				w.mu.Lock()
				w.failed = true
				w.latest = nil
				w.mu.Unlock()
				return
			}
			w.mu.Lock()
			n := *r.State
			w.latest = &n
			w.mu.Unlock()
			if !n.Claims.Active {
				return
			}
			timer := time.NewTimer(500 * time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
				w.mu.Lock()
				w.failed = true
				w.latest = nil
				w.mu.Unlock()
				return
			case <-timer.C:
			}
		}
	}()
	return w, nil
}
func (w *StateWatch) Snapshot() (NetworkState, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.failed || w.latest == nil || !VerifyGrant(w.issuer, w.grant, w.grant.Claims.Target, w.grant.Claims.Controller, w.clock()) || !VerifyNetworkState(w.issuer, *w.latest, w.clock()) || w.latest.Claims.Grant != w.grant || w.latest.Claims.Binding != w.binding {
		return NetworkState{}, ErrDenied
	}
	return *w.latest, nil
}
func (w *StateWatch) Stop() {
	w.cancel()
	<-w.done
	w.mu.Lock()
	w.failed = true
	w.latest = nil
	w.mu.Unlock()
}

// Done closes on cancellation, query failure or an authenticated inactive state.
func (w *StateWatch) Done() <-chan struct{} { return w.done }
