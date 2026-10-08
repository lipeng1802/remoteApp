package trustinstall

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/url"
	"remoteapp.local/selfhost-poc/trustprofile"
	"strconv"
	"time"
)

// TestPairInput is explicit TRUSTED management bootstrap, never a discovered
// peer/HTTP reply. Only isolated test builds use this path. No installed or
// production root is read, exported, replaced or implicitly authorized.
type TestPairInput struct {
	Version            int    `json:"version"`
	Certificate        string `json:"certificate"`
	Issuer             string `json:"issuer"`
	TargetEndpoint     string `json:"target_endpoint"`
	ControllerEndpoint string `json:"controller_endpoint"`
}
type TestPairMaterial struct {
	Anchor                           PublicAnchor
	Target, Controller               trustprofile.Profile
	TargetUpgrade, ControllerUpgrade trustprofile.Profile
}

// NewTestPairMaterial mints a throwaway TEST release root only in memory.
// Both profiles name the SAME service CA/issuer, but compile each machine's
// explicit management-forward endpoint. They cannot become production trust.
// Candidate epoch 2 is pre-signed, not an online signing/rotation API.
func NewTestPairMaterial(in TestPairInput, now time.Time) (TestPairMaterial, error) {
	if in.Version != 1 || !canonical(in.Issuer, 32) {
		return TestPairMaterial{}, trustprofile.ErrTrust
	}
	// Only explicit SSH-management forwards, never discovered public endpoints.
	for _, endpoint := range []string{in.TargetEndpoint, in.ControllerEndpoint} {
		u, e := url.Parse(endpoint)
		if e != nil {
			return TestPairMaterial{}, trustprofile.ErrTrust
		}
		host, port, e := net.SplitHostPort(u.Host)
		n, pe := strconv.Atoi(port)
		if e != nil || pe != nil || u.Scheme != "https" || host != "127.0.0.1" || n < 1 || n > 65535 || n == 443 || strconv.Itoa(n) != port || u.User != nil || u.Path != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.String() != endpoint {
			return TestPairMaterial{}, trustprofile.ErrTrust
		}
	}
	cert, e := base64.StdEncoding.DecodeString(in.Certificate)
	if e != nil || len(cert) > 4096 || base64.StdEncoding.EncodeToString(cert) != in.Certificate {
		return TestPairMaterial{}, trustprofile.ErrTrust
	}
	ca, e := x509.ParseCertificate(cert)
	if e != nil || !ca.IsCA || !ca.BasicConstraintsValid || ca.KeyUsage&x509.KeyUsageCertSign == 0 || now.Before(ca.NotBefore) || !now.Add(20*time.Minute).Before(ca.NotAfter) {
		return TestPairMaterial{}, trustprofile.ErrTrust
	}
	pub, secret, e := ed25519.GenerateKey(rand.Reader)
	if e != nil {
		return TestPairMaterial{}, e
	}
	defer func() {
		for i := range secret {
			secret[i] = 0
		}
	}()
	anchor := PublicAnchor{Root: hex.EncodeToString(pub), Service: "compiled-pair-test", MinimumEpoch: 1}
	profile := func(endpoint string, epoch uint64) (trustprofile.Profile, error) {
		c := trustprofile.Claims{Version: 1, Service: anchor.Service, Epoch: epoch, Endpoint: endpoint, Certificate: in.Certificate, Issuer: in.Issuer, IssuedAt: now.Unix(), ExpiresAt: now.Add(20 * time.Minute).Unix()}
		p, e := trustprofile.Sign(secret, c)
		if e != nil {
			return p, e
		}
		_, e = trustprofile.Verify(p, pub, anchor.Service, 1, now)
		return p, e
	}
	out := TestPairMaterial{Anchor: anchor}
	if out.Target, e = profile(in.TargetEndpoint, 1); e != nil {
		return TestPairMaterial{}, e
	}
	if out.Controller, e = profile(in.ControllerEndpoint, 1); e != nil {
		return TestPairMaterial{}, e
	}
	if out.TargetUpgrade, e = profile(in.TargetEndpoint, 2); e != nil {
		return TestPairMaterial{}, e
	}
	if out.ControllerUpgrade, e = profile(in.ControllerEndpoint, 2); e != nil {
		return TestPairMaterial{}, e
	}
	return out, nil
}

// TestPairPublicFiles exposes ONLY public data; callers use CreateNew writes.
func (p TestPairMaterial) TestPairPublicFiles() (map[string][]byte, error) {
	values := map[string]any{"installation-anchor.json": p.Anchor, "target-profile.json": p.Target, "controller-profile.json": p.Controller, "target-upgrade.json": p.TargetUpgrade, "controller-upgrade.json": p.ControllerUpgrade}
	out := make(map[string][]byte, len(values))
	for name, value := range values {
		b, e := json.Marshal(value)
		if e != nil {
			return nil, e
		}
		out[name] = b
	}
	return out, nil
}
