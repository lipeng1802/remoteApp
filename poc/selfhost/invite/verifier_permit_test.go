package invite

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"strings"
	"testing"
	"time"
)

func TestVerifierPermitSignedOwnershipFreshnessAndRevocation(t *testing.T) {
	f, g, b, _ := provisionFixture(t)
	ticket := enroll(t, f, g, b, f.owner, "a")
	verifier := "nodekey:" + strings.Repeat("c", 64)
	b.VerifierSource = func(context.Context) (string, error) { return verifier, nil }
	device := deviceID(f.owner.Public().(ed25519.PublicKey))
	nonce := strings.Repeat("d", 64)
	p, e := b.VerifierPermit(context.Background(), g, device, ticket, nonce)
	if e != nil || !VerifyVerifierPermit(f.s.PublicKey(), p, g, device, ticket, "nodekey:"+strings.Repeat("a", 64), nonce, f.now) {
		t.Fatal("valid verifier permit")
	}
	for name, edit := range map[string]func(*VerifierClaims){
		"ticket": func(c *VerifierClaims) { c.Ticket = strings.Repeat("1", 32) }, "node": func(c *VerifierClaims) { c.NodeKey = verifier },
		"grant": func(c *VerifierClaims) { c.GrantID = strings.Repeat("1", 32) }, "device": func(c *VerifierClaims) { c.Device = g.Claims.Controller },
		"challenge": func(c *VerifierClaims) { c.Challenge = strings.Repeat("1", 64) }, "time": func(c *VerifierClaims) { c.ExpiresAt = c.IssuedAt + 31 },
	} {
		t.Run(name, func(t *testing.T) {
			bad := p
			edit(&bad.Claims)
			if VerifyVerifierPermit(f.s.PublicKey(), bad, g, device, ticket, "nodekey:"+strings.Repeat("a", 64), nonce, f.now) {
				t.Fatal("mutated permit accepted")
			}
		})
	}
	if VerifyVerifierPermit(f.s.PublicKey(), p, g, device, ticket, "nodekey:"+strings.Repeat("a", 64), nonce, time.Unix(p.Claims.ExpiresAt, 0)) {
		t.Fatal("expired permit")
	}
	if _, e = b.VerifierPermit(context.Background(), g, g.Claims.Controller, ticket, nonce); e == nil {
		t.Fatal("foreign owner permit")
	}
	b.VerifierSource = nil
	if _, e = b.VerifierPermit(context.Background(), g, device, ticket, nonce); e == nil {
		t.Fatal("missing live source")
	}
	b.VerifierSource = func(context.Context) (string, error) { return verifier, nil }
	if e = b.Bind(context.Background(), ticket); e != nil {
		t.Fatal("fixture bind")
	}
	if _, e = b.VerifierPermit(context.Background(), g, device, ticket, nonce); e == nil {
		t.Fatal("already bound ticket reissued")
	}
}

func TestVerifierPermitNetworkSignatureReplayAndNoCallerKey(t *testing.T) {
	f, g, b, _ := provisionFixture(t)
	ticket := enroll(t, f, g, b, f.owner, "a")
	b.VerifierSource = func(context.Context) (string, error) { return "nodekey:" + strings.Repeat("c", 64), nil }
	req := apiEnvelope(f, f.owner, g, "verifier")
	req.Body.Ticket = ticket
	req.Body.Challenge = strings.Repeat("d", 64)
	req = SignNetworkRequest(f.owner, req.Body)
	reply, e := b.ExecuteNetworkRequest(context.Background(), req)
	if e != nil || reply.Verifier == nil || reply.Credential != nil {
		t.Fatal("signed verifier request")
	}
	if _, e = b.ExecuteNetworkRequest(context.Background(), req); e == nil {
		t.Fatal("replay accepted")
	}
	req.Body.Nonce = strings.Repeat("e", 32)
	req.Body.Intent.PublicKey = hex.EncodeToString(f.owner.Public().(ed25519.PublicKey))
	req = SignNetworkRequest(f.owner, req.Body)
	if _, e = b.ExecuteNetworkRequest(context.Background(), req); e == nil {
		t.Fatal("caller intent injected into verifier request")
	}
}
