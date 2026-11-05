package session

import (
	"context"
	"crypto/ed25519"
	"sync"
	"time"

	"remoteapp.local/selfhost-poc/invite"
)

// StateCache consumes authenticated, absolute-expiry snapshots. Receipt does
// not reset their TTL. An authenticated inactive snapshot permanently latches
// this grant off; a positive replay cannot undo revoke (new grant needs restart).
type StateCache struct {
	policy  invite.GrantPolicy
	mu      sync.Mutex
	issuer  ed25519.PublicKey
	grant   invite.Grant
	binding invite.NetworkBinding
	now     func() time.Time
	last    invite.NetworkState
	denied  bool
}

func NewStateCache(pub ed25519.PublicKey, g invite.Grant, b invite.NetworkBinding, now func() time.Time) *StateCache {
	return NewStateCacheForPolicy(invite.GrantPolicy{}, pub, g, b, now)
}

// NewStateCacheForPolicy binds the cache to trusted local scope as well as the
// exact grant and binding. Receipt time never extends a state or grant lease.
func NewStateCacheForPolicy(policy invite.GrantPolicy, pub ed25519.PublicKey, g invite.Grant, b invite.NetworkBinding, now func() time.Time) *StateCache {
	return &StateCache{policy: policy, issuer: append(ed25519.PublicKey(nil), pub...), grant: g, binding: b, now: now}
}
func (c *StateCache) Update(n invite.NetworkState) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.now == nil || !c.policy.Verify(c.issuer, c.grant, c.grant.Claims.Target, c.grant.Claims.Controller, time.Unix(c.grant.Claims.IssuedAt, 0)) || !invite.VerifyNetworkState(c.issuer, n, c.now()) || n.Claims.Grant != c.grant || n.Claims.Binding != c.binding || n.Claims.IssuedAt < c.last.Claims.IssuedAt {
		return false
	}
	if !n.Claims.Active {
		c.denied = true
	}
	if c.denied && n.Claims.Active {
		return false
	}
	c.last = n
	return true
}
func (c *StateCache) Check(ctx context.Context, g invite.Grant, a, b string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return ctx.Err() == nil && c.now != nil && !c.denied && g == c.grant && g.Claims.Target == a && g.Claims.Controller == b && c.last.Claims.Active && c.policy.Verify(c.issuer, c.grant, a, b, c.now()) && invite.VerifyNetworkState(c.issuer, c.last, c.now())
}
