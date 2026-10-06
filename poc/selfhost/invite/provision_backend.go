package invite

import (
	"context"
	"sync"
	"time"
)

type RegistrationCredential struct{ UserID, KeyID, Secret string }

// Driver is privileged. Mint is single-use, expires at record.Expires, isolated
// under ticket ID. Cleanup MUST discover orphan resources by that ID even when
// Mint had an unknown outcome, and MUST NOT delete unrelated users/nodes/keys.
// Verify checks live network-key possession, not only a control-plane row.
// ReplaceRules must atomically replace ONLY this backend's managed policy.
// A zero-rule projection means explicit deny-all, NEVER omitted/default policy.
// All calls must honor ctx. The headscale package supplies the server-local CLI
// adapter; it still requires a trusted live proof implementation for Bind.
type ProvisionDriver interface {
	Mint(context.Context, ProvisionRecord) (RegistrationCredential, error)
	Observe(context.Context, ProvisionRecord) (ProvisionObservation, error)
	Verify(context.Context, ProvisionRecord, ProvisionObservation) bool
	Cleanup(context.Context, ProvisionRecord) error
	ReplaceRules(context.Context, []NetworkRule) error
}
type ProvisionBackend struct {
	Store  *Store
	Driver ProvisionDriver
	mu     sync.Mutex
	ready  bool
}

func bounded(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, 5*time.Second)
}
func (b *ProvisionBackend) Enroll(ctx context.Context, g Grant, e ProvisionEnvelope) (string, RegistrationCredential, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.Store == nil || b.Driver == nil || !b.ready {
		return "", RegistrationCredential{}, ErrDenied
	}
	b.Store.provisionOps.Lock()
	defer b.Store.provisionOps.Unlock()
	if !b.Store.provisionReady {
		return "", RegistrationCredential{}, ErrDenied
	}
	ctx, cancel := bounded(ctx)
	defer cancel()
	r, err := b.Store.PlanProvision(g, e)
	if err != nil {
		return "", RegistrationCredential{}, err
	}
	c, err := b.Driver.Mint(ctx, r)
	if err != nil || ctx.Err() != nil || len(c.Secret) == 0 || len(c.Secret) > 256 {
		b.ready = false
		b.Store.provisionReady = false
		if err := b.Store.ProvisionCleanup(r.ID, false); err != nil {
			return r.ID, RegistrationCredential{}, err
		}
		return r.ID, RegistrationCredential{}, ErrDenied
	}
	if err = b.Store.ProvisionIssued(r.ID, c.UserID, c.KeyID); err != nil {
		// Intent was already durable; best-effort scoped cleanup even if the
		// completion commit poisoned the store. Startup must reconcile again.
		b.ready = false
		b.Store.provisionReady = false
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		b.Driver.Cleanup(cleanupCtx, r)
		cleanupCancel()
		return r.ID, RegistrationCredential{}, err
	}
	return r.ID, c, nil // secret returned once, never saved or logged
}
func (b *ProvisionBackend) Bind(ctx context.Context, id string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.Store == nil || b.Driver == nil || !b.ready {
		return ErrDenied
	}
	b.Store.provisionOps.Lock()
	defer b.Store.provisionOps.Unlock()
	if !b.Store.provisionReady {
		return ErrDenied
	}
	ctx, cancel := bounded(ctx)
	defer cancel()
	var r ProvisionRecord
	for _, v := range b.Store.ProvisionRecords() {
		if v.ID == id {
			r = v
		}
	}
	if r.Status != "issued" {
		return ErrDenied
	}
	o, err := b.Driver.Observe(ctx, r)
	if err != nil || ctx.Err() != nil {
		return ErrDenied
	}
	verified := b.Driver.Verify(ctx, r, o)
	if ctx.Err() != nil {
		return ErrDenied
	}
	return b.Store.ProvisionBound(id, o, verified)
}

// Reconcile is called on startup BEFORE serving credentials, and periodically.
// Unknown Mint outcomes are never retried: cleanup by durable ticket instead.
// Expiry of a registration key does not revoke a bound active grant.
func (b *ProvisionBackend) Reconcile(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.ready = false
	if b.Store == nil || b.Driver == nil {
		return ErrDenied
	}
	b.Store.provisionOps.Lock()
	defer b.Store.provisionOps.Unlock()
	b.Store.provisionReady = false
	ctx, cancel := bounded(ctx)
	defer cancel()
	rules, err := b.Store.ProvisionRules()
	if err != nil {
		b.Driver.ReplaceRules(ctx, nil)
		return err
	}
	if err = b.Driver.ReplaceRules(ctx, rules); err != nil || ctx.Err() != nil {
		return ErrDenied
	}
	for _, r := range b.Store.ProvisionRecords() {
		if r.Status == "cleaned" {
			continue
		}
		cleanup := r.Status == "planned" || r.Status == "cleanup_pending" || !b.Store.Authorized(r.Grant, r.Grant.Claims.Target, r.Grant.Claims.Controller)
		if r.Status == "issued" && b.Store.clock().Unix() >= r.Expires {
			cleanup = true
		}
		if !cleanup {
			continue
		}
		if err = b.Store.ProvisionCleanup(r.ID, false); err != nil {
			return err
		}
		if err = b.Driver.Cleanup(ctx, r); err != nil || ctx.Err() != nil {
			return ErrDenied
		}
		if err = b.Store.ProvisionCleanup(r.ID, true); err != nil {
			return err
		}
	}
	rules, err = b.Store.ProvisionRules()
	if err != nil {
		b.Driver.ReplaceRules(ctx, nil)
		return err
	}
	if err = b.Driver.ReplaceRules(ctx, rules); err != nil || ctx.Err() != nil {
		return ErrDenied
	}
	b.ready = true
	b.Store.provisionReady = true
	return nil
}
