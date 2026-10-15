package installeddevice

import (
	"context"
	"net"

	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/streamgate"
	"tailscale.com/tsnet"
)

// sessionEnrollmentNode is deliberately private. Product work receives a
// SessionRuntime, never the raw tsnet server or an arbitrary dial address.
type sessionEnrollmentNode interface {
	sessionServer() *tsnet.Server
}

func enrolledSessionServer(node EnrollmentNode) *tsnet.Server {
	if n, ok := node.(sessionEnrollmentNode); ok {
		return n.sessionServer()
	}
	return nil
}

// SessionRuntime is the only overlay capability exposed to compiled product
// work. ListenOverlay is fixed to :47476 and DialOverlay is fixed to the
// issuer-signed target binding. The gate also checks the live authorization,
// local node identity/address and remote WhoIs identity before returning a
// connection.
type SessionRuntime struct {
	peers *SessionPeers
	gate  *streamgate.Gate
	node  *tsnet.Server
}

func (r *SessionRuntime) Peers() SessionPeers {
	if r == nil || r.peers == nil {
		return SessionPeers{}
	}
	p := *r.peers
	p.target = r.peers.TargetPublic()
	p.controller = r.peers.ControllerPublic()
	p.runtime = nil
	return p
}

func (r *SessionRuntime) ListenOverlay(ctx context.Context) (net.Listener, error) {
	if ctx == nil || r == nil || r.peers == nil || !r.peers.server || r.gate == nil || r.node == nil {
		return nil, invite.ErrDenied
	}
	return r.gate.ListenOverlay(ctx, r.node)
}

func (r *SessionRuntime) DialOverlay(ctx context.Context) (net.Conn, error) {
	if ctx == nil || r == nil || r.peers == nil || r.peers.server || r.gate == nil || r.node == nil {
		return nil, invite.ErrDenied
	}
	return r.gate.DialOverlay(ctx, r.node)
}

func (r *SessionRuntime) ServeTLS(ctx context.Context, conn net.Conn) error {
	if ctx == nil || r == nil || r.peers == nil || !r.peers.server || r.gate == nil || r.node == nil || conn == nil {
		return invite.ErrDenied
	}
	return r.gate.ServeTLS(ctx, r.node, conn)
}

func (r *SessionRuntime) ConnectTLS(ctx context.Context, local net.Conn) error {
	if ctx == nil || r == nil || r.peers == nil || r.peers.server || r.gate == nil || r.node == nil || local == nil {
		return invite.ErrDenied
	}
	return r.gate.ConnectTLS(ctx, r.node, local)
}
