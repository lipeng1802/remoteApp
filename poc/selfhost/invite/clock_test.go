package invite

import (
	"crypto/ed25519"
	"crypto/rand"
	"strings"
	"testing"
	"time"
)

func TestFreshAuthorityClockConservativeAndBound(t *testing.T) {
	f, g, b, _ := provisionFixture(t)
	bindPair(t, f, g, b)
	nonce := strings.Repeat("a", 64)
	p, e := b.ClockProof(g, nonce)
	if e != nil {
		t.Fatal(e)
	}
	// Simulate a helper wall clock years away: calibration uses authority time
	// and monotonic elapsed, not receipt-based lifetime or local Unix time.
	start := time.Now().Add(-100 * time.Millisecond)
	now, e := AuthorityClock(f.s.PublicKey(), p, g.Claims.ID, nonce, start)
	if e != nil || now().UnixNano() < p.Claims.IssuedAt+int64(100*time.Millisecond) || now().UnixNano() > p.Claims.IssuedAt+int64(time.Second) {
		t.Fatal("clock calibration")
	}
	for _, kind := range []string{"nonce", "grant", "signature", "issuer", "timeout"} {
		v := p
		wantNonce, wantGrant, pub, s := nonce, g.Claims.ID, f.s.PublicKey(), start
		switch kind {
		case "nonce":
			wantNonce = strings.Repeat("b", 64)
		case "grant":
			wantGrant = strings.Repeat("c", 32)
		case "signature":
			v.Claims.IssuedAt--
		case "issuer":
			pub, _, _ = ed25519.GenerateKey(rand.Reader)
		case "timeout":
			s = time.Now().Add(-6 * time.Second)
		}
		if _, e := AuthorityClock(pub, v, wantGrant, wantNonce, s); e == nil {
			t.Fatal("invalid clock proof accepted: " + kind)
		}
	}
	f.exec(t, f.owner, Body{Action: "revoke", GrantID: g.Claims.ID})
	if _, e := b.ClockProof(g, nonce); e == nil {
		t.Fatal("revoked authority clock proof")
	}
}
