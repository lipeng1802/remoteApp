package broker

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"remoteapp.local/selfhost-poc/invite"
)

// LiveVerifier is a trusted server-owned dependency, never a client IPC flag.
// Live does not replace device signatures, grant checks or fresh nodeproof.
type LiveVerifier interface {
	Live() bool
	Done() <-chan struct{}
}

// NetworkSupervisor owns backend maintenance and network request contexts.
// Any verifier/policy/storage fault is terminal for this instance: explicit
// reconstruction is required, never automatic recovery after a good cycle.
type NetworkSupervisor struct {
	backend     *invite.ProvisionBackend
	driver      invite.ProvisionDriver
	verifier    LiveVerifier
	maintenance *invite.Maintenance
	ctx         context.Context
	cancel      context.CancelFunc
	closed      atomic.Bool
	withdrawn   atomic.Bool
	done        chan struct{}
	mu          sync.Mutex
	requests    sync.WaitGroup
}

func StartNetworkSupervisor(ctx context.Context, b *invite.ProvisionBackend, v LiveVerifier, interval time.Duration) (*NetworkSupervisor, error) {
	if ctx.Err() != nil || b == nil || b.Store == nil || b.Driver == nil || v == nil || !v.Live() || interval < 100*time.Millisecond || interval > time.Minute {
		return nil, ErrBroker
	}
	if _, alreadyOwned := b.Driver.(*supervisedDriver); alreadyOwned {
		return nil, ErrBroker
	}
	run, cancel := context.WithCancel(ctx)
	original := b.Driver
	// Owned, unused backend only: never swap Driver under active workers.
	guard := &supervisedDriver{inner: original, ctx: run}
	b.Driver = guard
	// No worker or IPC is enabled until a complete initial projection succeeds.
	if err := b.Reconcile(run); err != nil || !v.Live() || run.Err() != nil {
		cancel()
		b.Fence()
		return nil, ErrBroker
	}
	m, err := b.StartMaintenance(run, interval, interval)
	if err != nil {
		cancel()
		b.Fence()
		return nil, ErrBroker
	}
	s := &NetworkSupervisor{backend: b, driver: original, verifier: v, maintenance: m, ctx: run, cancel: cancel, done: make(chan struct{})}
	go s.watch()
	return s, nil
}

func (s *NetworkSupervisor) Ready() bool {
	if s == nil || s.closed.Load() || s.ctx.Err() != nil || !s.verifier.Live() || !s.backend.Store.Available() {
		return false
	}
	st := s.maintenance.Status()
	if health, ok := s.backend.Driver.(interface{ Healthy() bool }); ok && !health.Healthy() {
		return false
	}
	return st.Ready && st.Failures == 0 && !st.Stopped
}

func (s *NetworkSupervisor) watch() {
	defer close(s.done)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-s.ctx.Done():
			goto stopped
		case <-s.verifier.Done():
			goto stopped
		case <-ticker.C:
		}
		st := s.maintenance.Status()
		if health, ok := s.backend.Driver.(interface{ Healthy() bool }); ok && !health.Healthy() {
			break
		}
		if !s.verifier.Live() || !s.backend.Store.Available() || st.Failures != 0 || st.Stopped {
			break
		}
	}
stopped:
	// Close the admission latch before cancellation/join. Context cancellation
	// reaches already-running Mint/Observe/Verify/ReplaceRules via Execute.
	s.mu.Lock()
	s.closed.Store(true)
	s.cancel()
	s.mu.Unlock()
	// Store/driver must not be closed by the caller until Done. No fake timeout
	// join: a misbehaving driver keeps ownership and Stop returns unavailable.
	_ = s.maintenance.Stop(context.Background())
	s.requests.Wait()
	s.backend.Fence()
	// Best-effort explicit deny-all. Failure is NOT reported as ACL withdrawal;
	// API remains latched closed even if old ACL survives provider outage.
	clean, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	err := s.driver.ReplaceRules(clean, nil)
	s.withdrawn.Store(err == nil && clean.Err() == nil)
	cancel()
}

// DenyAllConfirmed reports only final driver success, not reachability evidence.
// Stop success means workers joined; it does NOT imply provider ACL withdrawal.
func (s *NetworkSupervisor) DenyAllConfirmed() bool { return s.withdrawn.Load() }

func (s *NetworkSupervisor) Stop(ctx context.Context) error {
	s.mu.Lock()
	s.closed.Store(true)
	s.cancel()
	s.mu.Unlock()
	select {
	case <-s.done:
		return nil
	case <-ctx.Done():
		return ErrBroker
	}
}

func (s *NetworkSupervisor) execute(ctx context.Context, r invite.NetworkRequest) (invite.NetworkReply, error) {
	s.mu.Lock()
	if !s.Ready() {
		s.mu.Unlock()
		return invite.NetworkReply{}, ErrBroker
	}
	s.requests.Add(1)
	s.mu.Unlock()
	defer s.requests.Done()
	work, cancel := context.WithCancel(ctx)
	defer cancel()
	stop := context.AfterFunc(s.ctx, cancel)
	defer stop()
	reply, err := s.backend.ExecuteNetworkRequest(work, r)
	if err != nil {
		return invite.NetworkReply{}, err
	}
	// Recheck after every backend operation, including signing state/clock/TLS.
	// A credential from a canceled operation is never forwarded to the frontend.
	s.mu.Lock()
	defer s.mu.Unlock()
	if work.Err() != nil || !s.Ready() {
		return invite.NetworkReply{}, ErrBroker
	}
	return reply, nil
}

// Authority creates the only supervised broker path; frontend cannot supply
// verifier readiness, Store, backend, nodeproof or control-plane addresses.
func (s *NetworkSupervisor) Authority() *Authority {
	return &Authority{Store: s.backend.Store, Backend: s.backend, NetworkReady: s.Ready, supervisor: s}
}
