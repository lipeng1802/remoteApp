package trustinstall

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"remoteapp.local/selfhost-poc/trustprofile"
	"strings"
	"testing"
	"time"
)

func pairInput(t *testing.T, now time.Time) TestPairInput {
	t.Helper()
	pub, key, _ := ed25519.GenerateKey(rand.Reader)
	issuer, _, _ := ed25519.GenerateKey(rand.Reader)
	ca := &x509.Certificate{SerialNumber: big.NewInt(1), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour)}
	der, e := x509.CreateCertificate(rand.Reader, ca, ca, pub, key)
	if e != nil {
		t.Fatal(e)
	}
	return TestPairInput{Version: 1, Certificate: base64.StdEncoding.EncodeToString(der), Issuer: hex.EncodeToString(issuer), TargetEndpoint: "https://127.0.0.1:41001", ControllerEndpoint: "https://127.0.0.1:41002"}
}
func TestTestPairMatchingIssuerCAAndAdvance(t *testing.T) {
	now := time.Now()
	in := pairInput(t, now)
	p, e := NewTestPairMaterial(in, now)
	if e != nil {
		t.Fatal(e)
	}
	root, _ := hex.DecodeString(p.Anchor.Root)
	for _, v := range []trustprofile.Profile{p.Target, p.Controller, p.TargetUpgrade, p.ControllerUpgrade} {
		if _, e := trustprofile.Verify(v, root, p.Anchor.Service, 1, now); e != nil {
			t.Fatal(e)
		}
		if v.Claims.Issuer != in.Issuer || v.Claims.Certificate != in.Certificate {
			t.Fatal("service mismatch")
		}
	}
	if _, e = trustprofile.Advance(p.Target, p.TargetUpgrade, root, p.Anchor.Service, 1, now); e != nil {
		t.Fatal(e)
	}
	if _, e = trustprofile.Advance(p.Target, p.Controller, root, p.Anchor.Service, 1, now); e == nil {
		t.Fatal("same epoch endpoint mutation accepted")
	}
	if _, e = trustprofile.Verify(p.Target, root, p.Anchor.Service, 1, now.Add(20*time.Minute)); e == nil {
		t.Fatal("expired profile accepted")
	}
	files, e := p.TestPairPublicFiles()
	if e != nil || len(files) != 5 {
		t.Fatal("public files")
	}
	for name, b := range files {
		if strings.Contains(string(b), "PRIVATE KEY") || strings.Contains(name, "secret") {
			t.Fatal("private material emitted")
		}
		var object map[string]any
		if json.Unmarshal(b, &object) != nil {
			t.Fatal("not public JSON")
		}
	}
	other, e := NewTestPairMaterial(in, now)
	if e != nil || other.Anchor.Root == p.Anchor.Root {
		t.Fatal("root reused")
	}
	otherRoot, _ := hex.DecodeString(other.Anchor.Root)
	if _, e = trustprofile.Verify(p.Target, otherRoot, p.Anchor.Service, 1, now); e == nil {
		t.Fatal("foreign root accepted")
	}
}
func TestTestPairRejectsBadTrustedBootstrap(t *testing.T) {
	now := time.Now()
	good := pairInput(t, now)
	changes := []func(*TestPairInput){func(p *TestPairInput) { p.Version = 0 }, func(p *TestPairInput) { p.Issuer = "ABC" }, func(p *TestPairInput) { p.Certificate = "garbage" }, func(p *TestPairInput) { p.TargetEndpoint = "http://127.0.0.1:1" }, func(p *TestPairInput) { p.ControllerEndpoint = "https://user@example.test:2" }, func(p *TestPairInput) { p.TargetEndpoint = "https://example.test:2?root=x" }}
	for _, change := range changes {
		in := good
		change(&in)
		if _, e := NewTestPairMaterial(in, now); e == nil {
			t.Fatal("invalid test bootstrap accepted")
		}
	}
	if _, e := NewTestPairMaterial(good, now.Add(time.Hour)); e == nil {
		t.Fatal("expired CA accepted")
	}
}
