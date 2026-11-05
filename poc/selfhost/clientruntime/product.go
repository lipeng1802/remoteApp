package clientruntime

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"remoteapp.local/selfhost-poc/cleanupwitness"
	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/privatepipe"
	"remoteapp.local/selfhost-poc/processowner"
	"remoteapp.local/selfhost-poc/session"
)

// Product is trusted runtime wiring for the actual Swift/C# TLS probes. The
// compiled command fixes Spec. Legacy runners may still supply ControllerKey;
// the autonomous installeddevice path rejects that field and performs its own
// mutually authenticated one-shot delivery into the anonymous product pipe.
type Product struct {
	Spec              processowner.Spec
	ControllerKey     string
	TargetReady       func(key, fingerprint string) // private parent pipe; never log
	Observation       *ProductObservation           // trusted local telemetry; never accepts child wiring
	CertificateScope  string                        // nonempty only for compiled Windows TEST target
	CertificateReopen bool
	CertificateOpened func(string) // local ownership bookkeeping; no private key
}

var ErrTLSCertificatePublication = errors.New("product_tls_publication_failed")
var ErrTestCertificateOpen = errors.New("product_test_certificate_open_failed")

type ProductNetworkAPI interface {
	Request(context.Context, invite.NetworkRequestBody) (invite.NetworkReply, error)
}
type networkAPI = ProductNetworkAPI
type productReady struct {
	Status           string `json:"status"`
	Key, Fingerprint string
	Permission       string `json:"Permission,omitempty"`
}

func hmacKey(s string) bool {
	b, e := base64.StdEncoding.DecodeString(s)
	return e == nil && len(b) == 32 && base64.StdEncoding.EncodeToString(b) == s
}
func nonce() (string, error) {
	var b [32]byte
	_, e := rand.Read(b[:])
	return hex.EncodeToString(b[:]), e
}
func descriptor(ctx context.Context, api networkAPI, issuer ed25519.PublicKey, c Config, clock func() time.Time) (invite.TLSIdentity, error) {
	n, e := nonce()
	if e != nil {
		return invite.TLSIdentity{}, session.ErrDenied
	}
	r, e := api.Request(ctx, invite.NetworkRequestBody{Action: "tls_identity", Grant: c.Grant, Challenge: n})
	if e != nil || r.TLS == nil || !invite.VerifyTLSIdentityForPolicy(c.grantPolicy, issuer, *r.TLS, c.Grant, c.Binding, n, clock()) {
		return invite.TLSIdentity{}, session.ErrDenied
	}
	return *r.TLS, nil
}
func (p Product) start(ctx context.Context, api networkAPI, issuer ed25519.PublicKey, c Config, clock func() time.Time, port int) (*processowner.Child, error) {
	// A bounded startup, but child cleanup is deliberately unbounded/joined by
	// the outer runner. Deadline failure cannot release the installation lease.
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	if !c.Server && (p.CertificateScope != "" || p.CertificateReopen || p.CertificateOpened != nil) {
		return nil, session.ErrDenied
	}
	if api == nil || clock == nil || len(issuer) != 32 || (c.Server && (p.ControllerKey != "" || p.TargetReady == nil)) || (!c.Server && (!hmacKey(p.ControllerKey) || port < 1 || port > 65535)) {
		return nil, session.ErrDenied
	}
	if !c.grantPolicy.Verify(issuer, c.Grant, c.Grant.Claims.Target, c.Grant.Claims.Controller, clock()) {
		return nil, session.ErrDenied
	}
	if !c.Server {
		d, e := descriptor(ctx, api, issuer, c, clock)
		if e != nil {
			return nil, e
		}
		child, e := startObservedTLS(ctx, p.Spec)
		if e != nil {
			return nil, e
		}
		bootstrap := map[string]any{"Port": port, "Key": p.ControllerKey, "Fingerprint": d.Claims.Fingerprint, "Target": c.Grant.Claims.Target}
		expected := ""
		if c.grantPolicy.IsDesktop() {
			expected = c.grantPolicy.Permission()
			bootstrap["Permission"] = expected
		}
		raw, e := child.Exchange(ctx, bootstrap)
		var ready struct {
			Status     string `json:"status"`
			Permission string `json:"Permission"`
		}
		if e != nil || decodeProductReady(raw, &ready) != nil || ready.Status != "ready" || ready.Permission != expected {
			return child, session.ErrDenied
		}
		p.Observation.bind(child, false)
		return child, nil
	}
	spec := p.Spec
	if p.CertificateScope != "" {
		var err error
		spec, err = certificateSpec(spec, p.CertificateScope, p.CertificateReopen)
		if err != nil {
			return nil, err
		}
	} else if p.CertificateReopen {
		return nil, session.ErrDenied
	}
	child, e := startObservedTLS(ctx, spec)
	if e != nil {
		return nil, e
	}
	raw, e := child.Exchange(ctx, nil)
	var ready productReady
	expected := ""
	if c.grantPolicy.IsDesktop() {
		expected = c.grantPolicy.Permission()
	}
	if e != nil || decodeProductReady(raw, &ready) != nil || ready.Status != "ready" || ready.Permission != expected || !hmacKey(ready.Key) {
		if p.CertificateScope != "" {
			return child, errors.Join(session.ErrDenied, ErrTestCertificateOpen)
		}
		return child, session.ErrDenied
	}
	if _, e = key(ready.Fingerprint); e != nil {
		return child, e
	}
	if p.CertificateScope != "" && p.CertificateOpened != nil {
		p.CertificateOpened(p.CertificateScope)
	}
	n, e := nonce()
	if e != nil {
		return child, session.ErrDenied
	}
	raw, e = child.Exchange(ctx, map[string]string{"Op": "certificate_proof", "GrantID": c.Grant.Claims.ID, "Target": c.Grant.Claims.Target, "Challenge": n})
	var proof struct {
		Status                        string `json:"status"`
		Certificate, CertificateProof string
	}
	if e != nil || json.Unmarshal(raw, &proof) != nil || proof.Status != "certificate_proof" {
		return child, session.ErrDenied
	}
	der, e := base64.StdEncoding.DecodeString(proof.Certificate)
	if e != nil || len(der) > 4096 || base64.StdEncoding.EncodeToString(der) != proof.Certificate {
		return child, session.ErrDenied
	}
	fp := sha256.Sum256(der)
	if hex.EncodeToString(fp[:]) != ready.Fingerprint {
		return child, session.ErrDenied
	}
	// The authority verifies the actual RSA private-key proof under the target
	// device's OS signature. No certificate private key is copied to this helper.
	if _, e = api.Request(ctx, invite.NetworkRequestBody{Action: "publish_tls", Grant: c.Grant, Challenge: n, Certificate: proof.Certificate, CertificateProof: proof.CertificateProof}); e != nil {
		return child, errors.Join(session.ErrDenied, ErrTLSCertificatePublication)
	}
	d, e := descriptor(ctx, api, issuer, c, clock)
	if e != nil || d.Claims.Fingerprint != ready.Fingerprint {
		return child, session.ErrDenied
	}
	p.Observation.bind(child, true)
	p.TargetReady(ready.Key, ready.Fingerprint)
	return child, nil
}

