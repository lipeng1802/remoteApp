package clientowner

import (
	"context"

	"remoteapp.local/selfhost-poc/trustinstall"
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
	return o.maintain(ctx, action, false)
}

// MaintainDurable commits a native pending intent BEFORE cancel/join, fencing
// a crash during join as well as a crash during subsequent maintenance action.
// Fresh owners and offline upgrades fail closed until explicit recovery exists.
func (o *Owner) MaintainDurable(ctx context.Context, action MaintenanceAction) error {
	return o.maintain(ctx, action, true)
}

// MaintainDurableState is a trusted in-process handoff. The immutable compiled
// release must be the owner's own release. The callback runs only after actual
// join, under the same lease/serial lock, with the exact committed transaction.
// The snapshot is an expectation for lease reacquisition, not deletion authority.
func (o *Owner) MaintainDurableState(ctx context.Context, r trustinstall.Release, action func(context.Context, trustprofile.MaintenanceState) error) error {
	if o == nil || r != o.release || action == nil {
		return trustprofile.ErrTrust
	}
	return o.MaintainDurable(ctx, func(life context.Context, epoch uint64) error {
		state, err := trustprofile.ReadMaintenance(o.release.Anchor(), o.journal, o.clock)
		if err != nil || state.State != "pending" || state.Epoch != epoch || state.Transaction != o.maintenanceTransaction {
			return trustprofile.ErrTrust
		}
		return action(life, state)
	})
}

func (o *Owner) maintain(ctx context.Context, action MaintenanceAction, durable bool) error {
	if err := o.lock(ctx); err != nil {
		return err
	}
	defer o.unlock()
	if action == nil || o.closed || o.maintenanceAttempted || (!o.maintenanceRequested && !o.healthy()) {
		return trustprofile.ErrTrust
	}
	if o.maintenanceRequested && o.maintenanceDurable != durable {
		return trustprofile.ErrTrust
	}
	if !o.maintenanceRequested {
		o.maintenanceRequested = true
		o.maintenanceDurable = durable
		if durable {
			// An uncertain intent commit cannot be retried on this owner. No
			// cancellation or external action is acknowledged in that case.
			o.maintenanceAttempted = true
			profile, _, err := o.installed.Current()
			if err != nil {
				o.broken = true
				return err
			}
			if err = o.installed.BeginMaintenance(); err != nil {
				o.broken = true
				return err
			}
			state, err := trustprofile.ReadMaintenance(o.release.Anchor(), o.journal, o.clock)
			if err != nil || state.Epoch != profile.Claims.Epoch {
				o.broken = true
				return trustprofile.ErrTrust
			}
			o.maintenanceEpoch = state.Epoch
			o.maintenanceTransaction = state.Transaction
			o.maintenanceAttempted = false // intent confirmed; only join may resume
		}
	}
	if err := o.join(ctx); err != nil {
		o.broken = true
		return err
	}
	epoch := o.maintenanceEpoch
	if durable {
		state, err := trustprofile.ReadMaintenance(o.release.Anchor(), o.journal, o.clock)
		if err != nil || state.Epoch != epoch || state.Transaction != o.maintenanceTransaction {
			o.broken = true
			return trustprofile.ErrTrust
		}
	} else {
		profile, _, err := o.installed.Current()
		if err != nil {
			o.broken = true
			return err
		}
		epoch = profile.Claims.Epoch
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	o.maintenanceAttempted = true
	if err := action(ctx, epoch); err != nil {
		o.broken = true
		return err
	}
	if err := ctx.Err(); err != nil {
		o.broken = true
		return err
	}
	return nil
}
