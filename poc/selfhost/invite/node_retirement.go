package invite

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"time"
)

// NodeRetirement is a short-lived proof of past, completed remote cleanup for
// one exact device/node. It grants no network access or identity selection.
type NodeRetirementClaims struct {
	Version   int    `json:"version"`
	Device    string `json:"device"`
	NodeKey   string `json:"node_key"`
	Address   string `json:"address"`
	GrantID   string `json:"grant_id"`
	Challenge string `json:"challenge"`
	IssuedAt  int64  `json:"issued_at"`
	ExpiresAt int64  `json:"expires_at"`
}
type NodeRetirement struct {
	Claims    NodeRetirementClaims `json:"claims"`
	Signature string               `json:"signature"`
}

func retirementBytes(c NodeRetirementClaims) []byte {
	raw, _ := json.Marshal(c)
	return append([]byte("remoteapp-node-retirement-v1\n"), raw...)
}

// Called under the ordinary device request's durable replay/rate transaction.
// Only Reconcile can turn a provision into cleaned after Driver.Cleanup returns.
func (s *Store) nodeRetirementLocked(st *snapshot, id string, b Body) (Result, error) {
	if b != (Body{Action: "node_retirement", Nonce: b.Nonce, IssuedAt: b.IssuedAt, NodeKey: b.NodeKey, Challenge: b.Challenge}) ||
		!validNodeKey(b.NodeKey) || !canonicalHex(b.Challenge, 32) {
		return Result{}, ErrDenied
	}
	var found *ProvisionRecord
	for _, record := range st.Provisions {
		if record.Device == id && record.NodeKey == b.NodeKey {
			if found != nil {
				return Result{}, ErrDenied
			}
			copy := record
			found = &copy
		}
	}
	if found == nil || found.Status != "cleaned" || !validOverlay(found.IP) {
		return Result{}, ErrDenied
	}
	revoked := false
	for _, request := range st.Requests {
		if request.Status == "revoked" && request.Grant != nil && *request.Grant == found.Grant {
			revoked = true
		}
	}
	if !revoked {
		return Result{}, ErrDenied
	}
	now := s.clock().UnixNano()
	c := NodeRetirementClaims{1, id, b.NodeKey, found.IP, found.Grant.Claims.ID, b.Challenge, now, now + int64(3*time.Second)}
	key, _ := hex.DecodeString(st.PrivateKey)
	p := NodeRetirement{c, hex.EncodeToString(ed25519.Sign(ed25519.PrivateKey(key), retirementBytes(c)))}
	return Result{Status: "node_retired", Retirement: &p}, nil
}

func VerifyNodeRetirement(issuer ed25519.PublicKey, p NodeRetirement, device, node, address, challenge string, now time.Time) bool {
	c := p.Claims
	sig, err := hex.DecodeString(p.Signature)
	return len(issuer) == 32 && err == nil && len(sig) == 64 && hex.EncodeToString(sig) == p.Signature &&
		c.Version == 1 && canonicalHex(device, 32) && validNodeKey(node) && validOverlay(address) && canonicalHex(challenge, 32) &&
		c.Device == device && c.NodeKey == node && c.Address == address && c.Challenge == challenge && canonicalHex(c.GrantID, 16) &&
		c.IssuedAt <= now.UnixNano() && c.ExpiresAt > now.UnixNano() && c.ExpiresAt-c.IssuedAt == int64(3*time.Second) &&
		ed25519.Verify(issuer, retirementBytes(c), sig)
}
