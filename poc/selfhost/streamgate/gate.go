// Package streamgate joins verified dynamic registration snapshots to the
// fixed-payload session and the opaque bridge to actual product TLS endpoints.
package streamgate

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"net"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/nodeproof"
	"remoteapp.local/selfhost-poc/session"
	"tailscale.com/tsnet"
)

type Gate struct {
	cfg     session.Config
	binding invite.NetworkBinding
	cache   *session.StateCache
	server  bool
	denied  atomic.Bool
}

func New(issuer, target, controller ed25519.PublicKey, key ed25519.PrivateKey, state invite.NetworkState, server bool, now func() time.Time) (*Gate, error) {
	signer, err := invite.NewMemorySigner(key)
	if err != nil {
		return nil, session.ErrDenied
	}
	return NewWithSigner(issuer, target, controller, signer, state, server, now)
}
func NewWithSigner(issuer, target, controller ed25519.PublicKey, signer invite.DeviceSigner, state invite.NetworkState, server bool, now func() time.Time) (*Gate, error) {
	if now == nil || signer == nil || len(target) != 32 || len(controller) != 32 || !state.Claims.Active ||
		!invite.VerifyNetworkState(issuer, state, now()) ||
		!invite.VerifyGrant(issuer, state.Claims.Grant, session.Identity(target), session.Identity(controller), now()) {
		return nil, session.ErrDenied
	}
	expected := controller
	if server {
		expected = target
	}
	if !bytes.Equal(signer.PublicKey(), expected) {
		return nil, session.ErrDenied
	}
	b := state.Claims.Binding
	c := session.Config{Issuer: append(ed25519.PublicKey(nil), issuer...), Signer: signer, Grant: state.Claims.Grant, Now: now,
		Binding: session.Binding{Target: append(ed25519.PublicKey(nil), target...), Controller: append(ed25519.PublicKey(nil), controller...), TargetIP: netip.MustParseAddr(b.TargetIP), ControllerIP: netip.MustParseAddr(b.ControllerIP)}}
	cache := session.NewStateCache(c.Issuer, c.Grant, b, now)
	if !cache.Update(state) {
		return nil, session.ErrDenied
	}
	return &Gate{cfg: c, binding: b, cache: cache, server: server}, nil
}

func (g *Gate) Update(n invite.NetworkState) bool { return g.cache.Update(n) }
func (g *Gate) Fresh(n invite.NetworkState) bool {
	return n.Claims.Grant == g.cfg.Grant && n.Claims.Binding == g.binding && invite.VerifyNetworkState(g.cfg.Issuer, n, g.cfg.Now())
}
func (g *Gate) Deny() { g.denied.Store(true) }
func (g *Gate) active(ctx context.Context) bool {
	return !g.denied.Load() && g.cache.Check(ctx, g.cfg.Grant, g.cfg.Grant.Claims.Target, g.cfg.Grant.Claims.Controller) &&
		invite.VerifyGrant(g.cfg.Issuer, g.cfg.Grant, g.cfg.Grant.Claims.Target, g.cfg.Grant.Claims.Controller, g.cfg.Now())
}

// LocalAPI dialing returns a local proxy socket. Only after actual self/map
// identity and WhoIs for the explicit overlay destination succeed do we expose
// authenticated overlay endpoints to session's endpoint/transcript checks.
type overlayConn struct {
	net.Conn
	local, remote net.Addr
}

func (c overlayConn) LocalAddr() net.Addr  { return c.local }
func (c overlayConn) RemoteAddr() net.Addr { return c.remote }

type overlayListener struct {
	net.Listener
	gate   *Gate
	server *tsnet.Server
	ctx    context.Context
	done   chan struct{}
	once   sync.Once
}

func (l *overlayListener) Accept() (net.Conn, error) {
	if !l.gate.active(l.ctx) {
		return nil, session.ErrDenied
	}
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	verified, err := l.gate.peer(l.ctx, l.server, conn)
	if err != nil {
		conn.Close()
		return nil, err
	}
	return verified, nil
}

