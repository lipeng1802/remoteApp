package installeddevice

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"remoteapp.local/selfhost-poc/cleanupwitness"
	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/startuptrace"
	"remoteapp.local/selfhost-poc/streamgate"
)

// enrollmentWatch is terminal after its first ACTIVE state. Before activation
// it may observe signed inactive snapshots while the peer finishes its own
// bind, but it opens no payload worker and has a fixed 12-second ceiling.
type enrollmentWatch struct {
	cancel context.CancelFunc
	done   chan struct{}
	once   sync.Once
	mu     sync.Mutex
	fault  atomic.Bool
	gate   *streamgate.Gate
}

func (w *enrollmentWatch) Done() <-chan struct{} { return w.done }
func (w *enrollmentWatch) Faulted() bool         { return w.fault.Load() }
func (w *enrollmentWatch) Stop() {
	w.once.Do(func() { w.cancel(); <-w.done })
}

// AttachGate serializes the initial signed state request with watch refreshes.
// Otherwise an older in-flight watch reply could arrive after a newer initial
// state and correctly trip the cache's anti-replay rejection.
func (w *enrollmentWatch) AttachGate(open func() (*streamgate.Gate, error)) (*streamgate.Gate, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	select {
	case <-w.done:
		return nil, invite.ErrDenied
	default:
	}
	if w.fault.Load() || w.gate != nil || open == nil {
		return nil, invite.ErrDenied
	}
	g, err := open()
	if err != nil {
		return nil, err
	}
	if g == nil {
		return nil, invite.ErrDenied
	}
	select {
	case <-w.done:
		g.Deny()
		return nil, invite.ErrDenied
	default:
	}
	w.gate = g
	return g, nil
}
func (w *enrollmentWatch) updateGate(ctx context.Context, query func() (invite.NetworkState, error)) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	state, err := query()
	// Stop cancels an in-flight refresh too. That cancellation is not an
	// authority failure and must not overwrite the original session outcome.
	if ctx.Err() != nil {
		return false
	}
	stage := startuptrace.LifecycleStateRejected
	var failure stateFailure
	if errors.As(err, &failure) {
		stage = startuptrace.Stage(failure)
	}
	if err == nil {
		stage = startuptrace.LifecycleInactive
		if state.Claims.Active {
			if w.gate == nil || w.gate.Update(state) {
				return true
			}
			stage = startuptrace.LifecycleGateRejected
		}
	}
	w.fault.Store(true)
	startuptrace.Emit(ctx, stage)
	return false
}
func (w *enrollmentWatch) denyGate() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.gate != nil {
		w.gate.Deny()
	}
}

func (e *enrollment) ownBinding(g *authorizationGate, b invite.NetworkBinding) bool {
	if !nodeKeyOK(b.TargetNode) || !nodeKeyOK(b.ControllerNode) || b.TargetNode == b.ControllerNode || !overlayOK(b.TargetIP) || !overlayOK(b.ControllerIP) || b.TargetIP == b.ControllerIP {
		return false
	}
	switch g.self {
	case g.grant.Claims.Target:
		return b.TargetNode == e.key && b.TargetIP == e.address
	case g.grant.Claims.Controller:
		return b.ControllerNode == e.key && b.ControllerIP == e.address
	default:
		return false
	}
}

func (e *enrollment) state(ctx context.Context, g *authorizationGate, client networkRequester, expected *invite.NetworkBinding) (invite.NetworkState, error) {
	return e.stateRequest(ctx, g, client, expected, false)
}

// These fixed TEST labels preserve the denial cause without retaining a reply,
// grant, endpoint or native provider error. All failures still wrap ErrDenied.
type stateFailure startuptrace.Stage

func (f stateFailure) Error() string { return string(f) }
func (f stateFailure) Unwrap() error { return invite.ErrDenied }

