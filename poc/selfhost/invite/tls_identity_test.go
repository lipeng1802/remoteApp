package invite

import (
	"context"
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"math/big"
	"strings"
	"testing"
	"time"
)

func identityCert(t *testing.T, now time.Time) (string, *rsa.PrivateKey) {
	t.Helper()
	key, e := rsa.GenerateKey(rand.Reader, 2048)
	if e != nil {
		t.Fatal(e)
	}
	c := &x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, e := x509.CreateCertificate(rand.Reader, c, c, &key.PublicKey, key)
	if e != nil {
		t.Fatal(e)
	}
	return base64.StdEncoding.EncodeToString(der), key
}
func identityPublish(t *testing.T, f *fixture, g Grant, cert string, key *rsa.PrivateKey, app ed25519.PrivateKey) NetworkRequest {
	t.Helper()
	e := apiEnvelope(f, app, g, "publish_tls")
	e.Body.Certificate = cert
	e.Body.Challenge = strings.Repeat("a", 64)
	_, fp, err := tlsCertificate(cert, f.now)
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(TLSCertificateProofBytes(g, fp, e.Body.Challenge))
	sig, err := rsa.SignPSS(rand.Reader, key, crypto.SHA256, h[:], &rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthEqualsHash})
	if err != nil {
		t.Fatal(err)
	}
	e.Body.CertificateProof = hex.EncodeToString(sig)
	return SignNetworkRequest(app, e.Body)
}
func TestTLSIdentityPossessionScopeFreshnessRestartRevocation(t *testing.T) {
	f, g, b, d := provisionFixture(t)
	cert, key := identityCert(t, f.now)
	e := identityPublish(t, f, g, cert, key, f.owner)
	if _, err := b.ExecuteNetworkRequest(context.Background(), e); err != ErrDenied {
		t.Fatal("unbound published identity")
	}
	bindPair(t, f, g, b)
	for _, mode := range []string{"controller", "wrong_private_key", "tamper", "scope"} {
		e = identityPublish(t, f, g, cert, key, f.owner)
		switch mode {
		case "controller":
			e = identityPublish(t, f, g, cert, key, f.client)
		case "wrong_private_key":
			_, wrong := identityCert(t, f.now)
			e = identityPublish(t, f, g, cert, wrong, f.owner)
		case "tamper":
			e.Body.CertificateProof = strings.Repeat("0", 512)
			e = SignNetworkRequest(f.owner, e.Body)
		case "scope":
			e.Body.Challenge = strings.Repeat("b", 64)
			e = SignNetworkRequest(f.owner, e.Body)
		}
		if _, err := b.ExecuteNetworkRequest(context.Background(), e); err == nil {
			t.Fatal(mode)
		}
	}
	f.now = f.now.Add(61 * time.Second) // replenish test device budget
	e = identityPublish(t, f, g, cert, key, f.owner)
	if _, err := b.ExecuteNetworkRequest(context.Background(), e); err != nil {
		t.Fatal(err)
	}
	n, _ := b.NetworkState(g)
	challenge := strings.Repeat("c", 64)
	identity, err := b.TLSIdentity(g, challenge)
	if err != nil || !VerifyTLSIdentity(f.s.PublicKey(), identity, g, n.Claims.Binding, challenge, f.now) {
		t.Fatal("signed descriptor")
	}
	for _, mode := range []string{"challenge", "binding", "fingerprint", "expiry"} {
		bad := identity
		binding := n.Claims.Binding
		check := challenge
		now := f.now
		switch mode {
		case "challenge":
			check = strings.Repeat("d", 64)
		case "binding":
			binding.TargetIP = "100.120.0.99"
		case "fingerprint":
			bad.Claims.Fingerprint = strings.Repeat("0", 64)
		case "expiry":
			now = now.Add(3 * time.Second)
		}
		if VerifyTLSIdentity(f.s.PublicKey(), bad, g, binding, check, now) {
			t.Fatal(mode)
		}
	}
	otherCert, otherKey := identityCert(t, f.now)
	if _, err := b.ExecuteNetworkRequest(context.Background(), identityPublish(t, f, g, otherCert, otherKey, f.owner)); err != ErrDenied {
		t.Fatal("silent rotation")
	}
	f.restart(t)
	b = &ProvisionBackend{Store: f.s, Driver: d}
	if b.Reconcile(context.Background()) != nil {
		t.Fatal("reconcile")
	}
	if _, err = b.TLSIdentity(g, challenge); err != nil {
		t.Fatal("identity persistence")
	}
	f.exec(t, f.owner, Body{Action: "revoke", GrantID: g.Claims.ID})
	if _, err = b.TLSIdentity(g, challenge); err != ErrDenied {
		t.Fatal("revoked identity")
	}
}
