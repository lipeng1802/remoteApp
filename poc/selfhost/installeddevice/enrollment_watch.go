package installeddevice

import (
	"bytes"
	"context"
	"sync"
	"sync/atomic"
	"time"

	"remoteapp.local/selfhost-poc/invite"
)

// enrollmentWatch is terminal after its first ACTIVE state. Before activation
// it may observe signed inactive snapshots while the peer finishes its own
// bind, but it opens no payload worker and has a fixed 12-second ceiling.
type enrollmentWatch struct {
	cancel context.CancelFunc
	done   chan struct{}
	once   sync.Once
	fault  atomic.Bool
}

func (w *enrollmentWatch) Done() <-chan struct{} { return w.done }
func (w *enrollmentWatch) Faulted() bool         { return w.fault.Load() }
func (w *enrollmentWatch) Stop() {
	w.once.Do(func() { w.cancel(); <-w.done })
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
	deny := func() (invite.NetworkState, error) { return invite.NetworkState{}, invite.ErrDenied }
	if ctx.Err() != nil || g == nil || g.clock == nil || g.signer == nil || client == nil || e.node == nil || e.node.PublicKey() != e.key || !bytes.Equal(g.public, g.signer.PublicKey()) || !g.validGrant(g.grant, g.clock()) {
		return deny()
	}
	call, cancel := context.WithTimeout(ctx, 5*time.Second)
	reply, err := client.Request(call, invite.NetworkRequestBody{Action: "state", Grant: g.grant})
	failed := call.Err() != nil
	cancel()
	if err != nil || failed || reply.State == nil || reply.Ticket != "" || reply.Credential != nil || reply.Proof != nil || reply.TLS != nil || reply.Session != nil || reply.Verifier != nil || reply.State.Claims.Grant != g.grant || !invite.VerifyNetworkState(g.issuer, *reply.State, g.clock()) || !e.ownBinding(g, reply.State.Claims.Binding) || !g.validGrant(g.grant, g.clock()) || !bytes.Equal(g.public, g.signer.PublicKey()) || e.node.PublicKey() != e.key {
		return deny()
	}
	if expected != nil && reply.State.Claims.Binding != *expected {
		return deny()
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
	for {
		state, err := e.state(activation, g, client, nil)
		if err != nil {
			return deny()
		}
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
	go func() {
		defer close(w.done)
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-life.Done():
				return
			case <-ticker.C:
				state, err := e.state(life, g, client, &binding)
				if err != nil || !state.Claims.Active {
					w.fault.Store(true)
					return
				}
			}
		}
	}()
	result.Passed = true
	result.Active = true
	return result, nil
}
