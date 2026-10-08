package session

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/netip"
	"strings"
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/invite"
)

// This fixture tests session protocol/closure only, not server persistence.
// Issuer private material exists only in memory; no Unix Store on Windows.
func portableSetup(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{t: t}
	f.now.Store(time.Now().Unix())
	issuer, key, _ := ed25519.GenerateKey(rand.Reader)
	_, f.owner, _ = ed25519.GenerateKey(rand.Reader)
	_, f.client, _ = ed25519.GenerateKey(rand.Reader)
	c := invite.Claims{Version: 1, ID: strings.Repeat("a", 32), Target: Identity(f.owner.Public().(ed25519.PublicKey)), Controller: Identity(f.client.Public().(ed25519.PublicKey)), Permission: "fixed-payload", IssuedAt: f.now.Load(), ExpiresAt: f.now.Load() + 300}
	raw, _ := json.Marshal(c)
	g := invite.Grant{Claims: c, Signature: hex.EncodeToString(ed25519.Sign(key, append([]byte("remoteapp-invite-grant-v1\n"), raw...)))}
	f.server = Config{Issuer: issuer, Key: f.owner, Grant: g, Now: f.clock, Binding: Binding{Target: f.owner.Public().(ed25519.PublicKey), Controller: f.client.Public().(ed25519.PublicKey), TargetIP: netip.MustParseAddr("127.0.0.1"), ControllerIP: netip.MustParseAddr("127.0.0.1")}}
	f.probe = f.server
	f.probe.Key = f.client
	return f
}
func (f *fixture) portableCheck(ctx context.Context, g invite.Grant, target, controller string) bool {
	return ctx.Err() == nil && g == f.server.Grant && invite.VerifyGrant(f.server.Issuer, g, target, controller, f.clock())
}
