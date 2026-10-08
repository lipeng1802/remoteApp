package clientruntime

import (
	"bytes"
	"context"
	"sync"
	"time"

	"remoteapp.local/selfhost-poc/privatepipe"
	"remoteapp.local/selfhost-poc/processowner"
	"remoteapp.local/selfhost-poc/session"
)

// ProductObservation queries the already pinned, owner-managed product. It
// cannot start, replace, stop or release that child or its installation lease.
// A zero/unavailable count is never evidence that TLS authenticated successfully.
type ProductObservation struct {
	mu     sync.Mutex
	child  *processowner.Child
	server bool
}

// ProductCount deliberately contains no HMAC, certificate, executable or keys.
type ProductCount struct {
	Status          string `json:"status"`
	Pongs           int64  `json:"pongs"`
	PreauthRejected int64  `json:"preauth_rejected,omitempty"`
	SessionsClosed  int64  `json:"sessions_closed,omitempty"`
	Ended           bool   `json:"ended"`
	Result          string `json:"result,omitempty"`
	PinRejected     bool   `json:"pin_rejected"`
}

func (o *ProductObservation) bind(child *processowner.Child, server bool) {
	if o == nil {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	// An observer is single-session. A subsequent runtime needs a new observer.
	if o.child == nil {
		o.child, o.server = child, server
	}
}

func (o *ProductObservation) Count(ctx context.Context) (ProductCount, error) {
	if o == nil {
		return ProductCount{}, session.ErrDenied
	}
	o.mu.Lock()
	child, server := o.child, o.server
	o.mu.Unlock()
	if child == nil {
		return ProductCount{}, session.ErrDenied
	}
	select {
	case <-child.Done():
		return ProductCount{}, session.ErrDenied
	default:
	}
	limit, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	raw, e := child.Exchange(limit, map[string]string{"Op": "count"})
	if e != nil {
		return ProductCount{}, e
	}
	return decodeProductCount(raw, server)
}

func decodeProductCount(raw []byte, server bool) (ProductCount, error) {
	// Separate role schemas prevent a controller reply from claiming target
	// evidence, and reject secret/unknown fields rather than forwarding them.
	decode := func(v any) error {
		return privatepipe.NewReader(bytes.NewReader(append(bytes.TrimSpace(raw), '\n'))).Read(v)
	}
	var c ProductCount
	if server {
		var r struct {
			Status          string `json:"status"`
			Pongs           *int64 `json:"pongs"`
			PreauthRejected *int64 `json:"preauth_rejected"`
			SessionsClosed  *int64 `json:"sessions_closed"`
		}
		if decode(&r) != nil || r.Status != "count" || r.Pongs == nil || r.PreauthRejected == nil || r.SessionsClosed == nil {
			return c, session.ErrDenied
		}
		c = ProductCount{Status: "product_count", Pongs: *r.Pongs, PreauthRejected: *r.PreauthRejected, SessionsClosed: *r.SessionsClosed}
	} else {
		var r struct {
			Status      string `json:"status"`
			Pongs       *int64 `json:"pongs"`
			Ended       *bool  `json:"ended"`
			Result      string `json:"result"`
			PinRejected *bool  `json:"pin_rejected"`
		}
		if decode(&r) != nil || r.Status != "count" || r.Pongs == nil || r.Ended == nil || r.PinRejected == nil {
			return c, session.ErrDenied
		}
		switch r.Result {
		case "pending", "success", "authentication_rejected", "timeout", "cancelled", "connection_closed":
		default:
			return c, session.ErrDenied
		}
		c = ProductCount{Status: "product_count", Pongs: *r.Pongs, Ended: *r.Ended, Result: r.Result, PinRejected: *r.PinRejected}
	}
	if c.Pongs < 0 || c.PreauthRejected < 0 || c.SessionsClosed < 0 {
		return ProductCount{}, session.ErrDenied
	}
	return c, nil
}
