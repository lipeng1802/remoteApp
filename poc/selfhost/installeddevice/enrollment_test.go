package installeddevice

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/clientruntime"
	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/nodeidentity"
)

type enrollmentNodeFixture struct {
	key, address, consumed string
	registers, closes      int
	err                    error
	join                   chan struct{}
	onRegister             func()
}

func (n *enrollmentNodeFixture) PublicKey() string { return n.key }
func (n *enrollmentNodeFixture) Register(ctx context.Context, secret string) (string, string, error) {
	n.registers++
	n.consumed = secret
	if n.onRegister != nil {
		n.onRegister()
	}
	return n.key, n.address, n.err
}
func (n *enrollmentNodeFixture) CloseJoined() error {
	if n.join != nil {
		<-n.join
	}
	n.closes++
	return n.err
}
func nodeFixture() *enrollmentNodeFixture {
	return &enrollmentNodeFixture{key: "nodekey:" + strings.Repeat("1", 64), address: "100.120.0.7"}
}
func enrollmentReply() invite.NetworkReply {
	return invite.NetworkReply{Ticket: strings.Repeat("e", 32), Credential: &invite.RegistrationCredential{UserID: "fixture-user", KeyID: "fixture-key", Secret: "fixture-registration-secret"}}
}
func calibratedFixture(t *testing.T) (*authorizationGate, invite.Grant) {
	t.Helper()
	g, grant, _, req := authorizationFixture(t)
	if _, err := g.request(context.Background(), req, invite.NetworkRequestBody{Action: "grant_clock", Grant: grant}); err != nil {
		t.Fatal(err)
	}
	return g, grant
}

func TestEnrollmentInternalIntentCredentialAndOneShot(t *testing.T) {
	g, grant := calibratedFixture(t)
	n := nodeFixture()
	r := enrollmentReply()
	requests := 0
	client := requestFunc(func(ctx context.Context, b invite.NetworkRequestBody) (invite.NetworkReply, error) {
		requests++
		intent := b.Intent
		raw, _ := json.Marshal(intent.Body)
		sig, _ := hex.DecodeString(intent.Signature)
		if b.Action != "enroll" || b.Grant != grant || b.Ticket != "" || b.Challenge != "" || intent.Body.NodeKey != n.key || intent.Body.GrantID != grant.Claims.ID || !canonical(intent.Body.Nonce, 16) || intent.Body.IssuedAt != g.clock().Unix() || intent.PublicKey != hex.EncodeToString(g.public) || !ed25519.Verify(g.public, append([]byte("remoteapp-provision-intent-v1\n"), raw...), sig) {
			t.Error("internal intent invalid")
		}
		return r, nil
	})
	var e enrollment
	f := func(context.Context) (EnrollmentNode, error) { return n, nil }
	result, err := e.start(context.Background(), g, client, f, grant)
	if err != nil || !result.Passed || result.Active || n.registers != 1 || n.consumed != "fixture-registration-secret" || *r.Credential != (invite.RegistrationCredential{}) || e.ticket != r.Ticket {
		t.Fatal("credential consumption failed")
	}
	raw, _ := json.Marshal(result)
	for _, value := range []string{n.consumed, r.Ticket, n.key, n.address, grant.Signature, "credential", "ticket", "binding"} {
		if strings.Contains(string(raw), value) {
			t.Fatal("private enrollment data escaped")
		}
	}
	if _, err := e.start(context.Background(), g, client, f, grant); err == nil || requests != 1 || n.registers != 1 {
		t.Fatal("attempt replayed")
	}
	if e.closeJoined() != nil || n.closes != 1 || e.ticket != "" {
		t.Fatal("node not joined")
	}
}