func (e *enrollment) stateRequest(ctx context.Context, g *authorizationGate, client networkRequester, expected *invite.NetworkBinding, initial bool) (invite.NetworkState, error) {
	deny := func(stage startuptrace.Stage) (invite.NetworkState, error) {
		return invite.NetworkState{}, stateFailure(stage)
	}
	if ctx.Err() != nil || g == nil || g.clock == nil || g.signer == nil || client == nil || e.node == nil || e.node.PublicKey() != e.key || !bytes.Equal(g.public, g.signer.PublicKey()) || !g.validGrant(g.grant, g.clock()) {
		return deny(startuptrace.LifecycleStateLocalRejected)
	}
	action, budget := "state", 5*time.Second
	if initial {
		action, budget = "await_binding", 12*time.Second
	}
	call, cancel := context.WithTimeout(ctx, budget)
	reply, err := client.Request(call, invite.NetworkRequestBody{Action: action, Grant: g.grant})
	failed := call.Err() != nil
	cancel()
	if err != nil || failed {
		return deny(startuptrace.LifecycleStateRequestRejected)
	}
	if reply.State == nil || reply.Ticket != "" || reply.Credential != nil || reply.Proof != nil || reply.TLS != nil || reply.Session != nil || reply.Verifier != nil || reply.State.Claims.Grant != g.grant {
		return deny(startuptrace.LifecycleStateRejected)
	}
	now := g.clock()
	if !invite.VerifyNetworkState(g.issuer, *reply.State, now) {
		// Classify time only after independently verifying the signed snapshot
		// at its own issuance. This never authorizes it at the current time.
		c := reply.State.Claims
		if invite.VerifyNetworkState(g.issuer, *reply.State, time.Unix(0, c.IssuedAt)) {
			if now.UnixNano() < c.IssuedAt {
				return deny(startuptrace.LifecycleStateFuture)
			}
			if now.UnixNano() >= c.ExpiresAt {
				return deny(startuptrace.LifecycleStateExpired)
			}
		}
		return deny(startuptrace.LifecycleStateRejected)
	}
	if !e.ownBinding(g, reply.State.Claims.Binding) || !g.validGrant(g.grant, g.clock()) || !bytes.Equal(g.public, g.signer.PublicKey()) || e.node.PublicKey() != e.key {
		return deny(startuptrace.LifecycleStateLocalRejected)
	}
	if expected != nil && reply.State.Claims.Binding != *expected {
		return deny(startuptrace.LifecycleStateBindingRejected)
	}
	return *reply.State, nil
}

func (e *enrollment) startWatch(ctx context.Context, g *authorizationGate, client networkRequester) (enrollmentResult, error) {
	result := enrollmentResult{Status: "network_watch"}
	deny := func() (enrollmentResult, error) { return result, invite.ErrDenied }
	if e.watch != nil || e.proofCancel == nil || !canonical(e.ticket, 16) || g == nil || g.clock == nil {
		return deny()
	}
	budget := time.Until(time.Unix(g.grant.Claims.ExpiresAt, 0))
	if g.clock != nil {
		budget = time.Unix(g.grant.Claims.ExpiresAt, 0).Sub(g.clock())
	}
	if budget > 12*time.Second {
		budget = 12 * time.Second
	}
	if budget <= 0 {
		return deny()
	}
	activation, stopActivation := context.WithTimeout(ctx, budget)
	defer stopActivation()
	var binding invite.NetworkBinding
	initial := true
	for {
		state, err := e.stateRequest(activation, g, client, nil, initial)
		if err != nil {
			return deny()
		}
		initial = false // never repeat the readiness wait after a verified pair
		if state.Claims.Active {
			binding = state.Claims.Binding
			break
		}
		timer := time.NewTimer(250 * time.Millisecond)
		select {
		case <-activation.Done():
			timer.Stop()
			return deny()
		case <-timer.C:
		}
	}
	if ctx.Err() != nil || !g.validGrant(g.grant, g.clock()) || !bytes.Equal(g.public, g.signer.PublicKey()) || e.node.PublicKey() != e.key {
		return deny()
	}
	e.binding = binding
	life, cancel := context.WithCancel(ctx)
	w := &enrollmentWatch{cancel: cancel, done: make(chan struct{})}
	e.watch = w
	e.witness.Begin(cleanupwitness.Watch)
	go func() {
		defer close(w.done)
		defer w.denyGate()
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-life.Done():
				return
			case <-ticker.C:
				if !w.updateGate(life, func() (invite.NetworkState, error) {
					return e.state(life, g, client, &binding)
				}) {
					return
				}
			}
		}
	}()
	result.Passed = true
	result.Active = true
	return result, nil
}
