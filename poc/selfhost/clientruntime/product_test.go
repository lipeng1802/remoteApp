package clientruntime

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
	"encoding/json"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/privatepipe"
	"remoteapp.local/selfhost-poc/processowner"
)

const testHMAC = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="

func TestTLSProductFixture(t *testing.T) {
	if len(os.Args) < 2 || !strings.HasPrefix(os.Args[len(os.Args)-1], "tls-fixture-") {
		return
	}
	mode := strings.TrimPrefix(os.Args[len(os.Args)-1], "tls-fixture-")
	prepared := strings.HasPrefix(mode, "prepared-")
	preparationMode := ""
	if prepared {
		mode = strings.TrimPrefix(mode, "prepared-")
		for _, flag := range []string{"deny", "wrong", "missing", "slow", "readywrong", "connect", "connectcancel", "connectwrong"} {
			if strings.HasPrefix(mode, flag+"-") {
				preparationMode, mode = flag, strings.TrimPrefix(mode, flag+"-")
				break
			}
		}
	}
	scope := ""
	for _, value := range []string{"desktop-view", "desktop-control"} {
		if strings.HasPrefix(mode, value+"-") {
			scope, mode = value, strings.TrimPrefix(mode, value+"-")
			break
		}
	}
	reader := privatepipe.NewReader(os.Stdin)
	if prepared {
		var prepare struct{ Op, Permission, Role, Self string }
		if reader.Read(&prepare) != nil || prepare.Op != "prepare" || prepare.Permission != scope || len(prepare.Self) != 64 {
			os.Exit(7)
		}
		role := "target"
		if strings.HasPrefix(mode, "controller") {
			role = "controller"
		}
		if prepare.Role != role {
			os.Exit(7)
		}
		ackScope := scope
		if preparationMode == "readywrong" {
			ackScope = "fixed-payload"
		}
		json.NewEncoder(os.Stdout).Encode(map[string]string{"status": "desktop_prepared", "Permission": ackScope, "Role": role})
		if strings.HasPrefix(preparationMode, "connect") {
			var intent struct{ Op string }
			if reader.Read(&intent) != nil || intent.Op != "connect_intent" {
				os.Exit(7)
			}
			r := map[string]string{"status": "desktop_connect", "Permission": scope, "TargetCode": "123456abcdef", "Code": "12345678"}
			if preparationMode == "connectcancel" {
				r = map[string]string{"status": "desktop_connect_cancelled", "Permission": scope}
			}
			if preparationMode == "connectwrong" {
				r["Permission"] = "fixed-payload"
			}
			json.NewEncoder(os.Stdout).Encode(r)
		}
		var consent struct{ Op, RequestID, Controller string }
		if reader.Read(&consent) != nil || consent.Op == "stop" {
			json.NewEncoder(os.Stdout).Encode(map[string]string{"status": "closed"})
			os.Exit(0)
		}
		if consent.Op != "consent" {
			os.Exit(7)
		}
		if preparationMode == "slow" {
			for _, arg := range os.Args {
				if marker, ok := strings.CutPrefix(arg, "prepared-barrier="); ok {
					_ = os.WriteFile(marker, []byte("entered"), 0600)
				}
			}
			// Hold the local dialog until cancellation closes the private pipe.
			var end struct{ Op string }
			_ = reader.Read(&end)
			time.Sleep(150 * time.Millisecond)
			json.NewEncoder(os.Stdout).Encode(map[string]string{"status": "closed"})
			os.Exit(0)
		}
		decision := map[string]any{"status": "desktop_decision", "Permission": scope, "RequestID": consent.RequestID, "Allow": preparationMode != "deny"}
		if preparationMode == "wrong" {
			decision["RequestID"] = "foreign"
		}
		if preparationMode == "missing" {
			delete(decision, "Allow")
		}
		json.NewEncoder(os.Stdout).Encode(decision)
		if !strings.HasPrefix(mode, "controller") {
			var activate struct{ Op string }
			if reader.Read(&activate) != nil || activate.Op == "stop" {
				json.NewEncoder(os.Stdout).Encode(map[string]string{"status": "closed"})
				os.Exit(0)
			}
			if activate.Op != "activate" {
				os.Exit(7)
			}
		}
	}
	var cert []byte
	var priv *rsa.PrivateKey
	if strings.HasPrefix(mode, "controller") {
		var cfg struct {
			Op                       string
			Port                     int
			Key, Fingerprint, Target string
			Permission               string
		}
		if reader.Read(&cfg) != nil {
			os.Exit(7)
		}
		if prepared && cfg.Op == "stop" {
			json.NewEncoder(os.Stdout).Encode(map[string]string{"status": "closed"})
			os.Exit(0)
		}
		if cfg.Op != "" || cfg.Port != 32123 || cfg.Key != testHMAC || cfg.Fingerprint != strings.Repeat("c", 64) || len(cfg.Target) != 64 || (scope != "" && cfg.Permission != scope) {
			os.Exit(7)
		}
		ready := map[string]string{"status": "ready"}
		if scope != "" {
			ready["Permission"] = scope
		}
		json.NewEncoder(os.Stdout).Encode(ready)
	} else {
		var e error
		priv, e = rsa.GenerateKey(rand.Reader, 2048)
		if e != nil {
			os.Exit(7)
		}
		now := time.Now()
		c := &x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
		cert, e = x509.CreateCertificate(rand.Reader, c, c, &priv.PublicKey, priv)
		if e != nil {
			os.Exit(7)
		}
		fp := sha256.Sum256(cert)
		json.NewEncoder(os.Stdout).Encode(productReady{Status: "ready", Key: testHMAC, Fingerprint: hex.EncodeToString(fp[:]), Permission: scope})
	}
	for {
		var cmd struct{ Op, GrantID, Target, Challenge string }
		if reader.Read(&cmd) != nil {
			os.Exit(7)
		}
		if cmd.Op == "stop" {
			json.NewEncoder(os.Stdout).Encode(map[string]string{"status": "closed"})
			if mode == "controller-nonzero" {
				os.Exit(7)
			}
			os.Exit(0)
		}
		if cmd.Op == "count" {
			if mode == "controller" {
				json.NewEncoder(os.Stdout).Encode(map[string]any{"status": "count", "pongs": 3, "ended": false, "result": "pending", "pin_rejected": false})
			} else {
				json.NewEncoder(os.Stdout).Encode(map[string]any{"status": "count", "pongs": 3, "preauth_rejected": 0, "sessions_closed": 0})
			}
			continue
		}
		if cmd.Op != "certificate_proof" || priv == nil {
			os.Exit(7)
		}
		fp := sha256.Sum256(cert)
		h := sha256.Sum256(invite.TLSCertificateProofBytes(invite.Grant{Claims: invite.Claims{ID: cmd.GrantID, Target: cmd.Target}}, hex.EncodeToString(fp[:]), cmd.Challenge))
		sig, e := rsa.SignPSS(rand.Reader, priv, crypto.SHA256, h[:], &rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthEqualsHash})
		if e != nil {
			os.Exit(7)
		}
		if mode == "badproof" {
			sig[0] ^= 1
		}
		json.NewEncoder(os.Stdout).Encode(map[string]string{"status": "certificate_proof", "Certificate": base64.StdEncoding.EncodeToString(cert), "CertificateProof": hex.EncodeToString(sig)})
	}
}

