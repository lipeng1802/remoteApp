package clientowner

import (
	"context"

	"remoteapp.local/selfhost-poc/trustprofile"
)

// MaintenanceAction is trusted in-process code. It must honor cancellation,
// return only after its own workers join, and never reenter this Owner. It
// must not delete native identity, trust, activation or epoch-floor records.
// The epoch is read from the still-leased committed installation after join.
type MaintenanceAction func(context.Context, uint64) error

// Maintain permanently seals this Owner against new sessions and upgrades,
// joins its runner, and executes one action while retaining both the native
// journal lease and the owner serialization lock. Close cannot release the
// lease while the action is running, even if the action ignores cancellation.
//
// A join timeout permits an explicit retry to finish that same join. The action
// is marked attempted before invocation: neither an error nor an uncertain
// receipt permits a second action on this Owner. No new process is authorized
// by a standalone epoch/status reply; future uninstall IPC must bind to this
// live owner and use this barrier, not copy a receipt across processes.
func (o *Owner) Maintain(ctx context.Context, action MaintenanceAction) error {
	if err := o.lock(ctx); err != nil {
		return err
	}
	defer o.unlock()
	if action == nil || o.closed || o.maintenanceAttempted || (!o.maintenanceRequested && !o.healthy()) {
		return trustprofile.ErrTrust
	}
	o.maintenanceRequested = true
	if err := o.join(ctx); err != nil {
		o.broken = true
		return err
	}
	profile, _, err := o.installed.Current()
	if err != nil {
		o.broken = true
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	o.maintenanceAttempted = true
	if err = action(ctx, profile.Claims.Epoch); err != nil {
		o.broken = true
		return err
	}
	if err = ctx.Err(); err != nil {
		o.broken = true
		return err
	}
	return nil
}
