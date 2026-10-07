package invite

import (
	"crypto"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"time"
)

// Public-only record. Certificate private keys/HMAC secrets never reach Store.
type TLSIdentityRecord struct {
	Certificate string
	GrantID     string
}
type TLSIdentityClaims struct {
	Version     int            `json:"version"`
	Grant       Grant          `json:"grant"`
	Binding     NetworkBinding `json:"binding"`
	Fingerprint string         `json:"fingerprint"`
	Challenge   string         `json:"challenge"`
	IssuedAt    int64          `json:"issued_at"`
	ExpiresAt   int64          `json:"expires_at"`
}
type TLSIdentity struct {
	Claims    TLSIdentityClaims `json:"claims"`
	Signature string            `json:"signature"`
}

func tlsIdentityBytes(c TLSIdentityClaims) []byte {
	raw, _ := json.Marshal(c)
	return append([]byte("remoteapp-tls-identity-v1\n"), raw...)
}
func tlsCertificate(raw string, now time.Time) (*x509.Certificate, string, error) {
	der, e := base64.StdEncoding.DecodeString(raw)
	if e != nil || len(der) > 4096 || base64.StdEncoding.EncodeToString(der) != raw {
		return nil, "", ErrDenied
	}
	c, e := x509.ParseCertificate(der)
	if e != nil || now.Before(c.NotBefore) || !now.Before(c.NotAfter) || c.IsCA || c.KeyUsage&x509.KeyUsageDigitalSignature == 0 {
		return nil, "", ErrDenied
	}
	rsaKey, ok := c.PublicKey.(*rsa.PublicKey)
	if !ok || rsaKey.N.BitLen() < 2048 || rsaKey.N.BitLen() > 4096 {
		return nil, "", ErrDenied
	}
	server := false
	for _, u := range c.ExtKeyUsage {
		if u == x509.ExtKeyUsageServerAuth {
			server = true
		}
	}
	if !server {
		return nil, "", ErrDenied
	}
	h := sha256.Sum256(der)
	return c, hex.EncodeToString(h[:]), nil
}

// TLS private-key possession is bound to this exact approved grant/target and
// fingerprint; it cannot be reused as an application/device signature.
func TLSCertificateProofBytes(g Grant, fingerprint, nonce string) []byte {
	return []byte("remoteapp-tls-certificate-proof-v1\n" + g.Claims.ID + "\n" + g.Claims.Target + "\n" + fingerprint + "\n" + nonce)
}
func (b *ProvisionBackend) publishTLS(e NetworkRequest) error {
	n, err := b.NetworkState(e.Body.Grant)
	if err != nil || !n.Claims.Active {
		return ErrDenied
	}
	s := b.Store
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.broken || !s.authorizedLocked(e.Body.Grant, e.Body.Grant.Claims.Target, e.Body.Grant.Claims.Controller) {
		return ErrDenied
	}
	now := s.clock()
	cert, fp, err := tlsCertificate(e.Body.Certificate, now)
	if err != nil {
		return err
	}
	proof, err := hex.DecodeString(e.Body.CertificateProof)
	if err != nil || len(proof) > 512 || hex.EncodeToString(proof) != e.Body.CertificateProof {
		return ErrDenied
	}
	h := sha256.Sum256(TLSCertificateProofBytes(e.Body.Grant, fp, e.Body.Challenge))
	if rsa.VerifyPSS(cert.PublicKey.(*rsa.PublicKey), crypto.SHA256, h[:], proof, &rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthEqualsHash}) != nil {
		return ErrDenied
	}
	old, found := s.state.TLSIdentities[e.Body.Grant.Claims.ID]
	r := TLSIdentityRecord{e.Body.Certificate, e.Body.Grant.Claims.ID}
	if found {
		if old != r {
			return ErrDenied
		}
		return nil
	} // no silent pin rotation
	if len(s.state.TLSIdentities) >= 512 {
		return ErrDenied
	}
	next := s.provisionClone()
	if next.TLSIdentities == nil {
		next.TLSIdentities = make(map[string]TLSIdentityRecord)
	}
	next.TLSIdentities[r.GrantID] = r
	return s.provisionCommit(next)
}
func (b *ProvisionBackend) TLSIdentity(g Grant, challenge string) (TLSIdentity, error) {
	n, err := b.NetworkState(g)
	if err != nil || !n.Claims.Active {
		return TLSIdentity{}, ErrDenied
	}
	s := b.Store
	s.mu.Lock()
	defer s.mu.Unlock()
	r, exists := s.state.TLSIdentities[g.Claims.ID]
	if s.broken || !exists || !s.authorizedLocked(g, g.Claims.Target, g.Claims.Controller) {
		return TLSIdentity{}, ErrDenied
	}
	_, fp, err := tlsCertificate(r.Certificate, s.clock())
	if err != nil {
		return TLSIdentity{}, err
	}
	now := s.clock().UnixNano()
	end := now + int64(3*time.Second)
	if exp := time.Unix(g.Claims.ExpiresAt, 0).UnixNano(); exp < end {
		end = exp
	}
	c := TLSIdentityClaims{1, g, n.Claims.Binding, fp, challenge, now, end}
	key, _ := hex.DecodeString(s.state.PrivateKey)
	return TLSIdentity{c, hex.EncodeToString(ed25519.Sign(ed25519.PrivateKey(key), tlsIdentityBytes(c)))}, nil
}
func VerifyTLSIdentity(pub ed25519.PublicKey, d TLSIdentity, g Grant, binding NetworkBinding, challenge string, now time.Time) bool {
	c := d.Claims
	sig, e := hex.DecodeString(d.Signature)
	fp, fe := hex.DecodeString(c.Fingerprint)
	nonce, ne := hex.DecodeString(challenge)
	return len(pub) == 32 && e == nil && len(sig) == 64 && fe == nil && len(fp) == 32 && c.Fingerprint == hex.EncodeToString(fp) && ne == nil && len(nonce) == 32 && challenge == hex.EncodeToString(nonce) && c.Version == 1 && c.Grant == g && c.Binding == binding && validBinding(binding) && c.Challenge == challenge && VerifyGrant(pub, g, g.Claims.Target, g.Claims.Controller, now) && c.IssuedAt <= now.UnixNano() && c.ExpiresAt > now.UnixNano() && c.ExpiresAt <= time.Unix(g.Claims.ExpiresAt, 0).UnixNano() && c.ExpiresAt-c.IssuedAt > 0 && c.ExpiresAt-c.IssuedAt <= int64(3*time.Second) && ed25519.Verify(pub, tlsIdentityBytes(c), sig)
}
