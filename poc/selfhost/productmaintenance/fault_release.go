package productmaintenance

import (
	"context"
	"errors"
)

var errFaultLock = errors.New("fault release lock unavailable")
var errFaultState = errors.New("fault release state rejected")
var errFaultRevocation = errors.New("fault release revocation unconfirmed")
var errFaultChild = errors.New("fault release child join unconfirmed")
var errFaultSigner = errors.New("fault release signer join unconfirmed")

// FaultReleaseReason exposes fixed TEST diagnostics, never underlying errors.
func FaultReleaseReason(err error) string {
	for _, item := range []struct {
		cause  error
		reason string
	}{
		{errFaultLock, "lock_unavailable"}, {errFaultState, "state_rejected"},
		{errFaultRevocation, "revocation_unconfirmed"}, {errFaultChild, "child_join_unconfirmed"},
		{errFaultSigner, "signer_join_unconfirmed"},
	} {
		if errors.Is(err, item.cause) {
			return item.reason
		}
	}
	return "unavailable"
}

var ErrFaultJoined = errors.New("product_fault_joined")

// ReleaseFault is a private TEST lifecycle action. The independent native
// signer first confirms the exact pinned request is revoked; unknown status
// cannot sacrifice that signer. Normal close and generic failed exits are not
// nested-cleanup receipts. Until a native product emits fault_joined from its
// explicit cleanup witness, real fault release remains deliberately denied.
func (c *Controller) ReleaseFault(ctx context.Context) error {
	if err := c.lock(ctx); err != nil {
		return errors.Join(err, ErrProduct, errFaultLock)
	}
	defer c.unlock()
	if c.faultJoined {
		return nil
	}
	if c.localCancelRequested || c.child == nil || c.signer == nil || c.revocation == nil || !c.provisioned || c.requested || c.prepared || c.released || c.faultReleaseFailed {
		return errors.Join(ErrProduct, errFaultState)
	}
	confirm, ok := c.revocation.(interface{ ConfirmRevoked(context.Context) error })
	if !ok || confirm.ConfirmRevoked(ctx) != nil {
		return errors.Join(ErrProduct, errFaultRevocation)
	}
	_ = c.child.Stop(ctx)
	if !c.child.JoinedFault() {
		return errors.Join(ErrProduct, errFaultChild)
	}
	c.revocation.Close()
	if err := c.signer.CloseJoined(); err != nil {
		c.faultReleaseFailed = true
		return errors.Join(err, ErrProduct, errFaultSigner)
	}
	c.faultJoined = true
	c.released = true
	return nil
}
