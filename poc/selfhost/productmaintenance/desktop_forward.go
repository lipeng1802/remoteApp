package productmaintenance

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"remoteapp.local/selfhost-poc/clientruntime"
	"remoteapp.local/selfhost-poc/installeddevice"
	"remoteapp.local/selfhost-poc/invite"
)

func (c *Controller) desktopTarget(g invite.Grant) bool {
	own := sha256.Sum256(c.signer.PublicKey())
	return hex.EncodeToString(own[:]) == g.Claims.Target
}

func (c *Controller) desktopForward(ctx context.Context, request installeddevice.Request) (json.RawMessage, error) {
	if err := c.lock(ctx); err != nil {
		return nil, err
	}
	defer c.unlock()
	if !c.policy.IsDesktop() || c.child == nil || c.revocation == nil || c.requested || c.prepared ||
		c.broken || c.closed || c.quiesced || c.released ||
		((c.provisioned || c.desktopPreprovisionPinned) && request.Op != "desktop_state" && request.Op != "desktop_arm_stop") ||
		(request.Op == "desktop_arm_stop" && (!c.provisioned || c.desktopStopArmed)) {
		return nil, ErrProduct
	}
	raw, err := c.child.Exchange(ctx, request)
	if err != nil {
		c.broken = true
		return nil, errForwardExchange
	}
	var ack struct {
		Status string `json:"status"`
	}
	if decode(raw, &ack) == nil && ack.Status == "request_rejected" {
		return nil, ErrProduct
	}
	switch request.Op {
	case "desktop_begin":
		if decode(raw, &ack) == nil && ack.Status == "desktop_pending" {
			return raw, nil
		}
	case "desktop_status":
		var p installeddevice.DesktopProgress
		if decode(raw, &p) == nil && p.Valid() && c.observeDesktopProgress(ctx, p) == nil {
			return raw, nil
		}
	case "desktop_arm_stop":
		if decode(raw, &ack) == nil && ack.Status == "desktop_stop_armed" {
			c.desktopStopArmed = true
			return raw, nil
		}
	case "desktop_state":
		if decode(raw, &ack) == nil && ack.Status == "product_unavailable" {
			return raw, nil
		}
		var s clientruntime.DesktopState
		if decode(raw, &s) == nil && s.Status == "desktop_state" &&
			s.Frames >= 0 && s.Inputs >= 0 && s.Releases >= 0 && s.Held >= 0 {
			return json.Marshal(s)
		}
	}
	c.broken = true
	return nil, ErrProduct
}

func (c *Controller) observeDesktopProgress(ctx context.Context, p installeddevice.DesktopProgress) error {
	if c.desktopRequest != "" && c.desktopRequest != p.RequestID {
		return ErrProduct
	}
	if p.RequestID != "" {
		if err := c.revocation.ObserveRequest(ctx, p.RequestID); err != nil {
			return err
		}
		c.desktopRequest = p.RequestID
	}
	if p.Grant == nil {
		return nil
	}
	if p.Grant.Claims.Permission != c.policy.Permission() {
		return ErrProduct
	}
	if err := c.revocation.ObserveContext(ctx, p.RequestID, *p.Grant); err != nil {
		return err
	}
	c.desktopApproved = p.Grant
	return nil
}
