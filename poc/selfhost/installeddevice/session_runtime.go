package installeddevice

import (
	"context"
	"crypto/ed25519"
	"net"
	"remoteapp.local/selfhost-poc/startuptrace"
	"time"

	"remoteapp.local/selfhost-poc/clientruntime"
	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/processowner"
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
	peers  *SessionPeers
	gate   *streamgate.Gate
	node   *tsnet.Server
	client clientruntime.ProductNetworkAPI
	issuer ed25519.PublicKey
	clock  func() time.Time
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

// ProductSession returns trusted in-process work that owns one pinned product
// child. HMAC and certificate fingerprint never cross this callback boundary.
func ProductSession(product clientruntime.Product) SessionWork {
	return func(ctx context.Context, runtime *SessionRuntime) error {
		return runtime.RunProduct(ctx, product)
	}
}

// RunProduct starts the fixed-digest product for this signed role. The target
// publishes certificate possession, then delivers its fresh HMAC and exact
// fingerprint over a one-shot mutually authenticated overlay exchange. The
// controller independently verifies the issuer TLS descriptor before its
// product receives the secret over the private child pipe.
func (r *SessionRuntime) RunProduct(ctx context.Context, product clientruntime.Product) error {
	if ctx == nil || r == nil || r.peers == nil || r.gate == nil || r.node == nil || r.client == nil || len(r.issuer) != ed25519.PublicKeySize || r.clock == nil || product.ControllerKey != "" || product.TargetReady != nil {
		return invite.ErrDenied
	}
	if r.peers.server {
		return r.runTargetProduct(ctx, product)
	}
	return r.runControllerProduct(ctx, product)
}

func (r *SessionRuntime) runTargetProduct(ctx context.Context, product clientruntime.Product) error {
	var key, fingerprint string
	called := false
	product.TargetReady = func(k, fp string) {
		if !called {
			called, key, fingerprint = true, k, fp
		}
	}
	startuptrace.Emit(ctx, startuptrace.TargetTLS)
	child, err := clientruntime.StartBoundProduct(ctx, product, r.client, r.issuer, r.peers.grant, r.peers.binding, true, r.clock, 0, "")
	if err != nil || child == nil || !called {
		return stopPartialProduct(child, err)
	}
	return processowner.Own(ctx, child, func(life context.Context) error {
		startuptrace.Emit(life, startuptrace.TargetListen)
		listener, err := trackedSessionListener(life, func() (net.Listener, error) { return r.ListenOverlay(life) })
		if err != nil {
			return err
		}
		defer listener.Close()
		secretConn, err := listener.Accept()
		if err != nil {
			return runtimeWorkerError(life, err)
		}
		startuptrace.Emit(life, startuptrace.TargetSecret)
		if err = r.gate.DeliverProductSecret(life, secretConn, key, fingerprint); err != nil {
			return runtimeWorkerError(life, err)
		}
		// Erase the owner's copies as soon as the authenticated one-shot delivery
		// finishes. The target product retains its own private session key.
		key, fingerprint = "", ""
		startuptrace.Emit(life, startuptrace.TargetServe)
		for {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				return runtimeWorkerError(life, acceptErr)
			}
			_ = r.ServeTLS(life, conn)
			if life.Err() != nil {
				return nil
			}
		}
	})
}

func (r *SessionRuntime) runControllerProduct(ctx context.Context, product clientruntime.Product) error {
	if product.CertificateScope != "" || product.CertificateReopen || product.CertificateOpened != nil {
		return invite.ErrDenied
	}
	startuptrace.Emit(ctx, startuptrace.ControllerListen)
	listener, err := trackedSessionListener(ctx, func() (net.Listener, error) { return net.Listen("tcp4", "127.0.0.1:0") })
	if err != nil {
		return invite.ErrDenied
	}
	defer listener.Close()
	startuptrace.Emit(ctx, startuptrace.ControllerDial)
	overlay, err := r.DialOverlay(ctx)
	if err != nil {
		return err
	}
	startuptrace.Emit(ctx, startuptrace.ControllerSecret)
	key, fingerprint, err := r.gate.ReceiveProductSecret(ctx, overlay)
	if err != nil {
		return err
	}
	product.ControllerKey = key
	port := listener.Addr().(*net.TCPAddr).Port
	startuptrace.Emit(ctx, startuptrace.ControllerTLS)
	child, err := clientruntime.StartBoundProduct(ctx, product, r.client, r.issuer, r.peers.grant, r.peers.binding, false, r.clock, port, fingerprint)
	key, fingerprint = "", ""
	product.ControllerKey = ""
	if err != nil || child == nil {
		return stopPartialProduct(child, err)
	}
	return processowner.Own(ctx, child, func(life context.Context) error {
		closed := make(chan struct{})
		workerDone := make(chan struct{})
		go func() {
			defer close(workerDone)
			select {
			case <-life.Done():
				listener.Close()
			case <-closed:
			}
		}()
		defer func() { close(closed); <-workerDone }()
		startuptrace.Emit(life, startuptrace.ControllerAccept)
		local, err := listener.Accept()
		if err != nil {
			return runtimeWorkerError(life, err)
		}
		startuptrace.Emit(life, startuptrace.ControllerBridge)
		return runtimeWorkerError(life, r.ConnectTLS(life, local))
	})
}

func stopPartialProduct(child *processowner.Child, cause error) error {
	if child != nil && child.Stop(context.Background()) != nil {
		return processowner.ErrProcess
	}
	if cause == nil {
		return invite.ErrDenied
	}
	return cause
}

func runtimeWorkerError(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return nil
	}
	return err
}