func (l *overlayListener) Close() error {
	var err error
	l.once.Do(func() {
		close(l.done)
		err = l.Listener.Close()
	})
	return err
}

func (g *Gate) peer(ctx context.Context, s *tsnet.Server, conn net.Conn) (net.Conn, error) {
	if s == nil || conn == nil {
		return nil, session.ErrDenied
	}
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	if !g.active(ctx) {
		return nil, session.ErrDenied
	}
	lc, e := s.LocalClient()
	if e != nil {
		return nil, session.ErrDenied
	}
	st, e := lc.Status(ctx)
	localIP, remoteIP := g.binding.ControllerIP, g.binding.TargetIP
	localKey, remoteKey := g.binding.ControllerNode, g.binding.TargetNode
	if g.server {
		localIP, remoteIP, localKey, remoteKey = remoteIP, localIP, remoteKey, localKey
	}
	if e != nil || st.Self == nil || st.Self.PublicKey.String() != localKey {
		return nil, session.ErrDenied
	}
	found := false
	for _, ip := range st.TailscaleIPs {
		if ip.String() == localIP {
			found = true
		}
	}
	if !found {
		return nil, session.ErrDenied
	}
	if g.server {
		l, e1 := netip.ParseAddrPort(conn.LocalAddr().String())
		r, e2 := netip.ParseAddrPort(conn.RemoteAddr().String())
		if e1 != nil || e2 != nil || l.Addr().String() != localIP || r.Addr().String() != remoteIP {
			return nil, session.ErrDenied
		}
	}
	w, e := lc.WhoIs(ctx, remoteIP)
	if e != nil || w == nil || w.Node == nil || w.Node.Key.String() != remoteKey {
		return nil, session.ErrDenied
	}
	if g.server {
		return conn, nil
	}
	return overlayConn{conn, &net.TCPAddr{IP: net.ParseIP(localIP)}, &net.TCPAddr{IP: net.ParseIP(remoteIP), Port: 47476}}, nil
}

// ListenOverlay is the target's only overlay listener capability. The caller
// cannot select an address or port, and the local node identity is checked
// before the listener is exposed.
func (g *Gate) ListenOverlay(ctx context.Context, s *tsnet.Server) (net.Listener, error) {
	if ctx == nil || s == nil || !g.server || !g.active(ctx) {
		return nil, session.ErrDenied
	}
	lc, err := s.LocalClient()
	if err != nil {
		return nil, session.ErrDenied
	}
	check, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	st, err := lc.Status(check)
	if err != nil || st.Self == nil || st.Self.PublicKey.String() != g.binding.TargetNode {
		return nil, session.ErrDenied
	}
	found := false
	for _, ip := range st.TailscaleIPs {
		if ip.String() == g.binding.TargetIP {
			found = true
		}
	}
	if !found {
		return nil, session.ErrDenied
	}
	listener, err := s.Listen("tcp", ":47476")
	if err != nil || !g.active(ctx) {
		if listener != nil {
			listener.Close()
		}
		return nil, session.ErrDenied
	}
	wrapped := &overlayListener{Listener: listener, gate: g, server: s, ctx: ctx, done: make(chan struct{})}
	go func() {
		select {
		case <-ctx.Done():
			wrapped.Close()
		case <-wrapped.done:
		}
	}()
	return wrapped, nil
}

// DialOverlay is the controller's only overlay dial capability. It always
// dials the signed target binding on port 47476, then verifies both the local
// node and remote WhoIs identity before exposing the connection.
func (g *Gate) DialOverlay(ctx context.Context, s *tsnet.Server) (net.Conn, error) {
	if ctx == nil || s == nil || g.server || !g.active(ctx) {
		return nil, session.ErrDenied
	}
	lc, err := s.LocalClient()
	if err != nil {
		return nil, session.ErrDenied
	}
	dialCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	conn, err := nodeproof.DialOnlyOverlay(dialCtx, lc, g.binding.TargetIP, 47476)
	if err != nil {
		cancel()
		return nil, session.ErrDenied
	}
	verified, err := g.peer(ctx, s, conn)
	if err != nil {
		conn.Close()
		cancel()
		return nil, err
	}
	return &overlayDialConn{Conn: verified, cancel: cancel}, nil
}