type productAPI struct {
	key                 ed25519.PrivateKey
	c                   Config
	fingerprint, mutate string
	published           bool
}

func (a *productAPI) Request(ctx context.Context, b invite.NetworkRequestBody) (invite.NetworkReply, error) {
	if ctx.Err() != nil || b.Grant != a.c.Grant {
		return invite.NetworkReply{}, invite.ErrDenied
	}
	if b.Action == "publish_tls" {
		der, e := base64.StdEncoding.DecodeString(b.Certificate)
		if e != nil {
			return invite.NetworkReply{}, invite.ErrDenied
		}
		cert, e := x509.ParseCertificate(der)
		if e != nil {
			return invite.NetworkReply{}, invite.ErrDenied
		}
		pub, ok := cert.PublicKey.(*rsa.PublicKey)
		if !ok {
			return invite.NetworkReply{}, invite.ErrDenied
		}
		fp := sha256.Sum256(der)
		fingerprint := hex.EncodeToString(fp[:])
		h := sha256.Sum256(invite.TLSCertificateProofBytes(b.Grant, fingerprint, b.Challenge))
		sig, e := hex.DecodeString(b.CertificateProof)
		if e != nil || rsa.VerifyPSS(pub, crypto.SHA256, h[:], sig, &rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthEqualsHash}) != nil {
			return invite.NetworkReply{}, invite.ErrDenied
		}
		if a.published && a.fingerprint != fingerprint {
			return invite.NetworkReply{}, invite.ErrDenied
		}
		a.fingerprint = fingerprint
		a.published = true
		return invite.NetworkReply{}, nil
	}
	if b.Action != "tls_identity" {
		return invite.NetworkReply{}, invite.ErrDenied
	}
	now := time.Now()
	claims := invite.TLSIdentityClaims{Version: 1, Grant: b.Grant, Binding: a.c.Binding, Fingerprint: a.fingerprint, Challenge: b.Challenge, IssuedAt: now.UnixNano(), ExpiresAt: now.Add(2 * time.Second).UnixNano()}
	switch a.mutate {
	case "challenge":
		claims.Challenge = strings.Repeat("0", 64)
	case "binding":
		claims.Binding.ControllerIP = "100.120.0.99"
	case "expired":
		claims.IssuedAt = now.Add(-3 * time.Second).UnixNano()
		claims.ExpiresAt = now.Add(-time.Second).UnixNano()
	}
	raw, _ := json.Marshal(claims)
	sig := ed25519.Sign(a.key, append([]byte("remoteapp-tls-identity-v1\n"), raw...))
	if a.mutate == "signature" {
		sig[0] ^= 1
	}
	d := invite.TLSIdentity{Claims: claims, Signature: hex.EncodeToString(sig)}
	r := invite.NetworkReply{TLS: &d}
	if a.mutate == "mixed" {
		r.Ticket = "extra"
	}
	return r, nil
}
func productFixture(t *testing.T) (*productAPI, ed25519.PublicKey) {
	t.Helper()
	pub, k, _ := ed25519.GenerateKey(rand.Reader)
	c := Config{Target: strings.Repeat("a", 64), Controller: strings.Repeat("b", 64), Binding: invite.NetworkBinding{TargetNode: "nodekey:" + strings.Repeat("a", 64), ControllerNode: "nodekey:" + strings.Repeat("b", 64), TargetIP: "100.120.0.42", ControllerIP: "100.120.0.93"}}
	now := time.Now().Unix()
	claims := invite.Claims{Version: 1, ID: strings.Repeat("a", 32), Target: c.Target, Controller: c.Controller, Permission: "fixed-payload", IssuedAt: now, ExpiresAt: now + 300}
	raw, _ := json.Marshal(claims)
	c.Grant = invite.Grant{Claims: claims, Signature: hex.EncodeToString(ed25519.Sign(k, append([]byte("remoteapp-invite-grant-v1\n"), raw...)))}
	if !invite.VerifyGrant(pub, c.Grant, c.Target, c.Controller, time.Now()) {
		t.Fatal("invalid grant fixture")
	}
	return &productAPI{key: k, c: c, fingerprint: strings.Repeat("c", 64)}, pub
}
func productSpec(t *testing.T, mode string) processowner.Spec {
	t.Helper()
	exe, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	exe, e = filepath.EvalSymlinks(exe)
	if e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(exe)
	if e != nil {
		t.Fatal(e)
	}
	h := sha256.Sum256(b)
	return processowner.Spec{Executable: exe, SHA256: hex.EncodeToString(h[:]), Args: []string{"-test.run=^TestTLSProductFixture$", "tls-fixture-" + mode}}
}
func stopProduct(t *testing.T, c *processowner.Child) {
	t.Helper()
	if c == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if e := c.Stop(ctx); e != nil {
		t.Fatal("fixture not cleanly joined", e)
	}
}
func TestProductTargetPublishesPossessionAndVerifiedDescriptor(t *testing.T) {
	a, pub := productFixture(t)
	cfg := a.c
	cfg.Server = true
	notified := false
	p := Product{Spec: productSpec(t, "target"), TargetReady: func(key, fp string) { notified = key == testHMAC && fp == a.fingerprint && a.published }}
	c, e := p.start(context.Background(), a, pub, cfg, time.Now, 0)
	if c != nil {
		defer stopProduct(t, c)
	}
	if e != nil || !notified {
		t.Fatal("target proof/descriptor not prepared", e)
	}
}
func TestProductBadPossessionRejectsWithoutSecretOutput(t *testing.T) {
	a, pub := productFixture(t)
	cfg := a.c
	cfg.Server = true
	notified := false
	p := Product{Spec: productSpec(t, "badproof"), TargetReady: func(string, string) { notified = true }}
	c, e := p.start(context.Background(), a, pub, cfg, time.Now, 0)
	if c != nil {
		defer stopProduct(t, c)
	}
	if e == nil || a.published || notified {
		t.Fatal("invalid possession published")
	}
}
func TestChangedCertificateRejectedBeforePublishingSessionSecret(t *testing.T) {
	a, pub := productFixture(t)
	cfg := a.c
	cfg.Server = true
	notified := false
	p := Product{Spec: productSpec(t, "target"), TargetReady: func(string, string) { notified = true }}
	first, e := p.start(context.Background(), a, pub, cfg, time.Now, 0)
	if e != nil {
		stopProduct(t, first)
		t.Fatal("initial publication failed", e)
	}
	stopProduct(t, first)
	fingerprint := a.fingerprint
	notified = false
	second, e := p.start(context.Background(), a, pub, cfg, time.Now, 0)
	defer stopProduct(t, second)
	if !errors.Is(e, ErrTLSCertificatePublication) || notified || a.fingerprint != fingerprint {
		t.Fatal("changed pin or session secret accepted")
	}
}
func TestProductControllerUsesVerifiedDescriptorAndPrivateConfig(t *testing.T) {
	a, pub := productFixture(t)
	p := Product{Spec: productSpec(t, "controller"), ControllerKey: testHMAC}
	c, e := p.start(context.Background(), a, pub, a.c, time.Now, 32123)
	if c != nil {
		defer stopProduct(t, c)
	}
	if e != nil {
		t.Fatal(e)
	}
}

