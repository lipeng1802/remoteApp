// Package clientowner serializes an installed configuration with its live
// session. This is a trusted in-process API, not a peer-controlled dispatcher.
package clientowner

import (
	"context"
	"sync"
	"time"

	"remoteapp.local/selfhost-poc/trustinstall"
	"remoteapp.local/selfhost-poc/trustprofile"
)

// Runner must return only after every child watch, connection, tunnel and node
// worker has joined. Cancellation alone is not completion. Runners must never
// reenter Owner. Resources/signers borrowed by a runner outlive its Handle.Done.
// Return nil for a successfully joined normal cancellation; a non-nil error
// latches this owner closed and requires explicit trusted-state reopen.
type Runner func(context.Context, trustprofile.Profile) error

type Handle struct {
	done   chan struct{}
	err    error // published by closing done
	cancel context.CancelFunc
}

func (h *Handle) Done() <-chan struct{} { return h.done }
func (h *Handle) Wait(ctx context.Context) error {
	select {
	case <-h.done:
		return h.err
	case <-ctx.Done():
		return ctx.Err()
	}
}

type Owner struct {
	serial         chan struct{}
	journal        trustinstall.Journal
	installed      *trustprofile.Installation
	running        *Handle
	broken, closed bool
}

// Open never initializes or repairs missing/corrupt native state. The journal
// owner lease is held until Close has joined the live runner successfully.
func Open(r trustinstall.Release, f trustinstall.Factory, clock func() time.Time) (*Owner, error) {
	if f == nil || clock == nil {
		return nil, trustprofile.ErrTrust
	}
	// Revalidate the compiled release, rather than trusting a zero value.
	encoded, e := trustinstall.EncodeForTrustedBuild(r.Specification(), time.Unix(r.Specification().Bundled.Claims.IssuedAt, 0))
	if e != nil {
		return nil, e
	}
	r, e = trustinstall.DecodeCompiled(encoded)
	if e != nil {
		return nil, e
	}
	j, e := f(false)
	if e != nil {
		return nil, e
	}
	s, e := trustprofile.OpenInstallation(r.Anchor(), j, clock)
	if e != nil {
		j.Close()
		return nil, e
	}
	return &Owner{serial: make(chan struct{}, 1), journal: j, installed: s}, nil
}

func (o *Owner) lock(ctx context.Context) error {
	if ctx == nil || ctx.Err() != nil {
		return trustprofile.ErrTrust
	}
	select {
	case o.serial <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (o *Owner) unlock() { <-o.serial }
func (o *Owner) healthy() bool {
	if o.running != nil {
		select {
		case <-o.running.done:
			if o.running.err != nil {
				o.broken = true
			}
		default:
		}
	}
	return !o.broken && !o.closed
}

// Start is explicit and never retries enrollment or reconnects automatically.
// No caller supplies a profile: only the committed installation is passed on.
func (o *Owner) Start(ctx context.Context, run Runner) (*Handle, error) {
	if e := o.lock(ctx); e != nil {
		return nil, e
	}
	defer o.unlock()
	if run == nil || !o.healthy() {
		return nil, trustprofile.ErrTrust
	}
	if o.running != nil {
		select {
		case <-o.running.done:
		default:
			return nil, trustprofile.ErrTrust
		}
	}
	p, _, e := o.installed.Current()
	if e != nil {
		o.broken = true
		return nil, e
	}
	// A loaded profile gets a monotonic deadline, independent of later wall
	// clock rollback. Managed HTTPS applies its own deadline as well.
	life, cancel := context.WithDeadline(ctx, time.Now().Add(time.Until(time.Unix(p.Claims.ExpiresAt, 0))))
	h := &Handle{done: make(chan struct{}), cancel: cancel}
	o.running = h
	go func() {
		defer close(h.done)
		defer cancel()
		h.err = run(life, p)
	}()
	return h, nil
}

type quiescer struct{ o *Owner }

func (q quiescer) Quiesce(ctx context.Context) error { return q.o.join(ctx) }
func (o *Owner) join(ctx context.Context) error {
	if o.running == nil {
		return nil
	}
	o.running.cancel()
	select {
	case <-o.running.done:
		if o.running.err != nil {
			return trustprofile.ErrTrust
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (o *Owner) Upgrade(ctx context.Context, p trustprofile.Profile) error {
	if e := o.lock(ctx); e != nil {
		return e
	}
	defer o.unlock()
	if !o.healthy() {
		return trustprofile.ErrTrust
	}
	// Installation verifies before quiescing; an invalid candidate leaves the
	// old session intact. Join/commit failures latch Installation itself closed.
	e := o.installed.Accept(ctx, p, quiescer{o})
	if e != nil {
		if _, _, current := o.installed.Current(); current != nil {
			o.broken = true
		}
	}
	return e
}

// Close may be retried after a timeout only to finish joining. It never unlocks
// the native journal while old code could still use the old authority.
func (o *Owner) Close(ctx context.Context) error {
	if e := o.lock(ctx); e != nil {
		return e
	}
	defer o.unlock()
	if o.closed {
		return nil
	}
	o.broken = true
	if e := o.join(ctx); e != nil {
		// A finished faulty runner is safe to release, but never a pending one.
		if o.running != nil {
			select {
			case <-o.running.done:
			default:
				return e
			}
		}
		closeErr := o.journal.Close()
		o.closed = true
		if closeErr != nil {
			return closeErr
		}
		return e
	}
	e := o.journal.Close()
	o.closed = true
	return e
}

// Group is a structured-ownership building block for trusted runtime adapters.
// Seal prevents new workers racing with join; the context passed to each worker
// is cancelled before waiting. A worker must join its own nested children.
type Group struct {
	mu     sync.Mutex
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	sealed bool
	done   chan struct{}
}

func NewGroup(parent context.Context) *Group {
	ctx, cancel := context.WithCancel(parent)
	return &Group{ctx: ctx, cancel: cancel, done: make(chan struct{})}
}
func (g *Group) Go(run func(context.Context)) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.sealed || g.ctx.Err() != nil || run == nil {
		return false
	}
	g.wg.Add(1)
	go func() { defer g.wg.Done(); run(g.ctx) }()
	return true
}
func (g *Group) Seal() <-chan struct{} {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.sealed {
		g.sealed = true
		g.cancel()
		go func() { g.wg.Wait(); close(g.done) }()
	}
	return g.done
}
