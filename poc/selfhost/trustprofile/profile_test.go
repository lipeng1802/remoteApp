package trustprofile

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"testing"
	"time"
)

func fixture(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey, Claims) {
	t.Helper()
	pub, key, _ := ed25519.GenerateKey(rand.Reader)
	caPub, caKey, _ := ed25519.GenerateKey(rand.Reader)
	issuer, _, _ := ed25519.GenerateKey(rand.Reader)
	now := time.Now().Truncate(time.Second)
	cert := &x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: now.Add(-time.Hour), NotAfter: now.Add(24 * time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature}
	der, e := x509.CreateCertificate(rand.Reader, cert, cert, caPub, caKey)
	if e != nil {
		t.Fatal(e)
	}
	return pub, key, Claims{1, "remoteapp-test", 2, "https://service.example:8443", base64.StdEncoding.EncodeToString(der), hex.EncodeToString(issuer), now.Add(-time.Minute).Unix(), now.Add(time.Hour).Unix()}
}
func TestReleaseProfileVerificationAndStrictParsing(t *testing.T) {
	root, key, c := fixture(t)
	p, e := Sign(key, c)
	if e != nil {
		t.Fatal(e)
	}
	raw, _ := json.Marshal(p)
	decoded, e := Decode(raw)
	if e != nil || decoded != p {
		t.Fatal("roundtrip")
	}
	if _, e = Verify(p, root, c.Service, 2, time.Now()); e != nil {
		t.Fatal(e)
	}
	for _, data := range [][]byte{append(raw, raw...), []byte(`{"claims":{},"claims":{}}`), []byte(`{"claims":{},"CLAIMS":{}}`), []byte(`{"claims":{},"extra":1}`), make([]byte, 8193)} {
		if _, e = Decode(data); e == nil {
			t.Fatal("ambiguous manifest accepted")
		}
	}
	wrong, _, _ := ed25519.GenerateKey(rand.Reader)
	if _, e = Verify(p, wrong, c.Service, 1, time.Now()); e == nil {
		t.Fatal("untrusted root")
	}
	if _, e = Verify(p, root, "other", 1, time.Now()); e == nil {
		t.Fatal("wrong service")
	}
	if _, e = Verify(p, root, c.Service, 3, time.Now()); e == nil {
		t.Fatal("rollback")
	}
	if _, e = Verify(p, root, c.Service, 0, time.Now()); e == nil {
		t.Fatal("missing installation floor")
	}
	mutated := p
	mutated.Claims.Endpoint = "https://evil.example"
	if _, e = Verify(mutated, root, c.Service, 1, time.Now()); e == nil {
		t.Fatal("endpoint tamper")
	}
	for _, change := range []func(*Claims){
		func(c *Claims) { c.Endpoint = "http://service.example" }, func(c *Claims) { c.Endpoint = "https://user@service.example" }, func(c *Claims) { c.Endpoint = "https://service.example/?query" },
		func(c *Claims) { c.ExpiresAt = time.Now().Unix() }, func(c *Claims) { c.IssuedAt = time.Now().Add(time.Hour).Unix() }, func(c *Claims) { c.Epoch = 0 }, func(c *Claims) { c.Certificate = "invalid" },
	} {
		changed := c
		change(&changed)
		signed, _ := Sign(key, changed)
		if _, e = Verify(signed, root, c.Service, 1, time.Now()); e == nil {
			t.Fatal("invalid authorized manifest")
		}
	}
}
func TestRotationRequiresAdvanceAndNoSameEpochMutation(t *testing.T) {
	root, key, c := fixture(t)
	old, _ := Sign(key, c)
	if _, e := Advance(old, old, root, c.Service, 1, time.Now()); e != nil {
		t.Fatal("idempotent profile")
	}
	nextClaims := c
	nextClaims.Endpoint = "https://rotated.example:8443"
	same, _ := Sign(key, nextClaims)
	if _, e := Advance(old, same, root, c.Service, 1, time.Now()); e == nil {
		t.Fatal("same epoch mutation")
	}
	nextClaims.Epoch++
	next, _ := Sign(key, nextClaims)
	if _, e := Advance(old, next, root, c.Service, 1, time.Now()); e != nil {
		t.Fatal("signed newer configuration")
	}
	if _, e := Advance(next, old, root, c.Service, 1, time.Now()); e == nil {
		t.Fatal("downgrade after rotation")
	}
}
