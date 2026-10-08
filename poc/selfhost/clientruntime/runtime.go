// Package clientruntime connects the installed owner to concrete tsnet and OS
// signer resources. It does not enroll a device, invent a grant or create keys.
package clientruntime

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"remoteapp.local/selfhost-poc/clientowner"
	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/nodeidentity"
	"remoteapp.local/selfhost-poc/nodeproof"
	"remoteapp.local/selfhost-poc/processowner"
	"remoteapp.local/selfhost-poc/session"
	"remoteapp.local/selfhost-poc/streamgate"
	"remoteapp.local/selfhost-poc/trustinstall"
	"remoteapp.local/selfhost-poc/trustprofile"
	"tailscale.com/tsnet"
)

const ControlURL = "https://mk.fengmap.com:8443"

type Config struct {
	StateDir           string
	AuthKey            string
	Verifier           string
	Target, Controller string
	Grant              invite.Grant
	Binding            invite.NetworkBinding
	Server             bool
}
type Signer interface {
	invite.DeviceSigner
	Close()
}
type SignerFactory func() (Signer, error)
type Ready func(port int) // trusted local callback; no secrets/authority passed

func key(s string) (ed25519.PublicKey, error) {
	b, e := hex.DecodeString(s)
	if e != nil || len(b) != 32 || hex.EncodeToString(b) != s {
		return nil, session.ErrDenied
	}
	return b, nil
}
func nodeKey(s string) bool {
	if !strings.HasPrefix(s, "nodekey:") {
		return false
	}
	_, e := key(strings.TrimPrefix(s, "nodekey:"))
	return e == nil
}
func validate(c Config) (ed25519.PublicKey, ed25519.PublicKey, error) {
	t, e := key(c.Target)
	if e != nil {
		return nil, nil, e
	}
	p, e := key(c.Controller)
	if e != nil {
		return nil, nil, e
	}
	if c.AuthKey == "" || len(c.AuthKey) > 256 || !nodeKey(c.Verifier) || !nodeKey(c.Binding.TargetNode) || !nodeKey(c.Binding.ControllerNode) || !filepath.IsAbs(c.StateDir) || filepath.Clean(c.StateDir) != c.StateDir {
		return nil, nil, session.ErrDenied
	}
	// This path must have been explicitly prepared/enrolled earlier. Runtime
	// neither initializes missing node state nor silently replaces corrupt state.
	real, e := filepath.EvalSymlinks(c.StateDir)
	if e != nil || real != c.StateDir {
		return nil, nil, session.ErrDenied
	}
	st, e := os.Lstat(filepath.Join(c.StateDir, "tailscaled.state"))
	if e != nil || !st.Mode().IsRegular() || st.Mode()&os.ModeSymlink != 0 || st.Size() == 0 || st.Size() > 1<<20 {
		return nil, nil, session.ErrDenied
	}
	return t, p, nil
}

// Runner returns a trusted clientowner.Runner, owning a freshly reopened OS
// signer and tsnet server for ONE explicit session. No reconnect or key delete.
// StateDir must be protected by trusted provisioning; journal ownership does
// not make arbitrary node files trusted or protect them from other processes.
func Runner(r trustinstall.Release, c Config, open SignerFactory, ready Ready) clientowner.Runner {
	return runner(r, c, open, ready, nil)
}

