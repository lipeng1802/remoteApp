// Package streamgate joins verified dynamic registration snapshots to the
// existing fixed-payload session. This is not the product TLS protocol.
package streamgate

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"net"
	"net/netip"
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
	if now == nil || len(key) != 64 || len(target) != 32 || len(controller) != 32 ||
		!bytes.Equal(key, ed25519.NewKeyFromSeed(key[:32])) || !state.Claims.Active ||
		!invite.VerifyNetworkState(issuer, state, now()) ||
		!invite.VerifyGrant(issuer, state.Claims.Grant, session.Identity(target), session.Identity(controller), now()) {
		return nil, session.ErrDenied
	}
	expected := controller
	if server {
		expected = target
	}
	if !bytes.Equal(key.Public().(ed25519.PublicKey), expected) {
		return nil, session.ErrDenied
	}
	b := state.Claims.Binding
	c := session.Config{Issuer: append(ed25519.PublicKey(nil), issuer...), Key: append(ed25519.PrivateKey(nil), key...), Grant: state.Claims.Grant, Now: now,
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

func (g *Gate) peer(ctx context.Context, s *tsnet.Server, conn net.Conn) (net.Conn, error) {
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
	go func() {
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