func startObservedTLS(ctx context.Context, spec processowner.Spec) (*processowner.Child, error) {
	w := cleanupwitness.From(ctx)
	if w == nil {
		return processowner.Start(spec)
	}
	w.Begin(cleanupwitness.TLSProduct)
	return processowner.StartObserved(spec, func(e error) { w.Complete(cleanupwitness.TLSProduct, e) })
}

func decodeProductReady(raw []byte, out any) error {
	return privatepipe.NewReader(bytes.NewReader(append(append([]byte(nil), raw...), '\n'))).Read(out)
}

// StartBoundProduct is trusted composition for an already-authorized native
// owner. It does not enroll a node or choose a destination; its caller must own
// the session transport and join the returned child on every outcome.
func StartBoundProduct(ctx context.Context, p Product, api ProductNetworkAPI, issuer ed25519.PublicKey, grant invite.Grant, binding invite.NetworkBinding, server bool, clock func() time.Time, port int, expectedFingerprint string) (*processowner.Child, error) {
	return startBoundProduct(ctx, invite.GrantPolicy{}, p, api, issuer, grant, binding, server, clock, port, expectedFingerprint)
}

// StartBoundDesktopProduct is separate trusted composition. The exact desktop
// permission is selected locally, then verified against the signed grant and
// descriptor before any private handoff. Child readiness must echo that scope;
// a legacy PONG product cannot stand in for a desktop product. The caller still
// owns pre-consent, the live gate and joining this same child on every outcome.
func StartBoundDesktopProduct(ctx context.Context, expected invite.DesktopPermission, p Product, api ProductNetworkAPI, issuer ed25519.PublicKey, grant invite.Grant, binding invite.NetworkBinding, server bool, clock func() time.Time, port int, expectedFingerprint string) (*processowner.Child, error) {
	policy, e := invite.DesktopGrantPolicy(expected)
	if e != nil || p.Observation != nil {
		return nil, session.ErrDenied
	}
	return startBoundProduct(ctx, policy, p, api, issuer, grant, binding, server, clock, port, expectedFingerprint)
}

func startBoundProduct(ctx context.Context, policy invite.GrantPolicy, p Product, api ProductNetworkAPI, issuer ed25519.PublicKey, grant invite.Grant, binding invite.NetworkBinding, server bool, clock func() time.Time, port int, expectedFingerprint string) (*processowner.Child, error) {
	if ctx == nil || api == nil {
		return nil, session.ErrDenied
	}
	if expectedFingerprint != "" {
		if _, err := key(expectedFingerprint); err != nil {
			return nil, session.ErrDenied
		}
		api = pinnedProductAPI{ProductNetworkAPI: api, fingerprint: expectedFingerprint}
	}
	return p.start(ctx, api, issuer, Config{grantPolicy: policy, Grant: grant, Binding: binding, Server: server}, clock, port)
}

type pinnedProductAPI struct {
	ProductNetworkAPI
	fingerprint string
}

func (p pinnedProductAPI) Request(ctx context.Context, request invite.NetworkRequestBody) (invite.NetworkReply, error) {
	reply, err := p.ProductNetworkAPI.Request(ctx, request)
	if err != nil {
		return invite.NetworkReply{}, err
	}
	if request.Action == "tls_identity" && (reply.TLS == nil || reply.TLS.Claims.Fingerprint != p.fingerprint) {
		return invite.NetworkReply{}, session.ErrDenied
	}
	return reply, nil
}
