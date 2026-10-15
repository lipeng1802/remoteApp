package invite

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"strings"
	"testing"
	"time"
)

func TestSessionIdentityBindsRegisteredKeysGrantAndActiveNetwork(t *testing.T) {
	f, g, b, _ := provisionFixture(t)
	bindPair(t, f, g, b)
	challenge := strings.Repeat("c", 64)
	d, err := b.SessionIdentity(g, challenge)
	if err != nil || !VerifySessionIdentity(f.s.PublicKey(), d, g, d.Claims.Binding, challenge, f.now) {
		t.Fatal("valid session identity")
	}
	if d.Claims.TargetPublic != hex.EncodeToString(f.owner.Public().(ed25519.PublicKey)) || d.Claims.ControllerPublic != hex.EncodeToString(f.client.Public().(ed25519.PublicKey)) {
		t.Fatal("registered public keys not bound")
	}
	for _, mode := range []string{"target", "controller", "binding", "challenge", "signature", "grant", "expired"} {
		bad := d
		now := f.now
		switch mode {
		case "target":
			bad.Claims.TargetPublic = hex.EncodeToString(f.other.Public().(ed25519.PublicKey))
		case "controller":
			bad.Claims.ControllerPublic = hex.EncodeToString(f.other.Public().(ed25519.PublicKey))
		case "binding":
			bad.Claims.Binding.ControllerIP = "100.120.0.9"
		case "challenge":
			bad.Claims.Challenge = strings.Repeat("d", 64)
		case "signature":
			bad.Signature = strings.Repeat("0", 128)
		case "grant":
			bad.Claims.Grant.Claims.ID = strings.Repeat("e", 32)
		case "expired":
			now = time.Unix(0, d.Claims.ExpiresAt)
		}
		if VerifySessionIdentity(f.s.PublicKey(), bad, g, d.Claims.Binding, challenge, now) {
			t.Fatal("tampered session identity", mode)
		}
	}
	f.exec(t, f.owner, Body{Action: "revoke", GrantID: g.Claims.ID})
	if _, err = b.SessionIdentity(g, challenge); err == nil {
		t.Fatal("revoked grant disclosed session keys")
	}
}

func TestSessionIdentitySignedAPIAndStrictReplyShape(t *testing.T) {
	f, g, b, _ := provisionFixture(t)
	bindPair(t, f, g, b)
	challenge := strings.Repeat("a", 64)
	req := apiEnvelope(f, f.owner, g, "session_identity")
	req.Body.Challenge = challenge
	req = SignNetworkRequest(f.owner, req.Body)
	reply, err := b.ExecuteNetworkRequest(context.Background(), req)
	if err != nil || reply.Session == nil || !VerifySessionIdentity(f.s.PublicKey(), *reply.Session, g, reply.Session.Claims.Binding, challenge, f.now) {
		t.Fatal("signed session identity API")
	}
	bad := req
	bad.Body.Challenge = ""
	bad = SignNetworkRequest(f.owner, bad.Body)
	if _, err = b.ExecuteNetworkRequest(context.Background(), bad); err == nil {
		t.Fatal("missing challenge")
	}
	foreign := apiEnvelope(f, f.other, g, "session_identity")
	foreign.Body.Challenge = challenge
	foreign = SignNetworkRequest(f.other, foreign.Body)
	if _, err = b.ExecuteNetworkRequest(context.Background(), foreign); err == nil {
		t.Fatal("foreign device read session identity")
	}
}
