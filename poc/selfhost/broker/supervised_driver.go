package broker

import (
	"context"
	"remoteapp.local/selfhost-poc/invite"
	"sync/atomic"
)

// Latch proof/policy health immediately after the privileged call returns.
// Healthy never takes the underlying driver's operation mutex, so a long
// Verify cannot block supervisor observation of verifier death/cancellation.
type supervisedDriver struct {
	inner invite.ProvisionDriver
	ctx   context.Context
	fault atomic.Bool
}

func (d *supervisedDriver) Healthy() bool { return !d.fault.Load() && d.ctx.Err() == nil }
func (d *supervisedDriver) check() {
	if h, ok := d.inner.(interface{ Healthy() bool }); ok && !h.Healthy() {
		d.fault.Store(true)
	}
}
func (d *supervisedDriver) work(ctx context.Context) (context.Context, func()) {
	c, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(d.ctx, cancel)
	if !d.Healthy() {
		cancel()
	}
	return c, func() { stop(); cancel() }
}
func (d *supervisedDriver) Mint(ctx context.Context, r invite.ProvisionRecord) (invite.RegistrationCredential, error) {
	c, end := d.work(ctx)
	defer end()
	if c.Err() != nil {
		return invite.RegistrationCredential{}, invite.ErrDenied
	}
	v, e := d.inner.Mint(c, r)
	d.check()
	if e != nil || c.Err() != nil || !d.Healthy() {
		return invite.RegistrationCredential{}, invite.ErrDenied
	}
	return v, nil
}
func (d *supervisedDriver) Observe(ctx context.Context, r invite.ProvisionRecord) (invite.ProvisionObservation, error) {
	c, end := d.work(ctx)
	defer end()
	if c.Err() != nil {
		return invite.ProvisionObservation{}, invite.ErrDenied
	}
	v, e := d.inner.Observe(c, r)
	d.check()
	if e != nil || c.Err() != nil || !d.Healthy() {
		return invite.ProvisionObservation{}, invite.ErrDenied
	}
	return v, nil
}
func (d *supervisedDriver) Verify(ctx context.Context, r invite.ProvisionRecord, o invite.ProvisionObservation) bool {
	c, end := d.work(ctx)
	defer end()
	if c.Err() != nil {
		return false
	}
	v := d.inner.Verify(c, r, o)
	d.check()
	return v && c.Err() == nil && d.Healthy()
}
func (d *supervisedDriver) Cleanup(ctx context.Context, r invite.ProvisionRecord) error {
	c, end := d.work(ctx)
	defer end()
	if c.Err() != nil {
		return invite.ErrDenied
	}
	e := d.inner.Cleanup(c, r)
	d.check()
	if e != nil || c.Err() != nil {
		return invite.ErrDenied
	}
	return nil
}
func (d *supervisedDriver) ReplaceRules(ctx context.Context, r []invite.NetworkRule) error {
	c, end := d.work(ctx)
	defer end()
	if c.Err() != nil {
		return invite.ErrDenied
	}
	e := d.inner.ReplaceRules(c, r)
	d.check()
	if e != nil || c.Err() != nil || !d.Healthy() {
		d.fault.Store(true)
		return invite.ErrDenied
	}
	return nil
}
