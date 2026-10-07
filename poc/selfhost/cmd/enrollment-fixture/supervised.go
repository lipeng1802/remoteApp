package main

import (
	"context"
	"encoding/hex"
	"net/http"
	"os"
	"path/filepath"
	"remoteapp.local/selfhost-poc/broker"
	"remoteapp.local/selfhost-poc/headscale"
	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/verifier"
	"sync"
	"time"
)

// Isolated privileged fixture only. It does not replace installed services.
type supervisedFixture struct {
	store                      *invite.Store
	runtime                    *verifier.Runtime
	supervisor                 *broker.NetworkSupervisor
	runner                     headscale.Run
	dir, runtimeDir, socketDir string
	dispatch                   *fixtureDispatch
	cancel                     context.CancelFunc
	done                       chan error
	report                     func(string)
}
type fixtureDispatch struct {
	mu sync.RWMutex
	a  *broker.Authority
}

func (d *fixtureDispatch) Execute(ctx context.Context, r broker.Request) broker.Reply {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.a == nil {
		return broker.Reply{Status: 503}
	}
	return d.a.Execute(ctx, r)
}
func (d *fixtureDispatch) set(a *broker.Authority) { d.mu.Lock(); defer d.mu.Unlock(); d.a = a }

func newSupervisedFixture(dir string, st *invite.Store, runner headscale.Run, report func(string)) (*supervisedFixture, error) {
	f := &supervisedFixture{dir: dir, store: st, runner: runner, runtimeDir: filepath.Join(dir, "persistent-verifier"), report: report, dispatch: &fixtureDispatch{}}
	raw := &headscale.Driver{Run: runner}
	r, e := verifier.Open(context.Background(), f.runtimeDir, true, raw, verifier.NewOverlay)
	if e != nil {
		// Unknown initial Mint outcomes retain the verifier journal for scoped
		// recovery; never let the caller erase it as an ordinary temp failure.
		return f, e
	}
	f.runtime = r
	return startFixtureBroker(f)
}

func startFixtureBroker(f *supervisedFixture) (*supervisedFixture, error) {
	fail := func() (*supervisedFixture, error) {
		if f.Close() != nil {
			return f, invite.ErrDenied
		}
		return nil, invite.ErrDenied
	}
	socketDir, e := os.MkdirTemp("/tmp", "ras-")
	if e != nil {
		return fail()
	}
	f.socketDir = socketDir
	f.socketDir, e = filepath.EvalSymlinks(socketDir)
	if e != nil {
		return fail()
	}
	if os.Chown(f.socketDir, os.Geteuid(), os.Getegid()) != nil {
		return fail()
	}
	ctx, cancel := context.WithCancel(context.Background())
	f.cancel = cancel
	f.done = make(chan error, 1)
	ready := make(chan struct{}, 1)
	go func() {
		f.done <- broker.Serve(ctx, broker.Config{Socket: filepath.Join(f.socketDir, "authority.sock"), PeerUID: uint32(os.Geteuid()), GroupID: uint32(os.Getegid()), Executor: f.dispatch}, func() { ready <- struct{}{} })
	}()
	select {
	case <-ready:
		return f, nil
	case <-f.done:
		f.done = nil
		return fail()
	case <-time.After(5 * time.Second):
		return fail()
	}
}
func (f *supervisedFixture) Start(st *invite.Store, interval time.Duration) (*invite.ProvisionBackend, *headscale.Driver, error) {
	if f.supervisor != nil {
		ctx, c := context.WithTimeout(context.Background(), 25*time.Second)
		e := f.supervisor.Stop(ctx)
		c()
		if e != nil {
			return nil, nil, e
		}
	}
	f.store = st
	d := &headscale.Driver{Run: f.runner, PolicyDir: f.dir, PolicyMode: "database", VerifierIP: f.runtime.Info().IP}
	d.Proof = func(ctx context.Context, r invite.ProvisionRecord, o invite.ProvisionObservation) bool {
		pub, e := st.DevicePublicKey(r.Device)
		return e == nil && f.runtime.VerifyWithReport(ctx, r, o, pub, f.report)
	}
	b := &invite.ProvisionBackend{Store: st, Driver: d}
	s, e := broker.StartNetworkSupervisor(context.Background(), b, f.runtime, interval)
	if e != nil {
		return nil, nil, e
	}
	f.supervisor = s
	end := time.Now().Add(10 * time.Second)
	for !s.Ready() {
		if time.Now().After(end) {
			return nil, nil, invite.ErrDenied
		}
		time.Sleep(10 * time.Millisecond)
	}
	f.dispatch.set(s.Authority())
	return b, d, nil
}
func (f *supervisedFixture) Handler() http.Handler {
	return broker.PrivateNetworkHandler(broker.Client{Socket: filepath.Join(f.socketDir, "authority.sock"), ServerUID: uint32(os.Geteuid())}, hex.EncodeToString(f.store.PublicKey()))
}
func (f *supervisedFixture) Stop(ctx context.Context) error {
	if f.supervisor == nil {
		return nil
	}
	return f.supervisor.Stop(ctx)
}

func (f *supervisedFixture) ResumeVerifier(ctx context.Context) error {
	if f.Stop(ctx) != nil {
		return invite.ErrDenied
	}
	if f.runtime.Stop(ctx) != nil {
		return invite.ErrDenied
	}
	previous := f.runtime.Info()
	r, e := verifier.Open(ctx, f.runtimeDir, false, &headscale.Driver{Run: f.runner}, verifier.NewOverlay)
	if e != nil {
		return e
	}
	if r.Info().Key != previous.Key || r.Info().IP != previous.IP {
		r.Stop(ctx)
		return invite.ErrDenied
	}
	f.runtime = r
	return nil
}
func (f *supervisedFixture) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if f.Stop(ctx) != nil {
		return invite.ErrDenied
	}
	if f.cancel != nil {
		f.cancel()
		if f.done != nil {
			select {
			case e := <-f.done:
				if e != nil {
					return e
				}
			case <-ctx.Done():
				return invite.ErrDenied
			}
		}
	}
	if f.runtime != nil {
		if e := f.runtime.Stop(ctx); e != nil {
			return e
		}
		if e := verifier.Retire(ctx, f.runtimeDir, &headscale.Driver{Run: f.runner}); e != nil {
			return e
		}
	}
	if f.socketDir != "" {
		if e := os.Remove(f.socketDir); e != nil {
			return e
		}
	}
	return nil
}
