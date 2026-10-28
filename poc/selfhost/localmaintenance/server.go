package localmaintenance

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"remoteapp.local/selfhost-poc/clientowner"
	"remoteapp.local/selfhost-poc/privatepipe"
	"remoteapp.local/selfhost-poc/trustinstall"
	"remoteapp.local/selfhost-poc/trustprofile"
)

// Owner remains a concrete installed owner in product wiring. The interface is
// useful for protocol fault tests; state diagnostics cannot implement its lease.
type Owner interface {
	MaintainDurableState(context.Context, trustinstall.Release, func(context.Context, trustprofile.MaintenanceState) error) error
	Close(context.Context) error
}

var _ Owner = (*clientowner.Owner)(nil)

type Server struct {
	release  trustinstall.Release
	owner    Owner
	signer   Signer
	public   ed25519.PublicKey
	listener net.Listener
	instance string
	started  atomic.Bool
	mu       sync.Mutex
	active   net.Conn
	cancel   context.CancelFunc
	closed   bool
}

// NewTest accepts only a loopback TCP listener and compiled TEST release. The
// signer must be a dedicated, pinned OS identity reopened from trusted native
// binding, independently owned until Serve joins. No key is read from a frame.
// Same installed-user device-key possession is the TEST authorization policy;
// this is not publisher identity validation or a production consent workflow.
func NewTest(r trustinstall.Release, o Owner, s Signer, l net.Listener) (*Server, error) {
	if !r.Specification().Test || o == nil || s == nil || l == nil || len(s.PublicKey()) != 32 || !localAddress(l.Addr().String()) {
		return nil, ErrDenied
	}
	if _, ok := l.Addr().(*net.TCPAddr); !ok {
		return nil, ErrDenied
	}
	instance, e := random(16)
	if e != nil {
		return nil, e
	}
	return &Server{release: r, owner: o, signer: s, public: append(ed25519.PublicKey(nil), s.PublicKey()...), listener: l, instance: instance}, nil
}

func (s *Server) Close() error {
	s.mu.Lock()
	s.closed = true
	active, cancel := s.active, s.cancel
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	s.listener.Close()
	if active != nil {
		active.Close()
	}
	return nil
}

// Serve processes bounded invalid attempts without retiring the owner. Exactly
// one authenticated request may reach Maintain; error/cancel never rearms it.
// Caller must wait for Serve to return before releasing signer/owner resources.
func (s *Server) Serve(ctx context.Context) (trustprofile.MaintenanceState, error) {
	if ctx == nil || ctx.Err() != nil || !s.started.CompareAndSwap(false, true) {
		return trustprofile.MaintenanceState{}, ErrDenied
	}
	life, cancel := context.WithCancel(ctx)
	defer cancel()
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return trustprofile.MaintenanceState{}, ErrDenied
	}
	s.cancel = cancel
	s.mu.Unlock()
	ctx = life
	stop := context.AfterFunc(ctx, func() { s.Close() })
	defer stop()
	defer s.Close()
	for attempts := 0; attempts < 16; attempts++ {
		conn, e := s.listener.Accept()
		if e != nil {
			return trustprofile.MaintenanceState{}, ErrDenied
		}
		s.mu.Lock()
		s.active = conn
		s.mu.Unlock()
		if ctx.Err() != nil {
			conn.Close()
			return trustprofile.MaintenanceState{}, ctx.Err()
		}
		state, authorized, e := s.exchange(ctx, conn)
		conn.Close()
		s.mu.Lock()
		s.active = nil
		s.mu.Unlock()
		if authorized {
			return state, e
		}
	}
	return trustprofile.MaintenanceState{}, ErrDenied
}

func (s *Server) exchange(ctx context.Context, c net.Conn) (state trustprofile.MaintenanceState, authorized bool, err error) {
	if !localAddress(c.RemoteAddr().String()) {
		return state, false, ErrDenied
	}
	life, cancel := context.WithTimeout(ctx, 40*time.Second)
	defer cancel()
	deadline, _ := life.Deadline()
	initial := time.Now().Add(3 * time.Second)
	if deadline.Before(initial) {
		initial = deadline
	}
	if c.SetDeadline(initial) != nil {
		return state, false, ErrDenied
	}
	stop := context.AfterFunc(life, func() { c.Close() })
	defer stop()
	read := privatepipe.NewBoundedReader(c, 4096)
	write := json.NewEncoder(c)
	var hello frame
	if read.Read(&hello) != nil || !canonical(hello.ClientNonce, 32) || hello != (frame{Op: "hello", Version: 1, Scope: s.release.Scope(), Release: releaseDigest(s.release), ClientNonce: hello.ClientNonce}) {
		return state, false, ErrDenied
	}
	nonce, e := random(32)
	if e != nil {
		return state, false, e
	}
	base := frame{Op: "challenge", Version: 1, Scope: s.release.Scope(), Release: releaseDigest(s.release), Instance: s.instance, ClientNonce: hello.ClientNonce, ServerNonce: nonce}
	challenge, e := sign(base, s.signer)
	if e != nil || write.Encode(challenge) != nil {
		return state, false, ErrDenied
	}
	var proof frame
	base.Op = "authenticate"
	if read.Read(&proof) != nil || !verify(proof, base, s.public) {
		return state, false, ErrDenied
	}
	if c.SetDeadline(deadline) != nil {
		return state, true, ErrDenied
	}
	// From this point the endpoint is spent even if join/output/finish fails.
	authorized = true
	err = s.owner.MaintainDurableState(life, s.release, func(joined context.Context, current trustprofile.MaintenanceState) error {
		if current.State != "pending" || current.Epoch < s.release.Specification().MinimumEpoch || !canonical(current.Transaction, 16) {
			return ErrDenied
		}
		state = current
		base.Op = "ready"
		base.Epoch = current.Epoch
		base.Transaction = current.Transaction
		ready, e := sign(base, s.signer)
		if e != nil || write.Encode(ready) != nil {
			return ErrDenied
		}
		var finish frame
		base.Op = "finish"
		if read.Read(&finish) != nil || !verify(finish, base, s.public) || joined.Err() != nil {
			return ErrDenied
		}
		return nil
	})
	if err != nil {
		return state, true, err
	}
	if err = s.owner.Close(life); err != nil {
		return state, true, err
	}
	base.Op = "closed"
	closed, e := sign(base, s.signer)
	if e != nil || write.Encode(closed) != nil {
		return state, true, ErrDenied
	}
	return state, true, nil
}
