package invite

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"time"
)

// VerifierPermit authenticates a source, NOT binding or payload permission.
type VerifierClaims struct {
	Version     int    `json:"version"`
	GrantID     string `json:"grant_id"`
	Device      string `json:"device"`
	Ticket      string `json:"ticket"`
	NodeKey     string `json:"node_key"`
	VerifierKey string `json:"verifier_key"`
	Challenge   string `json:"challenge"`
	IssuedAt    int64  `json:"issued_at"`
	ExpiresAt   int64  `json:"expires_at"`
}
type VerifierPermit struct {
	Claims    VerifierClaims `json:"claims"`
	Signature string         `json:"signature"`
}

func verifierBytes(c VerifierClaims) []byte {
	b, _ := json.Marshal(c)
	return append([]byte("remoteapp-verifier-permit-v1\n"), b...)
}
func canonicalHex(s string, n int) bool {
	b, e := hex.DecodeString(s)
	return e == nil && len(b) == n && hex.EncodeToString(b) == s
}
func VerifyVerifierPermit(pub ed25519.PublicKey, p VerifierPermit, grant Grant, device, ticket, node, challenge string, now time.Time) bool {
	c := p.Claims
	sig, e := hex.DecodeString(p.Signature)
	return len(pub) == 32 && VerifyGrant(pub, grant, grant.Claims.Target, grant.Claims.Controller, now) && e == nil && len(sig) == 64 && hex.EncodeToString(sig) == p.Signature && c.Version == 1 &&
		canonicalHex(c.GrantID, 16) && canonicalHex(device, 32) && canonicalHex(ticket, 16) && canonicalHex(challenge, 32) &&
		validNodeKey(node) && validNodeKey(c.VerifierKey) && c.VerifierKey != node &&
		c.GrantID == grant.Claims.ID && c.Device == device && (device == grant.Claims.Target || device == grant.Claims.Controller) &&
		c.Ticket == ticket && c.NodeKey == node && c.Challenge == challenge &&
		c.IssuedAt > 0 && c.IssuedAt <= now.Unix() && c.ExpiresAt > now.Unix() && c.ExpiresAt > c.IssuedAt && c.ExpiresAt-c.IssuedAt <= 30 &&
		c.ExpiresAt <= grant.Claims.ExpiresAt && ed25519.Verify(pub, verifierBytes(c), sig)
}
func (b *ProvisionBackend) VerifierPermit(ctx context.Context, g Grant, device, ticket, challenge string) (VerifierPermit, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.Store == nil || b.Driver == nil || !b.ready || b.VerifierSource == nil || ctx.Err() != nil || !canonicalHex(challenge, 32) {
		return VerifierPermit{}, ErrDenied
	}
	verifier, e := b.VerifierSource(ctx)
	if e != nil || ctx.Err() != nil || !validNodeKey(verifier) {
		return VerifierPermit{}, ErrDenied
	}
	s := b.Store
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.state.Provisions[ticket]
	now := s.clock().Unix()
	if s.broken || !s.provisionReady || !ok || r.Status != "issued" || r.Device != device || r.Grant != g || r.NodeKey == verifier || r.Expires <= now || !s.authorizedLocked(g, g.Claims.Target, g.Claims.Controller) {
		return VerifierPermit{}, ErrDenied
	}
	expires := now + 30
	if expires > r.Expires {
		expires = r.Expires
	}
	if expires > g.Claims.ExpiresAt {
		expires = g.Claims.ExpiresAt
	}
	c := VerifierClaims{1, g.Claims.ID, device, ticket, r.NodeKey, verifier, challenge, now, expires}
	key, _ := hex.DecodeString(s.state.PrivateKey)
	return VerifierPermit{c, hex.EncodeToString(ed25519.Sign(key, verifierBytes(c)))}, nil
}
