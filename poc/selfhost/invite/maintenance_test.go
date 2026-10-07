package invite

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func eventuallyMaintenance(t *testing.T, check func() bool) {
	t.Helper()
	end := time.Now().Add(4 * time.Second)
	for time.Now().Before(end) {
		if check() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("maintenance condition not reached")
}
func stopMaintenance(t *testing.T, m *Maintenance) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if m.Stop(ctx) != nil || !m.Status().Stopped || m.Status().Ready {
		t.Fatal("worker not joined/fenced")
	}
}

func TestMaintenancePolicyFailureRecoveryAndStopFence(t *testing.T) {
	f, g, b, d := provisionFixture(t)
	bindPair(t, f, g, b)
	m, e := b.StartMaintenance(context.Background(), 100*time.Millisecond, 400*time.Millisecond)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { stopMaintenance(t, m) })
	if _, e = b.StartMaintenance(context.Background(), 100*time.Millisecond, 400*time.Millisecond); e == nil {
		t.Fatal("duplicate worker")
	}
	eventuallyMaintenance(t, func() bool { return m.Status().Ready })
	b.mu.Lock()
	d.failPolicy = true
	b.mu.Unlock()
	eventuallyMaintenance(t, func() bool { return m.Status().Failures >= 2 })
	if n, e := b.NetworkState(g); e == nil && n.Claims.Active {
		t.Fatal("failed policy authorized state")
	}
	if _, c, e := b.Enroll(context.Background(), g, intent(f, g, f.owner, "c")); e == nil || c.Secret != "" {
		t.Fatal("failure minted credentials")
	}
	b.mu.Lock()
	d.failPolicy = false
	b.mu.Unlock()
	eventuallyMaintenance(t, func() bool { return m.Status().Ready })
	if n, e := b.NetworkState(g); e != nil || !n.Claims.Active {
		t.Fatal("recovery missing verified binding")
	}
	if m.Status().NextDelay != 100*time.Millisecond || m.Status().ConsecutiveFailures != 0 {
		t.Fatal("backoff not reset")
	}
	stopMaintenance(t, m)
	if n, e := b.NetworkState(g); e == nil && n.Claims.Active {
		t.Fatal("stopped worker left issuance ready")
	}
}

func TestMaintenanceExpiryCleanupRetryWithoutRemint(t *testing.T) {
	f, g, b, d := provisionFixture(t)
	bindPair(t, f, g, b)
	var clock atomic.Int64
	clock.Store(f.now.Unix())
	f.s.clock = func() time.Time { return time.Unix(clock.Load(), 0) }
	m, e := b.StartMaintenance(context.Background(), 100*time.Millisecond, 200*time.Millisecond)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { stopMaintenance(t, m) })
	eventuallyMaintenance(t, func() bool { return m.Status().Ready })
	b.mu.Lock()
	d.failCleanup = true
	b.mu.Unlock()
	clock.Add(301)
	eventuallyMaintenance(t, func() bool { return m.Status().Failures > 0 })
	b.mu.Lock()
	rules := len(d.rules)
	minted := d.minted
	d.failCleanup = false
	b.mu.Unlock()
	if rules != 0 || minted != 2 {
		t.Fatal("expiry did not withdraw first or retried mint")
	}
	eventuallyMaintenance(t, func() bool {
		b.mu.Lock()
		defer b.mu.Unlock()
		return len(d.resources) == 0 && b.ready
	})
	b.mu.Lock()
	defer b.mu.Unlock()
	if d.minted != 2 || len(d.rules) != 0 {
		t.Fatal("expired grant revived")
	}
}

type waitingMaintenanceDriver struct {
	*memoryDriver
	entered chan struct{}
}

func (d *waitingMaintenanceDriver) ReplaceRules(ctx context.Context, _ []NetworkRule) error {
	select {
	case d.entered <- struct{}{}:
	default:
	}
	<-ctx.Done()
	return ctx.Err()
}
func TestMaintenanceCancelBoundsActiveDriverAndColdRestart(t *testing.T) {
	f, g, b, d := provisionFixture(t)
	bindPair(t, f, g, b)
	b.Driver = &waitingMaintenanceDriver{d, make(chan struct{}, 1)}
	m, e := b.StartMaintenance(context.Background(), 100*time.Millisecond, 200*time.Millisecond)
	if e != nil {
		t.Fatal(e)
	}
	select {
	case <-b.Driver.(*waitingMaintenanceDriver).entered:
	case <-time.After(time.Second):
		t.Fatal("no startup cycle")
	}
	stopMaintenance(t, m)
	f.restart(t)
	b = &ProvisionBackend{Store: f.s, Driver: d}
	if n, e := b.NetworkState(g); e != nil || n.Claims.Active {
		t.Fatal("cold restart authorized before reconcile")
	}
	m, e = b.StartMaintenance(context.Background(), 100*time.Millisecond, 200*time.Millisecond)
	if e != nil {
		t.Fatal(e)
	}
	eventuallyMaintenance(t, func() bool { return m.Status().Ready })
	if n, e := b.NetworkState(g); e != nil || !n.Claims.Active {
		t.Fatal("startup did not recover")
	}
	stopMaintenance(t, m)
}
func TestMaintenanceInvalidConfiguration(t *testing.T) {
	_, _, b, _ := provisionFixture(t)
	for _, v := range [][2]time.Duration{{0, time.Second}, {time.Second, 0}, {time.Second, 2 * time.Minute}} {
		if _, e := b.StartMaintenance(context.Background(), v[0], v[1]); e == nil {
			t.Fatal("unsafe config accepted")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := b.StartMaintenance(ctx, time.Second, time.Second); e == nil {
		t.Fatal("cancelled start accepted")
	}
}

func TestMaintenanceUnboundRegistrationExpiresAutomatically(t *testing.T) {
	f, g, b, d := provisionFixture(t)
	enroll(t, f, g, b, f.owner, "a")
	var clock atomic.Int64
	clock.Store(f.now.Unix())
	f.s.clock = func() time.Time { return time.Unix(clock.Load(), 0) }
	m, e := b.StartMaintenance(context.Background(), 100*time.Millisecond, 200*time.Millisecond)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { stopMaintenance(t, m) })
	eventuallyMaintenance(t, func() bool { return m.Status().Ready })
	clock.Add(121)
	eventuallyMaintenance(t, func() bool { b.mu.Lock(); defer b.mu.Unlock(); return len(d.resources) == 0 && b.ready })
	if !f.s.Authorized(g, g.Claims.Target, g.Claims.Controller) {
		t.Fatal("registration expiry must not expire active grant")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if d.minted != 1 || len(d.rules) != 0 {
		t.Fatal("expired unbound credential reissued")
	}
}
