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
		return errors.Join(err, ErrProduct, errFaultLock)
	}
	defer c.unlock()
	if c.localCancelJoined {
		return nil
	}
	if c.child == nil || c.signer == nil || c.requested || c.prepared || c.released || c.faultReleaseFailed {
		return errors.Join(ErrProduct, errFaultState)
	}
	c.localCancelRequested = true
	c.broken = true // no late reply may rearm business or maintenance success
	_ = c.child.StopFault(ctx)
	if !c.child.JoinedFault() {
		return errors.Join(ErrProduct, errFaultChild, faultChildEvidence(c.child.FaultJoinFailure()))
	}
	if err := c.signer.CloseJoined(); err != nil {
		c.faultReleaseFailed = true
		return errors.Join(err, ErrProduct, errFaultSigner)
	}
	if c.revocation != nil {
		c.revocation.Close()
	}
	c.localCancelJoined = true
	c.released = true
	return nil
}