func TestBoundProductRequiresDeliveredFingerprintToMatchIssuerDescriptor(t *testing.T) {
	a, pub := productFixture(t)
	p := Product{Spec: productSpec(t, "controller"), ControllerKey: testHMAC}
	child, err := StartBoundProduct(context.Background(), p, a, pub, a.c.Grant, a.c.Binding, false, time.Now, 32123, strings.Repeat("d", 64))
	if child != nil {
		defer stopProduct(t, child)
	}
	if err == nil || child != nil {
		t.Fatal("mismatched delivered fingerprint started product")
	}
	child, err = StartBoundProduct(context.Background(), p, a, pub, a.c.Grant, a.c.Binding, false, time.Now, 32123, a.fingerprint)
	if child != nil {
		defer stopProduct(t, child)
	}
	if err != nil || child == nil {
		t.Fatal("matching delivered fingerprint rejected", err)
	}
}
func TestProductDescriptorTamperRejectedBeforeProcessStart(t *testing.T) {
	for _, mode := range []string{"signature", "challenge", "binding", "expired"} {
		t.Run(mode, func(t *testing.T) {
			a, pub := productFixture(t)
			a.mutate = mode
			c, e := (Product{Spec: productSpec(t, "controller"), ControllerKey: testHMAC}).start(context.Background(), a, pub, a.c, time.Now, 32123)
			if c != nil {
				defer stopProduct(t, c)
			}
			if e == nil || c != nil {
				t.Fatal("tamper launched child")
			}
		})
	}
}
func TestProductInvalidHMACRejectedBeforeProcessStart(t *testing.T) {
	a, pub := productFixture(t)
	for _, key := range []string{"", strings.Repeat("a", 32), testHMAC + "\n"} {
		c, e := (Product{Spec: productSpec(t, "controller"), ControllerKey: key}).start(context.Background(), a, pub, a.c, time.Now, 32123)
		if c != nil {
			stopProduct(t, c)
		}
		if e == nil || c != nil {
			t.Fatal("invalid HMAC launched child")
		}
	}
}
