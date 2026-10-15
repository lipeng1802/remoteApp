package invite

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"time"
)

// SessionIdentity is an issuer-signed, short-lived disclosure of the two
// registered device public keys for one already ACTIVE grant/binding. Device
// IDs in Grant are hashes and are insufficient for the mutual transcript.
type SessionIdentityClaims struct {
	Version          int            `json:"version"`
	Grant            Grant          `json:"grant"`
	Binding          NetworkBinding `json:"binding"`
	TargetPublic     string         `json:"target_public"`
	ControllerPublic string         `json:"controller_public"`
	Challenge        string         `json:"challenge"`
	IssuedAt         int64          `json:"issued_at"`
	ExpiresAt        int64          `json:"expires_at"`
}

type SessionIdentity struct {
	Claims    SessionIdentityClaims `json:"claims"`
	Signature string                `json:"signature"`
}

func sessionIdentityBytes(c SessionIdentityClaims) []byte {
	raw, _ := json.Marshal(c)
	return append([]byte("remoteapp-session-identity-v1\n"), raw...)
}

func registeredPublic(s *Store, id string) (ed25519.PublicKey, error) {
	d, ok := s.state.Devices[id]
	pub, err := hex.DecodeString(d.PublicKey)
	if !ok || err != nil || len(pub) != ed25519.PublicKeySize || hex.EncodeToString(pub) != d.PublicKey || deviceID(pub) != id {
		return nil, ErrDenied
	}
	return pub, nil
}

func (b *ProvisionBackend) SessionIdentity(g Grant, challenge string) (SessionIdentity, error) {
	n, err := b.NetworkState(g)
	if err != nil || !n.Claims.Active || !canonicalHex(challenge, 32) {
		return SessionIdentity{}, ErrDenied
	}
	s := b.Store
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.broken || !s.authorizedLocked(g, g.Claims.Target, g.Claims.Controller) {
		return SessionIdentity{}, ErrDenied
	}
	target, err := registeredPublic(s, g.Claims.Target)
	if err != nil {
		return SessionIdentity{}, err
	}
	controller, err := registeredPublic(s, g.Claims.Controller)
	if err != nil {
		return SessionIdentity{}, err
	}
	now := s.clock().UnixNano()
	end := now + int64(3*time.Second)
	if exp := time.Unix(g.Claims.ExpiresAt, 0).UnixNano(); exp < end {
		end = exp
	}
	c := SessionIdentityClaims{1, g, n.Claims.Binding, hex.EncodeToString(target), hex.EncodeToString(controller), challenge, now, end}
	key, _ := hex.DecodeString(s.state.PrivateKey)
	return SessionIdentity{c, hex.EncodeToString(ed25519.Sign(ed25519.PrivateKey(key), sessionIdentityBytes(c)))}, nil
}

func VerifySessionIdentity(pub ed25519.PublicKey, d SessionIdentity, g Grant, binding NetworkBinding, challenge string, now time.Time) bool {
	c := d.Claims
	sig, sigErr := hex.DecodeString(d.Signature)
	target, targetErr := hex.DecodeString(c.TargetPublic)
	controller, controllerErr := hex.DecodeString(c.ControllerPublic)
	nonce, nonceErr := hex.DecodeString(challenge)
	return len(pub) == ed25519.PublicKeySize && sigErr == nil && len(sig) == ed25519.SignatureSize &&
		targetErr == nil && len(target) == ed25519.PublicKeySize && c.TargetPublic == hex.EncodeToString(target) && deviceID(target) == g.Claims.Target &&
		controllerErr == nil && len(controller) == ed25519.PublicKeySize && c.ControllerPublic == hex.EncodeToString(controller) && deviceID(controller) == g.Claims.Controller &&
		!ed25519.PublicKey(target).Equal(ed25519.PublicKey(controller)) && nonceErr == nil && len(nonce) == 32 && challenge == hex.EncodeToString(nonce) &&
		c.Version == 1 && c.Grant == g && c.Binding == binding && validBinding(binding) && c.Challenge == challenge &&
		VerifyGrant(pub, g, g.Claims.Target, g.Claims.Controller, now) && c.IssuedAt <= now.UnixNano() && c.ExpiresAt > now.UnixNano() &&
		c.ExpiresAt <= time.Unix(g.Claims.ExpiresAt, 0).UnixNano() && c.ExpiresAt-c.IssuedAt > 0 && c.ExpiresAt-c.IssuedAt <= int64(3*time.Second) &&
		ed25519.Verify(pub, sessionIdentityBytes(c), sig)
}
