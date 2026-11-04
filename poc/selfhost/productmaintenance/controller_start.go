package productmaintenance

import (
	"context"
	"encoding/json"
	"remoteapp.local/selfhost-poc/installeddevice"
)

// One exact post-provision request; the product still verifies fresh authority.
// A lost/malformed receipt seals reuse but preserves the independent revoke owner.
func (c *Controller) startController(ctx context.Context, request installeddevice.Request) (json.RawMessage, error) {
	if e := c.lock(ctx); e != nil {
		return nil, e
	}
	defer c.unlock()
	if !installeddevice.ValidControllerStart(request) || c.child == nil || c.revocation == nil || !c.provisioned || !c.controllerRole || c.controllerStartAttempted || c.requested || c.prepared || c.broken || c.closed || c.released || c.quiesced {
		return nil, ErrProduct
	}
	c.controllerStartAttempted = true
	raw, e := c.child.Exchange(ctx, request)
	var ack struct {
		Status string `json:"status"`
		Passed bool   `json:"passed"`
	}
	if e != nil || decode(raw, &ack) != nil || ack.Status != "controller_started" || !ack.Passed {
		c.broken = true
		return nil, ErrProduct
	}
	return raw, nil
}
