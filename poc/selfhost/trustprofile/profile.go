// Package trustprofile verifies release-authorized service configuration.
// The release root and epoch floor must originate in the trusted installation,
// never from the endpoint, peer, or manifest itself.
package trustprofile

import (
	"bytes"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"strings"
	"time"

	"remoteapp.local/selfhost-poc/privatepipe"
)

var ErrTrust = errors.New("service_trust_rejected")

type Claims struct {
	Version     int    `json:"version"`
	Service     string `json:"service"`
	Epoch       uint64 `json:"epoch"`
	Endpoint    string `json:"endpoint"`
	Certificate string `json:"certificate"`
	Issuer      string `json:"issuer"`
	IssuedAt    int64  `json:"issued_at"`
	ExpiresAt   int64  `json:"expires_at"`
}
type Profile struct {
	Claims    Claims `json:"claims"`
	Signature string `json:"signature"`
}

func message(c Claims) []byte {
	b, _ := json.Marshal(c)
	return append([]byte("remoteapp-service-trust-v1\n"), b...)
}

// Signing is a release/offline tool operation, not an online issuer/device API.
func Sign(root ed25519.PrivateKey, c Claims) (Profile, error) {
	if len(root) != 64 || !bytes.Equal(ed25519.NewKeyFromSeed(root[:32]), root) {
		return Profile{}, ErrTrust
	}
	return Profile{c, hex.EncodeToString(ed25519.Sign(root, message(c)))}, nil
}

func Decode(data []byte) (Profile, error) {
	if len(data) > 8192 || len(data) == 0 {
		return Profile{}, ErrTrust
	}
	r := privatepipe.NewReader(bytes.NewReader(append(append([]byte(nil), data...), '\n')))
	var p Profile
	if r.Read(&p) != nil {
		return Profile{}, ErrTrust
	}
	var extra struct{}
	if r.Read(&extra) != io.EOF {
		return Profile{}, ErrTrust
	}
	return p, nil
}

func Verify(p Profile, root ed25519.PublicKey, service string, floor uint64, now time.Time) ([]byte, error) {
	c := p.Claims
	if len(root) != 32 || service == "" || len(service) > 64 || c.Service != service || c.Version != 1 || floor == 0 || c.Epoch < floor || c.Epoch > 1<<53-1 || c.IssuedAt <= 0 || c.ExpiresAt <= c.IssuedAt || c.ExpiresAt-c.IssuedAt > int64((180*24*time.Hour)/time.Second) || now.Unix() < c.IssuedAt || now.Unix() >= c.ExpiresAt {
		return nil, ErrTrust
	}
	if len(c.Endpoint) > 512 || len(c.Certificate) > 5500 || len(c.Issuer) != 64 || len(p.Signature) != 128 {
		return nil, ErrTrust
	}
	sig, se := hex.DecodeString(p.Signature)
	issuer, ie := hex.DecodeString(c.Issuer)
	if se != nil || ie != nil || hex.EncodeToString(sig) != p.Signature || hex.EncodeToString(issuer) != c.Issuer || len(issuer) != 32 || bytes.Equal(issuer, root) || !ed25519.Verify(root, message(c), sig) {
		return nil, ErrTrust
	}
	u, e := url.Parse(c.Endpoint)
	if e != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.RawPath != "" || u.Path != "" || u.Opaque != "" || u.String() != c.Endpoint || strings.ContainsAny(c.Endpoint, "\r\n\x00") {
		return nil, ErrTrust
	}
	cert, e := base64.StdEncoding.DecodeString(c.Certificate)
	if e != nil || len(cert) > 4096 || base64.StdEncoding.EncodeToString(cert) != c.Certificate {
		return nil, ErrTrust
	}
	parsed, e := x509.ParseCertificate(cert)
	if e != nil || !parsed.IsCA || !parsed.BasicConstraintsValid || parsed.KeyUsage&x509.KeyUsageCertSign == 0 || now.Before(parsed.NotBefore) || !now.Before(parsed.NotAfter) || time.Unix(c.ExpiresAt, 0).After(parsed.NotAfter) {
		return nil, ErrTrust
	}
	return cert, nil
}

// Advance rejects rollback and same-epoch mutation. Caller must durably commit
// this accepted profile/floor BEFORE use, and cancel old sessions before rotate.
// It does not store state, invent a replacement root, or auto-reconnect.
func Advance(old, next Profile, root ed25519.PublicKey, service string, floor uint64, now time.Time) ([]byte, error) {
	if old.Claims.Epoch > floor {
		floor = old.Claims.Epoch
	}
	cert, e := Verify(next, root, service, floor, now)
	if e != nil || (old.Claims.Epoch != 0 && next.Claims.Epoch == old.Claims.Epoch && next != old) {
		return nil, ErrTrust
	}
	return cert, nil
}