func TestEnrollmentRejectsPreconditionsBeforeNodeOrHTTP(t *testing.T) {
	for _, scenario := range []string{"no_clock", "wrong_grant", "identity_changed", "canceled", "nil_factory", "expired"} {
		t.Run(scenario, func(t *testing.T) {
			g, grant := calibratedFixture(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			calls := 0
			f := EnrollmentFactory(func(context.Context) (EnrollmentNode, error) { calls++; return nodeFixture(), nil })
			switch scenario {
			case "no_clock":
				g.clock = nil
			case "wrong_grant":
				grant.Claims.ID = strings.Repeat("f", 32)
			case "identity_changed":
				_, g.signer.(*testSigner).key, _ = ed25519.GenerateKey(rand.Reader)
			case "canceled":
				cancel()
			case "nil_factory":
				f = nil
			case "expired":
				g.clock = func() time.Time { return time.Unix(grant.Claims.ExpiresAt, 0) }
			}
			var e enrollment
			client := requestFunc(func(context.Context, invite.NetworkRequestBody) (invite.NetworkReply, error) {
				calls++
				return enrollmentReply(), nil
			})
			if result, err := e.start(ctx, g, client, f, grant); err == nil || result.Passed || calls != 0 {
				t.Fatal("invalid precondition had effects")
			}
		})
	}
}

func TestEnrollmentUnknownOrMalformedReplyNeverConsumesOrRetries(t *testing.T) {
	for _, scenario := range []string{"transport", "missing", "oversize", "ticket", "mixed_state", "mixed_proof", "cancel", "identity_changed", "node_changed"} {
		t.Run(scenario, func(t *testing.T) {
			g, grant := calibratedFixture(t)
			n := nodeFixture()
			r := enrollmentReply()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			calls := 0
			client := requestFunc(func(context.Context, invite.NetworkRequestBody) (invite.NetworkReply, error) {
				calls++
				switch scenario {
				case "transport":
					return r, invite.ErrDenied
				case "missing":
					r.Credential = nil
				case "oversize":
					r.Credential.Secret = strings.Repeat("x", 257)
				case "ticket":
					r.Ticket = "BAD"
				case "mixed_state":
					r.State = &invite.NetworkState{}
				case "mixed_proof":
					r.Proof = &invite.ClockProof{}
				case "cancel":
					cancel()
				case "identity_changed":
					_, g.signer.(*testSigner).key, _ = ed25519.GenerateKey(rand.Reader)
				case "node_changed":
					n.key = "nodekey:" + strings.Repeat("2", 64)
				}
				return r, nil
			})
			var e enrollment
			f := func(context.Context) (EnrollmentNode, error) { return n, nil }
			if result, err := e.start(ctx, g, client, f, grant); err == nil || result.Passed || n.registers != 0 {
				t.Fatal("bad reply consumed")
			}
			if r.Credential != nil && *r.Credential != (invite.RegistrationCredential{}) {
				t.Fatal("rejected secret retained")
			}
			if _, err := e.start(context.Background(), g, client, f, grant); err == nil || calls != 1 {
				t.Fatal("unknown outcome retried")
			}
			if e.closeJoined() != nil || n.closes != 1 {
				t.Fatal("fault node not joined")
			}
		})
	}
}

func TestEnrollmentRejectsRegisteredIdentityAndExpiryAndJoins(t *testing.T) {
	for _, scenario := range []string{"bad_key", "foreign_ip", "reserved_ip", "register_error", "expired", "canceled"} {
		t.Run(scenario, func(t *testing.T) {
			g, grant := calibratedFixture(t)
			n := nodeFixture()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			switch scenario {
			case "bad_key":
				n.onRegister = func() { n.key = "nodekey:" + strings.Repeat("2", 64) }
			case "foreign_ip":
				n.address = "100.64.0.1"
			case "reserved_ip":
				n.address = "100.120.0.255"
			case "register_error":
				n.err = invite.ErrDenied
			case "expired":
				n.onRegister = func() { g.clock = func() time.Time { return time.Unix(grant.Claims.ExpiresAt, 0) } }
			case "canceled":
				n.onRegister = cancel
			}
			var e enrollment
			client := requestFunc(func(context.Context, invite.NetworkRequestBody) (invite.NetworkReply, error) {
				return enrollmentReply(), nil
			})
			if result, err := e.start(ctx, g, client, func(context.Context) (EnrollmentNode, error) { return n, nil }, grant); err == nil || result.Passed {
				t.Fatal("invalid registration accepted")
			}
			e.closeJoined()
			if n.closes != 1 {
				t.Fatal("registration fault skipped join")
			}
		})
	}
}

func TestTSNetEnrollmentOfflinePreparationRetainsIdentityAndRejectsPaths(t *testing.T) {
	dir := t.TempDir()
	os.Chmod(dir, 0700)
	f := TSNetEnrollmentFactory(dir)
	a, err := f(context.Background())
	if err != nil || !nodeKeyOK(a.PublicKey()) {
		t.Fatal("offline prepare failed")
	}
	if a.CloseJoined() != nil {
		t.Fatal("offline close failed")
	}
	b, err := nodeidentity.Prepare(dir, clientruntime.ControlURL)
	if err != nil || a.PublicKey() != b {
		t.Fatal("retained identity changed")
	}
	if _, err := f(context.Background()); err == nil {
		t.Fatal("retained outcome or old login reused for a fresh credential")
	}
	for _, path := range []string{"relative", filepath.Join(dir, "missing"), dir + string(filepath.Separator) + ".."} {
		if _, err := TSNetEnrollmentFactory(path)(context.Background()); err == nil {
			t.Fatal("unprepared path accepted")
		}
	}
	link := filepath.Join(t.TempDir(), "link")
	if os.Symlink(dir, link) == nil {
		if _, err := TSNetEnrollmentFactory(link)(context.Background()); err == nil {
			t.Fatal("symlink accepted")
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "tailscaled.state"), []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := f(context.Background()); err == nil {
		t.Fatal("corrupt state repaired")
	}
}

func TestEnrollmentPreparationFaultIsOwnedAndCannotRetry(t *testing.T) {
	for _, scenario := range []string{"factory_error", "invalid_key", "expires_during_prepare"} {
		t.Run(scenario, func(t *testing.T) {
			g, grant := calibratedFixture(t)
			n := nodeFixture()
			calls, prepares := 0, 0
			factory := func(context.Context) (EnrollmentNode, error) {
				prepares++
				switch scenario {
				case "factory_error":
					return n, invite.ErrDenied
				case "invalid_key":
					n.key = "nodekey:" + strings.Repeat("0", 64)
				case "expires_during_prepare":
					g.clock = func() time.Time { return time.Unix(grant.Claims.ExpiresAt, 0) }
				}
				return n, nil
			}
			client := requestFunc(func(context.Context, invite.NetworkRequestBody) (invite.NetworkReply, error) {
				calls++
				return enrollmentReply(), nil
			})
			var e enrollment
			if r, err := e.start(context.Background(), g, client, factory, grant); err == nil || r.Passed || calls != 0 {
				t.Fatal("preparation fault reached authority")
			}
			if _, err := e.start(context.Background(), g, client, factory, grant); err == nil || prepares != 1 {
				t.Fatal("preparation retried")
			}
			if e.closeJoined() != nil || n.closes != 1 {
				t.Fatal("returned partial resource not joined")
			}
		})
	}
}

func TestProvisionFrameCannotInjectCapabilityAndDefaultEntryRejects(t *testing.T) {
	_, grant := calibratedFixture(t)
	b := Request{Op: "provision", Network: invite.NetworkRequestBody{Grant: grant}}
	if !validProvision(b) || validCommand(b) || ValidOpen(b) {
		t.Fatal("provision opt-in boundary lost")
	}
	for _, scenario := range []string{"scope", "public", "device", "action", "nonce", "time", "challenge", "ticket", "intent", "certificate", "proof"} {
		v := b
		switch scenario {
		case "scope":
			v.VaultScope = strings.Repeat("a", 32)
		case "public":
			v.ExpectedPublic = strings.Repeat("a", 64)
		case "device":
			v.Device.Action = "register"
		case "action":
			v.Network.Action = "enroll"
		case "nonce":
			v.Network.Nonce = strings.Repeat("a", 32)
		case "time":
			v.Network.IssuedAt = 1
		case "challenge":
			v.Network.Challenge = strings.Repeat("a", 64)
		case "ticket":
			v.Network.Ticket = strings.Repeat("a", 32)
		case "intent":
			v.Network.Intent.Body.NodeKey = nodeFixture().key
		case "certificate":
			v.Network.Certificate = "injected"
		case "proof":
			v.Network.CertificateProof = "injected"
		}
		if validProvision(v) {
			t.Fatalf("injected %s accepted", scenario)
		}
	}
}
