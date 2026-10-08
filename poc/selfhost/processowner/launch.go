package processowner

import (
	"context"
	"encoding/json"
	"time"
)

// Plan is a trusted LOCAL adapter, never an API request. Target preparation
// must publish and verify certificate possession using the signed device API;
// controller preparation must fetch/verify the fresh issuer TLS descriptor.
// Callbacks are synchronous and must obey ctx; no detached secret supplier.
type Plan struct {
	Target           bool
	Bridge, Product  Spec
	BridgeStart      any
	PrepareTarget    func(context.Context, *Child, json.RawMessage) error
	ControllerConfig func(context.Context, json.RawMessage) (any, error)
}

// Launch retains partial ownership on any failure. Caller must retain this
// object if Close times out; it must not delete state/credentials or retry
// enrollment while any started child is unjoined. Ready is private, not a log.
type Launch struct {
	Bridge, Product *Child
	Pair            *Pair
	Ready           json.RawMessage
}

func StartEndpoint(ctx context.Context, p Plan) (*Launch, error) {
	l := &Launch{}
	if ctx == nil || ctx.Err() != nil || p.BridgeStart == nil || (p.Target && p.PrepareTarget == nil) || (!p.Target && p.ControllerConfig == nil) {
		return l, ErrProcess
	}
	fail := func() (*Launch, error) {
		cleanup, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		l.Close(cleanup) // failure stays failure; timeout retains the handles
		return l, ErrProcess
	}
	if p.Target {
		var e error
		l.Product, e = Start(p.Product)
		if e != nil {
			return fail()
		}
		l.Ready, e = l.Product.Exchange(ctx, nil)
		if e != nil || !productReady(l.Ready) {
			return fail()
		}
		if p.PrepareTarget(ctx, l.Product, l.Ready) != nil {
			return fail()
		}
	}
	var e error
	l.Bridge, e = Start(p.Bridge)
	if e != nil {
		return fail()
	}
	b, e := l.Bridge.Exchange(ctx, p.BridgeStart)
	if e != nil || !bridgeReady(b, p.Target) {
		return fail()
	}
	if !p.Target {
		cfg, e := p.ControllerConfig(ctx, b)
		if e != nil || cfg == nil {
			return fail()
		}
		l.Product, e = Start(p.Product)
		if e != nil {
			return fail()
		}
		l.Ready, e = l.Product.Exchange(ctx, cfg)
		if e != nil || !productReady(l.Ready) {
			return fail()
		}
	}
	l.Pair, e = NewPair(l.Bridge, l.Product)
	if e != nil {
		return fail()
	}
	return l, nil
}
func productReady(raw json.RawMessage) bool {
	var v struct {
		Status string `json:"status"`
	}
	return json.Unmarshal(raw, &v) == nil && v.Status == "ready"
}
func bridgeReady(raw json.RawMessage, target bool) bool {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || len(fields) != 2 {
		return false
	}
	var v struct {
		Status string `json:"status"`
		Port   int    `json:"port"`
	}
	if json.Unmarshal(raw, &v) != nil || v.Status != "ready" {
		return false
	}
	if target {
		return v.Port == 0
	}
	return v.Port > 0 && v.Port < 65536
}
func (l *Launch) Close(ctx context.Context) error {
	if ctx == nil {
		return ErrProcess
	}
	if l.Pair != nil {
		return l.Pair.Close(ctx)
	}
	// Partial startup also stops both concurrently; no short-circuit on error.
	errs := make(chan error, 2)
	n := 0
	for _, c := range []*Child{l.Bridge, l.Product} {
		if c != nil {
			n++
			go func(c *Child) { errs <- c.Stop(ctx) }(c)
		}
	}
	var result error
	for i := 0; i < n; i++ {
		if e := <-errs; e != nil {
			result = e
		}
	}
	return result
}
