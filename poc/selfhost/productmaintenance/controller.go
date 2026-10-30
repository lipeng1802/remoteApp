// Package productmaintenance supervises TEST maintenance outside the product
// process. A signed closed frame is only possible after actual product exit.
package productmaintenance

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"

	"remoteapp.local/selfhost-poc/autonomousbinding"
	"remoteapp.local/selfhost-poc/clientruntime"
	"remoteapp.local/selfhost-poc/grantrevocation"
	"remoteapp.local/selfhost-poc/installeddevice"
	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/localmaintenance"
	"remoteapp.local/selfhost-poc/processowner"
	"remoteapp.local/selfhost-poc/trustinstall"
	"remoteapp.local/selfhost-poc/trustprofile"
)

var ErrProduct = errors.New("product_maintenance_denied")

type Signer interface {
	localmaintenance.Signer
	CloseJoined() error
}
type OpenSigner func(autonomousbinding.Record) (Signer, error)
type boundRevocation interface {
	Observe(string, invite.Grant) error
	Pin(invite.Grant) error
	Revoke(context.Context) error
	Close()
}

type Controller struct {
	r                                             trustinstall.Release
	trust                                         trustinstall.Factory
	records                                       autonomousbinding.JournalFactory
	binding                                       autonomousbinding.Record
	signer                                        Signer
	child                                         *processowner.Child
	serial                                        chan struct{}
	requested, prepared, broken, closed, released bool
	provisioned                                   bool
	quiesced                                      bool
	faultJoined, faultReleaseFailed               bool
	expected                                      trustprofile.MaintenanceState
	revocation                                    boundRevocation
}

// ConfigureRevocation is trusted compiled wiring, never a pipe-supplied switch.
func (c *Controller) ConfigureRevocation(ctx context.Context, tunnel bool) error {
	if e := c.lock(ctx); e != nil {
		return e
	}
	defer c.unlock()
	if c.revocation != nil || c.child == nil || c.requested || c.broken || c.provisioned || c.released {
		return ErrProduct
	}
	r, e := grantrevocation.New(c.r.Specification(), c.signer, tunnel)
	if e != nil {
		return e
	}
	c.revocation = r
	return nil
}

func (c *Controller) revoke(ctx context.Context) (json.RawMessage, error) {
	if e := c.lock(ctx); e != nil {
		return nil, e
	}
	defer c.unlock()
	// Product/watch failure may already have sealed business IPC; retain the
	// independent signer for this exact decrease-of-authority operation.
	if c.revocation == nil || !c.provisioned || c.released {
		return nil, ErrProduct
	}
	if e := c.revocation.Revoke(ctx); e != nil {
		return nil, e
	}
	return json.RawMessage(`{"status":"session_revoked","passed":true}`), nil
}

// StartTest reads identity ONLY from the existing native binding and closes
// that inspection lease before starting the hash-pinned product. It never
// initializes identities/stores. Failure can return a partial Controller;
// callers must Release and join it, not discard ownership on a deadline.
func StartTest(ctx context.Context, r trustinstall.Release, spec processowner.Spec, trust trustinstall.Factory, records autonomousbinding.JournalFactory, open OpenSigner) (*Controller, error) {
	if ctx == nil || ctx.Err() != nil || !r.Specification().Test || trust == nil || records == nil || open == nil {
		return nil, ErrProduct
	}
	b, e := autonomousbinding.OpenWithJournal(r.Scope(), true, records)
	if e != nil {
		return nil, e
	}
	if e = b.Check(); e != nil {
		b.Close()
		return nil, e
	}
	record := b.Record()
	if e = b.Close(); e != nil {
		return nil, e
	}
	c := &Controller{r: r, trust: trust, records: records, binding: record, serial: make(chan struct{}, 1)}
	c.signer, e = open(record)
	if e != nil || c.signer == nil {
		return c, ErrProduct
	}
	public, e := hex.DecodeString(record.PublicKey)
	if e != nil || !bytes.Equal(public, c.signer.PublicKey()) {
		return c, ErrProduct
	}
	c.child, e = processowner.Start(spec)
	if e != nil {
		return c, e
	}
	ready, e := c.child.Exchange(ctx, map[string]string{"Op": "open"})
	var initial struct {
		Status string `json:"status"`
	}
	if e != nil || decode(ready, &initial) != nil || initial.Status != "ready" {
		c.broken = true
		return c, ErrProduct
	}
	return c, nil
}

