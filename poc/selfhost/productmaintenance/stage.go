package productmaintenance

import (
	"context"
	"remoteapp.local/selfhost-poc/startuptrace"
)

// ProductStage remains readable after a work fault; it cannot authorize reuse.
func (c *Controller) ProductStage(ctx context.Context) (startuptrace.Report, error) {
	if e := c.lock(ctx); e != nil {
		return startuptrace.Report{}, e
	}
	defer c.unlock()
	if c.child == nil || c.released {
		return startuptrace.Report{}, ErrProduct
	}
	return startuptrace.Report{Status: "product_stage", Stage: c.child.ProductStage()}, nil
}
