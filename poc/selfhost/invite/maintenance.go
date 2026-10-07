package invite

import (
	"context"
	"sync"
	"time"
)

// Maintenance is one serial worker per backend. The caller must stop/join it
// before closing/replacing Store or Driver. No provider output or secrets are
// exposed through status. A successful cycle means the full Reconcile passed.
type Maintenance struct {
	cancel context.CancelFunc
	done   chan struct{}
	mu     sync.Mutex
	status MaintenanceStatus
}
type MaintenanceStatus struct {
	Cycles, Failures, ConsecutiveFailures uint64
	Ready, Stopped                        bool
	NextDelay                             time.Duration
}

func (m *Maintenance) Status() MaintenanceStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.status
}
func (m *Maintenance) Stop(ctx context.Context) error {
	m.cancel()
	select {
	case <-m.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Startup fences issuance synchronously; the first cycle runs immediately.
// Retries never mint credentials: only reconcile durable intents/policy. Work
// does not overlap, and delays start after completion (not ticker catch-up).
func (b *ProvisionBackend) StartMaintenance(ctx context.Context, interval, maxBackoff time.Duration) (*Maintenance, error) {
	if ctx.Err() != nil || interval < 100*time.Millisecond || interval > time.Minute || maxBackoff < interval || maxBackoff > time.Minute || !b.maintenance.CompareAndSwap(false, true) {
		return nil, ErrDenied
	}
	b.fence()
	run, cancel := context.WithCancel(ctx)
	m := &Maintenance{cancel: cancel, done: make(chan struct{})}
	go func() {
		defer close(m.done)
		defer b.maintenance.Store(false)
		defer func() {
			b.fence()
			m.mu.Lock()
			m.status.Ready = false
			m.status.Stopped = true
			m.mu.Unlock()
		}()
		delay := interval
		for run.Err() == nil {
			err := b.Reconcile(run)
			m.mu.Lock()
			m.status.Cycles++
			m.status.Ready = err == nil && run.Err() == nil
			if err != nil {
				m.status.Failures++
				m.status.ConsecutiveFailures++
			} else {
				m.status.ConsecutiveFailures = 0
				delay = interval
			}
			m.status.NextDelay = delay
			m.mu.Unlock()
			timer := time.NewTimer(delay)
			select {
			case <-run.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
			if err != nil {
				delay *= 2
				if delay > maxBackoff {
					delay = maxBackoff
				}
			}
		}
	}()
	return m, nil
}

// Stop/failure must not leave any wrapper of the Store able to issue active
// state. This does not claim to withdraw ACL when the driver is unavailable;
// existing signed states retain only their original absolute lease.
func (b *ProvisionBackend) fence() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.ready = false
	if b.Store != nil {
		b.Store.provisionOps.Lock()
		b.Store.provisionReady = false
		b.Store.provisionOps.Unlock()
	}
}

// Fence blocks all wrappers sharing this Store. Caller must first cancel and
// join maintenance; otherwise a later successful Reconcile could reopen it.
// It does not claim an unavailable provider has withdrawn existing ACLs.
func (b *ProvisionBackend) Fence() { b.fence() }
