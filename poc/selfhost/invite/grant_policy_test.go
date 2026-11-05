package invite

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func policyFixture(t *testing.T, scope DesktopPermission) (*fixture, Grant, GrantPolicy, NetworkBinding, ed25519.PrivateKey) {
	t.Helper()
	f := desktopFixture(t)
	r := scopedClaim(t, f, scope)
	g := *f.exec(t, f.owner, Body{Action: "approve", RequestID: r.RequestID, Permission: scope, Allow: true}).Grant
	p, err := DesktopGrantPolicy(scope)
	if err != nil {
		t.Fatal(err)
	}
	b := NetworkBinding{TargetNode: "nodekey:" + strings.Repeat("a", 64), ControllerNode: "nodekey:" + strings.Repeat("b", 64), TargetIP: "100.120.0.42", ControllerIP: "100.120.0.93"}
	key, _ := hex.DecodeString(f.s.state.PrivateKey)
	return f, g, p, b, ed25519.PrivateKey(key)
}

func TestDesktopGrantPolicyCannotBeSelectedByJSONOrUnknownScope(t *testing.T) {
	for _, scope := range []DesktopPermission{"", "fixed-payload", "clipboard", "desktop-control "} {
		if _, e := DesktopGrantPolicy(scope); e != ErrDenied {
			t.Fatal("invalid compiled policy")
		}
	}
	p, _ := DesktopGrantPolicy(DesktopControl)
	for _, raw := range []string{`{}`, `{"desktop":"desktop-view"}`, `"desktop-control"`, `null`} {
		if json.Unmarshal([]byte(raw), &p) == nil {
			t.Fatal("IPC selected policy")
		}
		if p.Permission() != string(DesktopControl) {
			t.Fatal("failed decode changed local policy")
		}
	}
	if _, e := json.Marshal(p); e == nil {
		t.Fatal("policy serialized into IPC")
	}
	if (GrantPolicy{}).Permission() != permission || (GrantPolicy{}).IsDesktop() {
		t.Fatal("legacy default broadened")
	}
}

func TestDesktopPolicyPinsTLSIdentitySessionAndVerifierProofs(t *testing.T) {
	for _, scope := range []DesktopPermission{DesktopView, DesktopControl} {
		t.Run(string(scope), func(t *testing.T) {
			f, g, p, b, key := policyFixture(t, scope)
			wrongScope := DesktopView
			if scope == DesktopView {
				wrongScope = DesktopControl
			}
			wrong, _ := DesktopGrantPolicy(wrongScope)
			challenge := strings.Repeat("c", 64)
			start, end := f.now.UnixNano(), f.now.Add(3*time.Second).UnixNano()
			tls := TLSIdentity{Claims: TLSIdentityClaims{Version: 1, Grant: g, Binding: b, Fingerprint: strings.Repeat("d", 64), Challenge: challenge, IssuedAt: start, ExpiresAt: end}}
			tls.Signature = hex.EncodeToString(ed25519.Sign(key, tlsIdentityBytes(tls.Claims)))
			id := SessionIdentity{Claims: SessionIdentityClaims{Version: 1, Grant: g, Binding: b, TargetPublic: hex.EncodeToString(f.owner.Public().(ed25519.PublicKey)), ControllerPublic: hex.EncodeToString(f.client.Public().(ed25519.PublicKey)), Challenge: challenge, IssuedAt: start, ExpiresAt: end}}
			id.Signature = hex.EncodeToString(ed25519.Sign(key, sessionIdentityBytes(id.Claims)))
			ticket := strings.Repeat("e", 32)
			v := VerifierPermit{Claims: VerifierClaims{Version: 1, GrantID: g.Claims.ID, Device: g.Claims.Target, Ticket: ticket, NodeKey: b.TargetNode, VerifierKey: "nodekey:" + strings.Repeat("f", 64), Challenge: challenge, IssuedAt: f.now.Unix(), ExpiresAt: f.now.Add(30 * time.Second).Unix()}}
			v.Signature = hex.EncodeToString(ed25519.Sign(key, verifierBytes(v.Claims)))
			for _, policy := range []GrantPolicy{p, wrong, {}} {
				want := policy == p
				if VerifyTLSIdentityForPolicy(policy, f.s.PublicKey(), tls, g, b, challenge, f.now) != want ||
					VerifySessionIdentityForPolicy(policy, f.s.PublicKey(), id, g, b, challenge, f.now) != want ||
					VerifyVerifierPermitForPolicy(policy, f.s.PublicKey(), v, g, g.Claims.Target, ticket, b.TargetNode, challenge, f.now) != want {
					t.Fatal("proof scope substitution or valid proof rejected")
				}
			}
			if VerifyTLSIdentity(f.s.PublicKey(), tls, g, b, challenge, f.now) || VerifySessionIdentity(f.s.PublicKey(), id, g, b, challenge, f.now) || VerifyVerifierPermit(f.s.PublicKey(), v, g, g.Claims.Target, ticket, b.TargetNode, challenge, f.now) {
				t.Fatal("legacy proof accepted desktop scope")
			}
			for _, now := range []time.Time{f.now.Add(-time.Nanosecond), f.now.Add(3 * time.Second)} {
				if VerifyTLSIdentityForPolicy(p, f.s.PublicKey(), tls, g, b, challenge, now) || VerifySessionIdentityForPolicy(p, f.s.PublicKey(), id, g, b, challenge, now) {
					t.Fatal("proof lease widened")
				}
			}
			if VerifyTLSIdentityForPolicy(p, f.s.PublicKey(), tls, g, b, strings.Repeat("0", 64), f.now) || VerifySessionIdentityForPolicy(p, f.s.PublicKey(), id, g, b, strings.Repeat("0", 64), f.now) {
				t.Fatal("challenge substituted")
			}
			bad := tls
			bad.Claims.Fingerprint = strings.Repeat("0", 64)
			if VerifyTLSIdentityForPolicy(p, f.s.PublicKey(), bad, g, b, challenge, f.now) {
				t.Fatal("pin tampering accepted")
			}
		})
	}
}
