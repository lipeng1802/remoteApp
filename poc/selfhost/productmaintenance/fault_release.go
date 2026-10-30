package productmaintenance

import (
	"context"
	"errors"
)

var ErrFaultJoined = errors.New("product_fault_joined")

// ReleaseFault is a private TEST lifecycle action. The independent native
// signer first confirms the exact pinned request is revoked; unknown status
// cannot sacrifice that signer. Normal close and generic failed exits are not
// nested-cleanup receipts. Until a native product emits fault_joined from its
// explicit cleanup witness, real fault release remains deliberately denied.
func (c *Controller) ReleaseFault(ctx context.Context) error {
	if err := c.lock(ctx); err != nil {
		return err
	}
	defer c.unlock()
	if c.faultJoined {
		return nil
	}
	if c.child == nil || c.signer == nil || c.revocation == nil || !c.provisioned || c.requested || c.prepared || c.released || c.faultReleaseFailed {
		return ErrProduct
	}
	confirm, ok := c.revocation.(interface{ ConfirmRevoked(context.Context) error })
	if !ok || confirm.ConfirmRevoked(ctx) != nil {
		return ErrProduct
	}
	_ = c.child.Stop(ctx)
	if !c.child.JoinedFault() {
		return ErrProduct
	}
	c.revocation.Close()
	if err := c.signer.CloseJoined(); err != nil {
		c.faultReleaseFailed = true
		return err
	}
	c.faultJoined = true
	c.released = true
	return nil
}
