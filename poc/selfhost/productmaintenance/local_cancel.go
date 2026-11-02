package productmaintenance

import (
	"context"
	"errors"
)

var ErrLocalCancelJoined = errors.New("local_cancel_joined")

// CancelLocalFault is a trusted-parent-only decrease of authority. It neither
// queries nor proves remote revocation and must never produce its receipt.
// This is not exposed through business requests or network endpoints.
// Only a product's nested fault-cleanup witness and actual process join can
// release the independent signer. Unknown exits retain ownership.
func (c *Controller) CancelLocalFault(ctx context.Context) error {
	if err := c.lock(ctx); err != nil {
		return err
	}
	defer c.unlock()
	if c.localCancelJoined {
		return nil
	}
	if c.child == nil || c.signer == nil || c.requested || c.prepared || c.released || c.faultReleaseFailed {
		return ErrProduct
	}
	c.localCancelRequested = true
	c.broken = true // no late reply may rearm business or maintenance success
	_ = c.child.Stop(ctx)
	if !c.child.JoinedFault() {
		return ErrProduct
	}
	if err := c.signer.CloseJoined(); err != nil {
		c.faultReleaseFailed = true
		return err
	}
	if c.revocation != nil {
		c.revocation.Close()
	}
	c.localCancelJoined = true
	c.released = true
	return nil
}
