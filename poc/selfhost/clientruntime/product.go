package clientruntime

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"time"

	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/processowner"
	"remoteapp.local/selfhost-poc/session"
)

// Product is trusted runtime wiring for the actual Swift/C# TLS probes. The
// compiled command fixes Spec; only the TEST controller HMAC is parent-supplied
// through its anonymous pipe. This is not final product secret distribution.
type Product struct {
	Spec          processowner.Spec
	ControllerKey string
	TargetReady   func(key, fingerprint string) // private parent pipe; never log
}
type networkAPI interface {
	Request(context.Context, invite.NetworkRequestBody) (invite.NetworkReply, error)
}
type productReady struct {
	Status           string `json:"status"`
	Key, Fingerprint string
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
	if e != nil || r.TLS == nil || !invite.VerifyTLSIdentity(issuer, *r.TLS, c.Grant, c.Binding, n, clock()) {
		return invite.TLSIdentity{}, session.ErrDenied
	}
	return *r.TLS, nil
}
func (p Product) start(ctx context.Context, api networkAPI, issuer ed25519.PublicKey, c Config, clock func() time.Time, port int) (*processowner.Child, error) {
	// A bounded startup, but child cleanup is deliberately unbounded/joined by
	// the outer runner. Deadline failure cannot release the installation lease.
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	if api == nil || clock == nil || len(issuer) != 32 || (c.Server && (p.ControllerKey != "" || p.TargetReady == nil)) || (!c.Server && (!hmacKey(p.ControllerKey) || port < 1 || port > 65535)) {
		return nil, session.ErrDenied
	}
	if !c.Server {
		d, e := descriptor(ctx, api, issuer, c, clock)
		if e != nil {
			return nil, e
		}
		child, e := processowner.Start(p.Spec)
		if e != nil {
			return nil, e
		}
		raw, e := child.Exchange(ctx, map[string]any{"Port": port, "Key": p.ControllerKey, "Fingerprint": d.Claims.Fingerprint, "Target": c.Grant.Claims.Target})
		var ready struct {
			Status string `json:"status"`
		}
		if e != nil || json.Unmarshal(raw, &ready) != nil || ready.Status != "ready" {
			return child, session.ErrDenied
		}
		return child, nil
	}
	child, e := processowner.Start(p.Spec)
	if e != nil {
		return nil, e
	}
	raw, e := child.Exchange(ctx, nil)
	var ready productReady
	if e != nil || json.Unmarshal(raw, &ready) != nil || ready.Status != "ready" || !hmacKey(ready.Key) {
		return child, session.ErrDenied
	}
	if _, e = key(ready.Fingerprint); e != nil {
		return child, e
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
		return child, session.ErrDenied
	}
	d, e := descriptor(ctx, api, issuer, c, clock)
	if e != nil || d.Claims.Fingerprint != ready.Fingerprint {
		return child, session.ErrDenied
	}
	p.TargetReady(ready.Key, ready.Fingerprint)
	return child, nil
}