func decode(raw []byte, target any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(target) != nil || d.Decode(new(any)) != io.EOF {
		return ErrProduct
	}
	return nil
}
func (c *Controller) lock(ctx context.Context) error {
	if ctx == nil || ctx.Err() != nil {
		return ErrProduct
	}
	select {
	case c.serial <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (c *Controller) unlock()                            { <-c.serial }
func (c *Controller) Signer() localmaintenance.Signer    { return c.signer }
func (c *Controller) Identity() autonomousbinding.Record { return c.binding }
func (c *Controller) Exited() <-chan struct{} {
	if c.child == nil {
		return nil
	}
	return c.child.Exited()
}

// Forward carries trusted LOCAL business commands/replies, which may contain
// invitation/grant material: never log them. It serializes with maintenance.
// Successful provision seals this path because the autonomous product owns all
// subsequent watch/session work; ordinary requests cannot interrupt that owner.
func (c *Controller) Forward(ctx context.Context, request installeddevice.Request) (json.RawMessage, error) {
	if installeddevice.ValidSupervisedQuiesce(request) {
		return c.quiesce(ctx)
	}
	if installeddevice.ValidSupervisedRevocation(request) {
		return c.revoke(ctx)
	}
	if !installeddevice.ValidSupervisedCommand(request) {
		return nil, ErrProduct
	}
	if e := c.lock(ctx); e != nil {
		return nil, e
	}
	defer c.unlock()
	if c.child == nil || c.requested || c.provisioned || c.broken || c.closed || c.released {
		return nil, ErrProduct
	}
	if request.Op == "provision" && c.revocation != nil {
		if e := c.revocation.Pin(request.Network.Grant); e != nil {
			return nil, e
		}
		c.provisioned = true // retain revoke ability if the product reply is lost
	}
	raw, e := c.child.Exchange(ctx, request)
	if e != nil {
		c.broken = true
		return nil, e
	}
	if request.Op == "provision" {
		// Product enrollment is single-use even on failure. Do not resend or
		// treat a late/malformed reply as an ordinary usable device result.
		c.provisioned = true
		var result struct {
			Status string `json:"status"`
			Passed bool   `json:"passed"`
			Active bool   `json:"active"`
		}
		if decode(raw, &result) != nil || result.Status != "network_watch" || !result.Passed || !result.Active {
			c.broken = true
			return nil, ErrProduct
		}
	}
	if c.revocation != nil && request.Op == "device" && (request.Device.Action == "approve" || request.Device.Action == "status") {
		var result struct {
			Status string        `json:"status"`
			Passed bool          `json:"passed"`
			Device invite.Result `json:"device"`
		}
		if decode(raw, &result) == nil && result.Status == "device_result" && result.Passed && result.Device.Status == "approved" && result.Device.Grant != nil {
			if result.Device.RequestID != request.Device.RequestID || c.revocation.Observe(result.Device.RequestID, *result.Device.Grant) != nil {
				c.broken = true
				return nil, ErrProduct
			}
		}
	}
	return raw, nil
}

// Count queries read-only TEST telemetry, including after provision. An absent
// payload is not evidence of an active connection and does not retire the owner.
func (c *Controller) Count(ctx context.Context) (clientruntime.ProductCount, error) {
	if e := c.lock(ctx); e != nil {
		return clientruntime.ProductCount{}, e
	}
	defer c.unlock()
	if c.child == nil || c.requested || c.broken || c.closed || c.released || c.quiesced {
		return clientruntime.ProductCount{}, ErrProduct
	}
	raw, e := c.child.Exchange(ctx, map[string]string{"Op": "product_count"})
	if e != nil {
		c.broken = true
		return clientruntime.ProductCount{}, e
	}
	var missing struct {
		Status string `json:"status"`
	}
	if decode(raw, &missing) == nil && missing.Status == "product_unavailable" {
		return clientruntime.ProductCount{}, ErrProduct
	}
	var count clientruntime.ProductCount
	if decode(raw, &count) != nil || count.Status != "product_count" || count.Pongs < 0 || count.PreauthRejected < 0 || count.SessionsClosed < 0 {
		c.broken = true
		return clientruntime.ProductCount{}, ErrProduct
	}
	return count, nil
}

func (c *Controller) MaintainDurableState(ctx context.Context, r trustinstall.Release, action func(context.Context, trustprofile.MaintenanceState) error) error {
	if e := c.lock(ctx); e != nil {
		return e
	}
	defer c.unlock()
	if c.child == nil || r != c.r || action == nil || c.requested || c.broken || c.closed || c.released || c.quiesced {
		return ErrProduct
	}
	c.requested = true
	raw, e := c.child.Exchange(ctx, map[string]string{"Op": "maintenance_prepare"})
	var ready struct {
		Status      string `json:"status"`
		Epoch       uint64 `json:"epoch"`
		Joined      bool   `json:"joined"`
		Transaction string `json:"transaction"`
	}
	if e != nil || decode(raw, &ready) != nil || ready.Status != "maintenance_ready" || !ready.Joined || ready.Epoch != r.Specification().Bundled.Claims.Epoch || len(ready.Transaction) != 32 || strings.Trim(ready.Transaction, "0123456789abcdef") != "" {
		c.broken = true
		return ErrProduct
	}
	c.expected = trustprofile.MaintenanceState{State: "pending", Epoch: ready.Epoch, Transaction: ready.Transaction}
	if e = action(ctx, c.expected); e != nil {
		c.broken = true
		return e
	}
	if ctx.Err() != nil {
		c.broken = true
		return ctx.Err()
	}
	c.prepared = true // product still holds native leases, awaiting finish
	return nil
}

// Close is the authenticated success path. It never treats a JSON closed reply
// as process completion: FinishMaintenance waits zero exit + EOF + reader join.
// Timeout/fault latches failure. Explicit Release can later join, not grant.
func (c *Controller) Close(ctx context.Context) error {
	if e := c.lock(ctx); e != nil {
		return e
	}
	defer c.unlock()
	if c.broken || c.released || !c.prepared {
		return ErrProduct
	}
	if c.closed {
		return nil
	}
	if e := c.child.FinishMaintenance(ctx); e != nil {
		c.broken = true
		return e
	}
	if e := c.checkReleased(); e != nil {
		c.broken = true
		return e
	}
	c.closed = true
	return nil
}

func (c *Controller) checkReleased() (result error) {
	j, e := c.trust(false)
	if e != nil {
		return e
	}
	defer func() {
		if e := j.Close(); result == nil {
			result = e
		}
	}()
	state, e := trustprofile.ReadMaintenance(c.r.Anchor(), j, time.Now)
	if e != nil || state != c.expected {
		return ErrProduct
	}
	b, e := autonomousbinding.OpenWithJournal(c.r.Scope(), true, c.records)
	if e != nil {
		return e
	}
	if b.Check() != nil || b.Record() != c.binding {
		b.Close()
		return ErrProduct
	}
	return b.Close()
}

// Release is explicit supervisor teardown, also on failure. It never force
// kills. A deadline retains child+signer ownership; retry joins the same child.
// Signer closes only after child Done (including failed exit) and Serve joined.
func (c *Controller) Release(ctx context.Context) error {
	if e := c.lock(ctx); e != nil {
		return e
	}
	defer c.unlock()
	if c.released {
		return nil
	}
	var result error
	if c.child != nil && !c.closed && !c.quiesced {
		result = c.child.Stop(ctx)
		select {
		case <-c.child.Done():
		default:
			return result
		}
	}
	if c.signer != nil {
		if c.revocation != nil {
			c.revocation.Close()
		}
		if e := c.signer.CloseJoined(); result == nil {
			result = e
		}
	}
	c.released = true
	return result
}

var _ localmaintenance.Owner = (*Controller)(nil)
