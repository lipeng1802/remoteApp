package productmaintenance

import "context"

// RetainAfterMaintenance reports whether a successfully maintained product had
// a pinned live grant. Its independent revoke owner must outlive the payload
// and authenticated maintenance endpoint; no unsolicited lifecycle frame is due.
func (c *Controller) RetainAfterMaintenance(ctx context.Context) bool {
	if c.lock(ctx) != nil {
		return false
	}
	defer c.unlock()
	return c.closed && c.prepared && c.provisioned && c.revocation != nil && !c.broken && !c.released
}

// AllowRetainedStop only confirms the original pinned request. It never revokes
// implicitly or treats missing/expired authority as permission to release the
// last signer. Other (idle/ordinary quiesce) teardown retains its old behavior.
func (c *Controller) AllowRetainedStop(ctx context.Context) error {
	if e := c.lock(ctx); e != nil {
		return e
	}
	defer c.unlock()
	return c.confirmMaintainedRevocation(ctx)
}
func (c *Controller) confirmMaintainedRevocation(ctx context.Context) error {
	if !c.closed || !c.provisioned || c.released {
		return nil
	}
	proof, ok := c.revocation.(interface{ ConfirmRevoked(context.Context) error })
	if !ok || proof.ConfirmRevoked(ctx) != nil {
		return ErrProduct
	}
	return nil
}