type overlayDialConn struct {
	net.Conn
	cancel context.CancelFunc
}

func (c *overlayDialConn) Close() error {
	err := c.Conn.Close()
	c.cancel()
	return err
}

func (g *Gate) Serve(ctx context.Context, s *tsnet.Server, conn net.Conn) error {
	defer conn.Close()
	if !g.server {
		return session.ErrDenied
	}
	c, e := g.peer(ctx, s, conn)
	if e != nil {
		return e
	}
	return session.Serve(ctx, c, g.cfg, func(ctx context.Context, grant invite.Grant, target, controller string) bool {
		return grant == g.cfg.Grant && target == grant.Claims.Target && controller == grant.Claims.Controller && g.active(ctx)
	})
}

func (g *Gate) Probe(ctx context.Context, s *tsnet.Server, received func()) error {
	if g.server || !g.active(ctx) {
		return session.ErrDenied
	}
	lc, e := s.LocalClient()
	if e != nil {
		return session.ErrDenied
	}
	dialCtx, stop := context.WithCancel(ctx)
	timer := time.AfterFunc(3*time.Second, stop)
	conn, e := nodeproof.DialOnlyOverlay(dialCtx, lc, g.binding.TargetIP, 47476)
	timer.Stop()
	// Keep the upgrade request context alive for the entire stream, otherwise
	// cancelling it can close the upgraded socket. It is released on return.
	defer stop()
	if e != nil {
		return session.ErrDenied
	}
	defer conn.Close()
	c, e := g.peer(ctx, s, conn)
	if e != nil {
		return e
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	watchJoined := make(chan struct{})
	defer func() { cancel(); <-watchJoined }()
	go func() {
		defer close(watchJoined)
		t := time.NewTicker(250 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if !g.active(ctx) {
					cancel()
					return
				}
			}
		}
	}()
	return session.Probe(ctx, c, g.cfg, received)
}

func (g *Gate) check(ctx context.Context, grant invite.Grant, target, controller string) bool {
	return grant == g.cfg.Grant && target == grant.Claims.Target && controller == grant.Claims.Controller && g.active(ctx)
}

// The only target is the development-only product fixed-payload TLS listener.
// No caller/peer supplied destination and no product native actions are exposed.
func (g *Gate) ServeTLS(ctx context.Context, s *tsnet.Server, conn net.Conn) error {
	defer conn.Close()
	if !g.server {
		return session.ErrDenied
	}
	c, e := g.peer(ctx, s, conn)
	if e != nil {
		return e
	}
	return session.Tunnel(ctx, c, g.cfg, g.check, true, func(ctx context.Context) (net.Conn, error) {
		return (&net.Dialer{Timeout: time.Second}).DialContext(ctx, "tcp", "127.0.0.1:47478")
	})
}
func (g *Gate) ConnectTLS(ctx context.Context, s *tsnet.Server, local net.Conn) error {
	defer local.Close()
	if g.server || !g.active(ctx) {
		return session.ErrDenied
	}
	lc, e := s.LocalClient()
	if e != nil {
		return session.ErrDenied
	}
	dialCtx, stop := context.WithCancel(ctx)
	defer stop()
	timer := time.AfterFunc(3*time.Second, stop)
	conn, e := nodeproof.DialOnlyOverlay(dialCtx, lc, g.binding.TargetIP, 47476)
	timer.Stop()
	if e != nil {
		return session.ErrDenied
	}
	defer conn.Close()
	c, e := g.peer(ctx, s, conn)
	if e != nil {
		return e
	}
	return session.Tunnel(ctx, c, g.cfg, g.check, false, func(context.Context) (net.Conn, error) { return local, nil })
}
