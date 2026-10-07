package invite

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"time"
)

// ClockProof is NOT authorization. A helper owns a fresh nonce and records its
// monotonic request start; the issuer signs its time for that nonce and grant.
type ClockClaims struct {
	Version  int    `json:"version"`
	GrantID  string `json:"grant_id"`
	Nonce    string `json:"nonce"`
	IssuedAt int64  `json:"issued_at"`
}
type ClockProof struct {
	Claims    ClockClaims `json:"claims"`
	Signature string      `json:"signature"`
}

func clockBytes(c ClockClaims) []byte {
	b, _ := json.Marshal(c)
	return append([]byte("remoteapp-clock-proof-v1\n"), b...)
}
func (b *ProvisionBackend) ClockProof(g Grant, nonce string) (ClockProof, error) {
	raw, e := hex.DecodeString(nonce)
	if e != nil || len(raw) != 32 || hex.EncodeToString(raw) != nonce {
		return ClockProof{}, ErrDenied
	}
	n, e := b.NetworkState(g)
	if e != nil || !n.Claims.Active {
		return ClockProof{}, ErrDenied
	}
	return b.GrantClockProof(g, nonce)
}

// A newly approved device needs time calibration before network enrollment.
// This proves issuer time for an online approved grant, NEVER network readiness.
func (b *ProvisionBackend) GrantClockProof(g Grant, nonce string) (ClockProof, error) {
	raw, e := hex.DecodeString(nonce)
	if e != nil || len(raw) != 32 || hex.EncodeToString(raw) != nonce {
		return ClockProof{}, ErrDenied
	}
	s := b.Store
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.broken || !s.authorizedLocked(g, g.Claims.Target, g.Claims.Controller) {
		return ClockProof{}, ErrDenied
	}
	c := ClockClaims{1, g.Claims.ID, nonce, s.clock().UnixNano()}
	key, _ := hex.DecodeString(s.state.PrivateKey)
	return ClockProof{c, hex.EncodeToString(ed25519.Sign(key, clockBytes(c)))}, nil
}

// AuthorityClock deliberately counts the entire request RTT from the helper's
// monotonic start, rather than restarting a lease at receipt. It conservatively
// runs ahead by the outbound leg, never adds clock skew to a signed lease.
// The helper must generate/consume this nonce once and pin the issuer separately.
func AuthorityClock(pub ed25519.PublicKey, proof ClockProof, grantID, nonce string, start time.Time) (func() time.Time, error) {
	c := proof.Claims
	raw, e := hex.DecodeString(nonce)
	sig, se := hex.DecodeString(proof.Signature)
	elapsed := time.Since(start)
	if len(pub) != 32 || e != nil || len(raw) != 32 || hex.EncodeToString(raw) != nonce || se != nil || len(sig) != 64 || c.Version != 1 || c.GrantID != grantID || len(grantID) != 32 || c.Nonce != nonce || c.IssuedAt <= 0 || elapsed < 0 || elapsed > 5*time.Second || !ed25519.Verify(pub, clockBytes(c), sig) {
		return nil, ErrDenied
	}
	return func() time.Time { return time.Unix(0, c.IssuedAt).Add(time.Since(start)) }, nil
}
