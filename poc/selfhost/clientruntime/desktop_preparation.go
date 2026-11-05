package clientruntime

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"slices"
	"sync"
	"time"

	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/processowner"
	"remoteapp.local/selfhost-poc/session"
)

// PreparedDesktop owns one hash-pinned GUI child before any TLS startup. Its
// private pipe is not a network/argv capability. Local consent is not a grant;
// ObserveApproved and the normal live owner/gate are still mandatory.
type PreparedDesktop struct {
	mu                                                  sync.Mutex
	child                                               *processowner.Child
	spec                                                processowner.Spec
	policy                                              invite.GrantPolicy
	self                                                string
	server                                              bool
	request, controller                                 string
	decisionStarted, allowed, consumed, closing, broken bool
	connectStarted                                      bool
	approved                                            invite.Grant
	issuer                                              ed25519.PublicKey
	opCancel                                            context.CancelFunc
	opDone                                              chan struct{}
}

func (*PreparedDesktop) String() string   { return "PreparedDesktop(private)" }
func (*PreparedDesktop) GoString() string { return "PreparedDesktop(private)" }

// PrepareDesktop starts only a GUI preparation phase. The child must not open
// TLS, capture or input before subsequent consent and activation. Failure may
// return a partial owner which the caller MUST Stop/join, including on timeout.
func PrepareDesktop(ctx context.Context, expected invite.DesktopPermission, self string, server bool, spec processowner.Spec) (*PreparedDesktop, error) {
	policy, e := invite.DesktopGrantPolicy(expected)
	if e != nil || ctx == nil || ctx.Err() != nil {
		return nil, session.ErrDenied
	}
	if _, e = key(self); e != nil {
		return nil, session.ErrDenied
	}
	spec.Args = append([]string(nil), spec.Args...)
	child, e := startObservedTLS(ctx, spec)
	if e != nil {
		return nil, e
	}
	p := &PreparedDesktop{child: child, spec: spec, policy: policy, self: self, server: server}
	ready, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	role := "controller"
	if server {
		role = "target"
	}
	raw, e := child.Exchange(ready, map[string]string{"Op": "prepare", "Permission": policy.Permission(), "Role": role, "Self": self})
	var ack struct {
		Status           string `json:"status"`
		Permission, Role string
	}
	if e != nil || decodeProductReady(raw, &ack) != nil || ack.Status != "desktop_prepared" || ack.Permission != policy.Permission() || ack.Role != role {
		p.broken = true
		return p, session.ErrDenied
	}
	return p, nil
}

