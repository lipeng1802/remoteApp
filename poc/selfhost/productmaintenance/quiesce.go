package productmaintenance

import (
	"context"
	"encoding/json"
)

// quiesce decreases authority: stop/join this pinned product, retaining the
// independent revocation signer. It is neither maintenance nor file removal.
// Any uncertain/failed stop latches failure; late exit cannot rearm success.
func (c *Controller) quiesce(ctx context.Context) (json.RawMessage, error) {
	if err := c.lock(ctx); err != nil {
		return nil, err
	}
	defer c.unlock()
	if c.child == nil || c.revocation == nil || !c.provisioned || c.requested || c.prepared || c.broken || c.closed || c.released {
		return nil, ErrProduct
	}
	if !c.quiesced {
		if err := c.child.Stop(ctx); err != nil {
			c.broken = true
			return nil, err
		}
		select {
		case <-c.child.Done():
		default:
			c.broken = true
			return nil, ErrProduct
		}
		c.quiesced = true
	}
	return json.RawMessage(`{"status":"session_quiesced","joined":true,"zero_exit":true}`), nil
}
