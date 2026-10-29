// Package grantrevocation is a TEST-only owner-side revocation channel. It
// cannot register, approve or choose a grant supplied by a revoke request.
package grantrevocation

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/trustinstall"
	"remoteapp.local/selfhost-poc/trustprofile"
	"sync"
	"time"
)

var ErrDenied = errors.New("bound_revocation_unconfirmed")

type API interface {
	DeviceRequest(context.Context, invite.Body) (invite.Result, error)
	Close()
}
type Channel struct {
	mu                                   sync.Mutex
	api                                  API
	issuer                               ed25519.PublicKey
	subject                              string
	now                                  func() time.Time
	issued, expires                      int64
	grant                                invite.Grant
	request                              string
	pinned, attempted, confirmed, closed bool
}

func New(s trustinstall.Specification, signer invite.DeviceSigner, tunnel bool) (*Channel, error) {
	if !s.Test || signer == nil {
		return nil, ErrDenied
	}
	root, e := hex.DecodeString(s.Root)
	if e != nil {
		return nil, ErrDenied
	}
	create := invite.NewProfileNetworkClient
	if tunnel {
		create = invite.NewTestTunnelNetworkClient
	}
	client, e := create(s.Bundled, root, s.Service, s.MinimumEpoch, signer)
	if e != nil {
		return nil, e
	}
	c, e := newChannel(s, signer.PublicKey(), client, time.Now)
	if e != nil {
		client.Close()
	}
	return c, e
}

func newChannel(s trustinstall.Specification, public ed25519.PublicKey, api API, clock func() time.Time) (*Channel, error) {
	root, e := hex.DecodeString(s.Root)
	if e != nil || !s.Test || len(public) != 32 || api == nil || clock == nil {
		return nil, ErrDenied
	}
	if _, e = trustprofile.Verify(s.Bundled, root, s.Service, s.MinimumEpoch, clock()); e != nil {
		return nil, e
	}
	issuer, e := hex.DecodeString(s.Bundled.Claims.Issuer)
	if e != nil {
		return nil, e
	}
	id := sha256.Sum256(public)
	return &Channel{api: api, issuer: issuer, subject: hex.EncodeToString(id[:]), now: clock, issued: s.Bundled.Claims.IssuedAt, expires: s.Bundled.Claims.ExpiresAt}, nil
}

func (c *Channel) valid(g invite.Grant) bool {
	return c.now().Unix() >= c.issued && c.now().Unix() < c.expires && invite.VerifyGrant(c.issuer, g, g.Claims.Target, g.Claims.Controller, c.now()) && (g.Claims.Target == c.subject || g.Claims.Controller == c.subject)
}

// Observe accepts only the exact request's successful native product reply.
// The caller validates action=approve/status and binds RequestID to the input.
func (c *Channel) Observe(request string, g invite.Grant) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed || c.pinned || request == "" || !c.valid(g) {
		return ErrDenied
	}
	if c.request != "" && (c.request != request || c.grant != g) {
		return ErrDenied
	}
	c.request = request
	c.grant = g
	return nil
}

// Pin happens before the single provision attempt, including uncertain startup.
func (c *Channel) Pin(g invite.Grant) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed || c.pinned || c.request == "" || c.grant != g || !c.valid(g) {
		return ErrDenied
	}
	c.pinned = true
	return nil
}

// Revoke is explicit and serialized with Close. On an uncertain mutation the
// NEXT explicit call first queries the same request; no blind mutation retry.
// A signed grant alone is not a revocation receipt or payload-join evidence.
func (c *Channel) Revoke(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if ctx == nil || ctx.Err() != nil || c.closed || !c.pinned || c.grant.Claims.Target != c.subject || c.now().Unix() < c.issued || c.now().Unix() >= c.expires {
		return ErrDenied
	}
	if c.confirmed {
		return nil
	}
	if c.attempted {
		r, e := c.api.DeviceRequest(ctx, invite.Body{Action: "status", RequestID: c.request})
		if e != nil || r.RequestID != c.request {
			return ErrDenied
		}
		if r.Status == "revoked" && r.Grant == nil {
			c.confirmed = true
			return nil
		}
		if r.Status != "approved" || r.Grant == nil || *r.Grant != c.grant {
			return ErrDenied
		}
	}
	c.attempted = true
	r, e := c.api.DeviceRequest(ctx, invite.Body{Action: "revoke", GrantID: c.grant.Claims.ID})
	if e != nil || r.Status != "revoked" || r.Grant != nil {
		return ErrDenied
	}
	c.confirmed = true
	return nil
}
func (c *Channel) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.closed {
		c.closed = true
		c.api.Close()
	}
}