// RunnerWithProduct owns the pinned product INSIDE the installation lease.
// Product is trusted compiled/local configuration, not peer-selected argv.
func RunnerWithProduct(r trustinstall.Release, c Config, open SignerFactory, ready Ready, product Product) clientowner.Runner {
	return runner(r, c, open, ready, &product)
}
func runner(r trustinstall.Release, c Config, open SignerFactory, ready Ready, product *Product) clientowner.Runner {
	return func(ctx context.Context, p trustprofile.Profile) (result error) {
		if ctx.Err() != nil {
			return nil
		}
		a := r.Anchor()
		issuer, e := key(p.Claims.Issuer)
		if e != nil {
			return e
		}
		target, controller, e := validate(c)
		if e != nil {
			return e
		}
		if _, e = trustprofile.Verify(p, a.Root[:], a.Service, a.MinimumEpoch, time.Now()); e != nil {
			return e
		}
		if !invite.VerifyGrant(issuer, c.Grant, session.Identity(target), session.Identity(controller), time.Now()) || open == nil {
			return session.ErrDenied
		}
		signer, e := open()
		if e != nil {
			return e
		}
		if signer == nil {
			return session.ErrDenied
		}
		defer func() {
			// NativeSigner exposes the strict process barrier; trusted test
			// signers may implement only Close (and must join synchronously).
			if joined, ok := signer.(interface{ CloseJoined() error }); ok {
				if err := joined.CloseJoined(); err != nil {
					result = err
				}
			} else {
				signer.Close()
			}
		}() // after product, node, managed watch, listener and streams join
		expected := controller
		if c.Server {
			expected = target
		}
		if signer == nil || string(signer.PublicKey()) != string(expected) {
			return session.ErrDenied
		}
		expectedNode, expectedIP := c.Binding.ControllerNode, c.Binding.ControllerIP
		if c.Server {
			expectedNode, expectedIP = c.Binding.TargetNode, c.Binding.TargetIP
		}
		pub, e := nodeidentity.Prepare(c.StateDir, ControlURL)
		if e != nil || pub != expectedNode {
			return session.ErrDenied
		}
		nodeidentity.ConfigureOnline()
		node := &tsnet.Server{Dir: c.StateDir, Hostname: "poc-installed-session", AuthKey: c.AuthKey, ControlURL: ControlURL, Logf: func(string, ...any) {}, UserLogf: func(string, ...any) {}}
		started := false
		defer func() {
			if started {
				e := node.Close()
				if result == nil {
					result = e
				}
			}
		}()
		return streamgate.ProfileSessionWithClient(ctx, r, p, signer, target, controller, c.Grant, c.Binding, c.Server, func(ctx context.Context, g *streamgate.Gate, client *invite.NetworkClient, clock func() time.Time) error {
			// Require an issuer-signed ACTIVE binding watch before Headscale Up.
			started = true
			st, err := node.Up(ctx)
			if err != nil {
				return err
			}
			if st.Self == nil || st.Self.PublicKey.String() != pub {
				return session.ErrDenied
			}
			found := false
			for _, ip := range st.TailscaleIPs {
				if ip.String() == expectedIP {
					found = true
				}
			}
			if !found {
				return session.ErrDenied
			}
			life, cancel := context.WithCancel(ctx)
			defer cancel()
			var ln net.Listener
			if c.Server {
				ln, e = node.Listen("tcp", ":47476")
			} else {
				ln, e = net.Listen("tcp4", "127.0.0.1:0")
			}
			if e != nil {
				return e
			}
			proof := func(ctx context.Context) error {
				return nodeproof.ServeBoundWithSigner(ctx, node, signer, c.Verifier, c.Grant.Claims.ID)
			}
			stream := func(ctx context.Context, conn net.Conn) error {
				if c.Server {
					return g.ServeTLS(ctx, node, conn)
				}
				return g.ConnectTLS(ctx, node, conn)
			}
			port := 0
			if !c.Server {
				port = ln.Addr().(*net.TCPAddr).Port
			}
			work := func(ctx context.Context) error {
				return supervise(ctx, ln, proof, stream, func() {
					if ready != nil {
						ready(port)
					}
				})
			}
			if product == nil {
				return work(life)
			}
			child, err := product.start(life, client, issuer, c, clock, port)
			if err != nil {
				ln.Close()
				if child != nil {
					if closeErr := child.Stop(context.Background()); closeErr != nil {
						return processowner.ErrProcess
					}
				}
				return err
			}
			return processowner.Own(life, child, work)
		})
	}
}

// No detached close/wait timeout here. The outer owner supplies the deadline
// and retains its journal lease until this entire supervisor actually returns.
func supervise(ctx context.Context, ln net.Listener, proof func(context.Context) error, stream func(context.Context, net.Conn) error, ready func()) error {
	life, cancel := context.WithCancel(ctx)
	defer cancel()
	var wg sync.WaitGroup
	errors := make(chan error, 2)
	finished := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		select {
		case <-life.Done():
			ln.Close()
		case <-finished:
		}
	}()
	if proof != nil {
		wg.Add(1)
		go func() { defer wg.Done(); e := proof(life); errors <- e; cancel() }()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			conn, e := ln.Accept()
			if e != nil {
				errors <- e
				cancel()
				return
			}
			// One active TLS stream per session. The stream owns and closes conn.
			stream(life, conn)
			conn.Close()
			if life.Err() != nil {
				return
			}
		}
	}()
	if ready != nil {
		ready()
	}
	<-life.Done()
	ln.Close()
	wg.Wait()
	close(finished)
	if ctx.Err() != nil {
		return nil
	}
	return session.ErrDenied // unexpected listener/proof exit is never usable
}