// Decide asks the same GUI to obtain explicit local consent for one verified
// request. The trusted caller supplies the HTTPS-observed IDs, never peer-selected
// identity/trust fields. Waiting precedes the original 8s TLS startup budget;
// the authority's original request/grant expiry remains independently enforced.
// Every attempt, including refusal/malformed/unknown reply, is single-use.
func (p *PreparedDesktop) Decide(ctx context.Context, request, controller string) (bool, error) {
	if p == nil || ctx == nil || ctx.Err() != nil {
		return false, session.ErrDenied
	}
	r, e := hex.DecodeString(request)
	if e != nil || len(r) != 16 || hex.EncodeToString(r) != request {
		return false, session.ErrDenied
	}
	if _, e = key(controller); e != nil {
		return false, session.ErrDenied
	}
	p.mu.Lock()
	if p.child == nil || p.broken || p.closing || p.decisionStarted || p.consumed || p.opCancel != nil || (!p.server && controller != p.self) || (p.server && controller == p.self) {
		p.mu.Unlock()
		return false, session.ErrDenied
	}
	life, cancel := context.WithTimeout(ctx, 120*time.Second)
	p.decisionStarted = true
	p.request, p.controller = request, controller
	p.opCancel, p.opDone = cancel, make(chan struct{})
	done := p.opDone
	p.mu.Unlock()
	defer func() { cancel(); p.mu.Lock(); p.opCancel = nil; close(done); p.mu.Unlock() }()
	raw, e := p.child.Exchange(life, map[string]string{"Op": "consent", "RequestID": request, "Controller": controller})
	var decision struct {
		Status                string `json:"status"`
		RequestID, Permission string
		Allow                 *bool
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if e != nil || life.Err() != nil || p.closing || decodeProductReady(raw, &decision) != nil || decision.Status != "desktop_decision" || decision.RequestID != request || decision.Permission != p.policy.Permission() || decision.Allow == nil {
		p.broken = true
		return false, session.ErrDenied
	}
	p.allowed = *decision.Allow
	return p.allowed, nil
}

// ObserveApproved is called only after the trusted owner's successful matching
// approve/status reply (which correlates RequestID to the signed grant). Grants
// do not themselves sign RequestID. Signature checking here is historical only;
// activation still verifies current time and every original live network gate.
func (p *PreparedDesktop) ObserveApproved(issuer ed25519.PublicKey, request string, g invite.Grant) error {
	if p == nil {
		return session.ErrDenied
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.child == nil || p.broken || p.closing || p.consumed || !p.allowed || p.opCancel != nil || p.approved != (invite.Grant{}) || request != p.request || g.Claims.Controller != p.controller ||
		(p.server && g.Claims.Target != p.self) || (!p.server && g.Claims.Controller != p.self) ||
		!p.policy.Verify(issuer, g, g.Claims.Target, g.Claims.Controller, time.Unix(g.Claims.IssuedAt, 0)) {
		return session.ErrDenied
	}
	p.approved = g
	p.issuer = append(ed25519.PublicKey(nil), issuer...)
	return nil
}

func (p *PreparedDesktop) matchesLocked(policy invite.GrantPolicy, spec processowner.Spec, issuer ed25519.PublicKey, g invite.Grant, server bool) bool {
	if p.child == nil || p.broken || p.closing || p.consumed || p.opCancel != nil || !p.allowed || p.approved != g || p.policy != policy || p.server != server || !bytes.Equal(p.issuer, issuer) ||
		p.spec.Executable != spec.Executable || p.spec.SHA256 != spec.SHA256 || p.spec.CaptureProductStage != spec.CaptureProductStage || !slices.Equal(p.spec.Args, spec.Args) {
		return false
	}
	select {
	case <-p.child.Exited():
		return false
	default:
		return true
	}
}

// take transfers no process ownership: both the session worker and preparation
// owner retain this SAME child for joined cleanup. It never starts a substitute.
func (p *PreparedDesktop) take(ctx context.Context, policy invite.GrantPolicy, spec processowner.Spec, issuer ed25519.PublicKey, g invite.Grant, server bool) (context.Context, *processowner.Child, func(), error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.matchesLocked(policy, spec, issuer, g, server) {
		return nil, nil, nil, session.ErrDenied
	}
	life, cancel := context.WithCancel(ctx)
	p.consumed = true
	p.opCancel, p.opDone = cancel, make(chan struct{})
	done := p.opDone
	var once sync.Once
	finish := func() { once.Do(func() { cancel(); p.mu.Lock(); p.opCancel = nil; close(done); p.mu.Unlock() }) }
	return life, p.child, finish, nil
}

// Stop cancels an outstanding decision/activation, then waits for this child,
// EOF/reader and the private operation to join. Timeout retains ownership; a
// later Stop only waits the same join. It never retries consent or force kills.
func (p *PreparedDesktop) Stop(ctx context.Context) error {
	if p == nil || ctx == nil {
		return session.ErrDenied
	}
	p.mu.Lock()
	p.closing = true
	if p.opCancel != nil {
		p.opCancel()
	}
	child, done := p.child, p.opDone
	p.mu.Unlock()
	if child == nil {
		return session.ErrDenied
	}
	err := child.Stop(ctx)
	if done != nil {
		select {
		case <-done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return err
}

func (p *PreparedDesktop) PID() int {
	if p == nil || p.child == nil {
		return 0
	}
	return p.child.PID()
}
