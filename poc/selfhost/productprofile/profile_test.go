package productprofile

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"remoteapp.local/selfhost-poc/trustprofile"
	"testing"
	"time"
)

func TestVerifiedPublicProfileCannotReplaceTrustOrExtendValidity(t *testing.T) {
	root, key, _ := ed25519.GenerateKey(rand.Reader)
	caPub, caKey, _ := ed25519.GenerateKey(rand.Reader)
	issuer, _, _ := ed25519.GenerateKey(rand.Reader)
	now := time.Now().Truncate(time.Second)
	cert := &x509.Certificate{SerialNumber: big.NewInt(1), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(24 * time.Hour)}
	der, e := x509.CreateCertificate(rand.Reader, cert, cert, caPub, caKey)
	if e != nil {
		t.Fatal(e)
	}
	c := trustprofile.Claims{Version: 1, Service: "remoteapp-prerelease", Epoch: 1, Endpoint: "https://127.0.0.1:8444", Certificate: base64.StdEncoding.EncodeToString(der), Issuer: hex.EncodeToString(issuer), IssuedAt: now.Add(-time.Minute).Unix(), ExpiresAt: now.Add(time.Hour).Unix()}
	encode := func(c trustprofile.Claims) []byte {
		p, _ := trustprofile.Sign(key, c)
		raw, _ := json.Marshal(p)
		return raw
	}
	got, e := verified(encode(c), root, now)
	if e != nil {
		t.Fatal(e)
	}
	expected := c
	expected.IssuedAt = now.Unix()
	expected.ExpiresAt = now.Add(20 * time.Minute).Unix()
	if got != expected {
		t.Fatal("changed connectivity claims or invalid TEST lifetime")
	}
	c.ExpiresAt = now.Add(5 * time.Minute).Unix()
	got, e = verified(encode(c), root, now)
	if e != nil || got.ExpiresAt != c.ExpiresAt {
		t.Fatal("extended source validity")
	}
	for _, mode := range []string{"foreign_root", "expired", "endpoint", "service", "bad_signature"} {
		t.Run(mode, func(t *testing.T) {
			candidate := c
			anchor := root
			switch mode {
			case "foreign_root":
				anchor, _, _ = ed25519.GenerateKey(rand.Reader)
			case "expired":
				candidate.ExpiresAt = now.Unix()
			case "endpoint":
				candidate.Endpoint = "https://attacker.example:8444"
			case "service":
				candidate.Service = "other"
			}
			raw := encode(candidate)
			if mode == "bad_signature" {
				raw[len(raw)-3] ^= 1
			}
			if _, e := verified(raw, anchor, now); e == nil {
				t.Fatal("untrusted profile accepted")
			}
		})
	}
	if _, e = Claims(encode(c), now); e == nil {
		t.Fatal("production entry accepted test anchor")
	}
}
