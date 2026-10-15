package installeddevice

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/trustprofile"
)

type requestFunc func(context.Context, invite.NetworkRequestBody) (invite.NetworkReply, error)

func (f requestFunc) Request(c context.Context, b invite.NetworkRequestBody) (invite.NetworkReply, error) {
	return f(c, b)
}

func testSignature(k ed25519.PrivateKey, prefix string, v any) string {
	b, _ := json.Marshal(v)
	return hex.EncodeToString(ed25519.Sign(k, append([]byte(prefix), b...)))
}

func authorizationFixture(t *testing.T) (*authorizationGate, invite.Grant, ed25519.PrivateKey, requestFunc) {
	t.Helper()
	issuer, k, _ := ed25519.GenerateKey(rand.Reader)
	_, device, _ := ed25519.GenerateKey(rand.Reader)
	signer := &testSigner{key: device, joined: make(chan struct{})}
	id := sha256.Sum256(signer.PublicKey())
	now := time.Now().Unix()
	g := invite.Grant{Claims: invite.Claims{Version: 1, ID: strings.Repeat("a", 32), Target: hex.EncodeToString(id[:]), Controller: strings.Repeat("b", 64), Permission: "fixed-payload", IssuedAt: now - 1, ExpiresAt: now + 299}}
	g.Signature = testSignature(k, "remoteapp-invite-grant-v1\n", g.Claims)
	gate := newAuthorizationGate(trustprofile.Profile{Claims: trustprofile.Claims{Issuer: hex.EncodeToString(issuer)}}, signer)
	request := requestFunc(func(ctx context.Context, b invite.NetworkRequestBody) (invite.NetworkReply, error) {
		if ctx.Err() != nil {
			return invite.NetworkReply{}, ctx.Err()
		}
		if b.Action == "grant_clock" {
			p := invite.ClockProof{Claims: invite.ClockClaims{Version: 1, GrantID: b.Grant.Claims.ID, Nonce: b.Challenge, IssuedAt: time.Now().UnixNano()}}
			p.Signature = testSignature(k, "remoteapp-clock-proof-v1\n", p.Claims)
			return invite.NetworkReply{Proof: &p}, nil
		}
		now := time.Now().UnixNano()
		n := invite.NetworkState{Claims: invite.NetworkClaims{Version: 1, Grant: b.Grant, Binding: invite.NetworkBinding{TargetNode: "nodekey:" + strings.Repeat("1", 64), ControllerNode: "nodekey:" + strings.Repeat("2", 64), TargetIP: "100.120.0.1", ControllerIP: "100.120.0.2"}, Active: true, IssuedAt: now, ExpiresAt: now + int64(3*time.Second)}}
		n.Signature = testSignature(k, "remoteapp-network-state-v1\n", n.Claims)
		return invite.NetworkReply{State: &n}, nil
	})
	return gate, g, k, request
}

func TestAuthorizationClockThenStateAndNoPrivateReply(t *testing.T) {
	gate, g, _, request := authorizationFixture(t)
	for _, action := range []string{"grant_clock", "state"} {
		r, err := gate.request(context.Background(), request, invite.NetworkRequestBody{Action: action, Grant: g})
		if err != nil || !r.Passed || r.Active != (action == "state") || r.Phase != action {
			t.Fatal("authorization sequence rejected")
		}
		raw, _ := json.Marshal(r)
		for _, secret := range []string{g.Claims.ID, g.Signature, "credential", "proof", "binding", "nodekey", "ticket"} {
			if strings.Contains(string(raw), secret) {
				t.Fatal("private reply escaped gate")
			}
		}
	}
}

func TestAuthorizationCancellationAndClockNonceReplayCannotPass(t *testing.T) {
	gate, g, _, good := authorizationFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	request := requestFunc(func(c context.Context, b invite.NetworkRequestBody) (invite.NetworkReply, error) {
		r, err := good(c, b)
		cancel()
		return r, err
	})
	if result, err := gate.request(ctx, request, invite.NetworkRequestBody{Action: "grant_clock", Grant: g}); err == nil || result.Passed || gate.clock != nil {
		t.Fatal("cancelled result accepted")
	}
	gate, g, _, good = authorizationFixture(t)
	var previous invite.NetworkReply
	request = requestFunc(func(c context.Context, b invite.NetworkRequestBody) (invite.NetworkReply, error) {
		if previous.Proof == nil {
			previous, _ = good(c, b)
		}
		return previous, nil
	})
	b := invite.NetworkRequestBody{Action: "grant_clock", Grant: g}
	if _, err := gate.request(context.Background(), request, b); err != nil {
		t.Fatal(err)
	}
	if result, err := gate.request(context.Background(), request, b); err == nil || result.Passed || gate.clock != nil {
		t.Fatal("old proof accepted for fresh challenge")
	}
}

func TestAuthorizationRejectsStateBeforeClockAndInvalidGrantWithoutHTTP(t *testing.T) {
	for _, scenario := range []string{"state_first", "wrong_subject", "wrong_signature", "same_subjects", "injected_challenge", "enroll"} {
		t.Run(scenario, func(t *testing.T) {
			gate, g, k, _ := authorizationFixture(t)
			b := invite.NetworkRequestBody{Action: "grant_clock", Grant: g}
			switch scenario {
			case "state_first":
				b.Action = "state"
			case "wrong_subject":
				b.Grant.Claims.Target = strings.Repeat("c", 64)
				b.Grant.Signature = testSignature(k, "remoteapp-invite-grant-v1\n", b.Grant.Claims)
			case "wrong_signature":
				b.Grant.Signature = strings.Repeat("0", 128)
			case "same_subjects":
				b.Grant.Claims.Controller = b.Grant.Claims.Target
				b.Grant.Signature = testSignature(k, "remoteapp-invite-grant-v1\n", b.Grant.Claims)
			case "injected_challenge":
				b.Challenge = "parent_nonce"
			case "enroll":
				b.Action = "enroll"
			}
			_, err := gate.request(context.Background(), requestFunc(func(context.Context, invite.NetworkRequestBody) (invite.NetworkReply, error) {
				t.Fatal("invalid request reached HTTP")
				return invite.NetworkReply{}, nil
			}), b)
			if err == nil {
				t.Fatal("invalid authorization accepted")
			}
		})
	}
}

func TestAuthorizationRejectsClockReplayWrongIssuerAndPrivateFields(t *testing.T) {
	for _, scenario := range []string{"wrong_nonce", "wrong_signature", "wrong_issuer", "wrong_grant", "credential", "ticket", "expired_grant", "identity_change"} {
		t.Run(scenario, func(t *testing.T) {
			gate, g, k, good := authorizationFixture(t)
			if scenario == "expired_grant" {
				g.Claims.IssuedAt = time.Now().Unix() - 301
				g.Claims.ExpiresAt = g.Claims.IssuedAt + 300
				g.Signature = testSignature(k, "remoteapp-invite-grant-v1\n", g.Claims)
			}
			bad := requestFunc(func(ctx context.Context, b invite.NetworkRequestBody) (invite.NetworkReply, error) {
				r, _ := good(ctx, b)
				switch scenario {
				case "wrong_nonce":
					r.Proof.Claims.Nonce = strings.Repeat("3", 64)
					r.Proof.Signature = testSignature(k, "remoteapp-clock-proof-v1\n", r.Proof.Claims)
				case "wrong_signature":
					r.Proof.Signature = strings.Repeat("0", 128)
				case "wrong_issuer":
					_, other, _ := ed25519.GenerateKey(rand.Reader)
					r.Proof.Signature = testSignature(other, "remoteapp-clock-proof-v1\n", r.Proof.Claims)
				case "identity_change":
					_, other, _ := ed25519.GenerateKey(rand.Reader)
					gate.signer.(*testSigner).key = other
				case "wrong_grant":
					r.Proof.Claims.GrantID = strings.Repeat("4", 32)
					r.Proof.Signature = testSignature(k, "remoteapp-clock-proof-v1\n", r.Proof.Claims)
				case "credential":
					r.Credential = &invite.RegistrationCredential{Secret: "TEST_SECRET"}
				case "ticket":
					r.Ticket = "TEST_SECRET"
				}
				return r, nil
			})
			r, err := gate.request(context.Background(), bad, invite.NetworkRequestBody{Action: "grant_clock", Grant: g})
			if err == nil || r.Passed || gate.clock != nil {
				t.Fatal("untrusted clock/private reply accepted")
			}
		})
	}
}

func TestSignedInactiveStateIsTerminalAndFaultClearsClock(t *testing.T) {
	for _, scenario := range []string{"inactive", "signature", "lease", "different_grant", "transport"} {
		t.Run(scenario, func(t *testing.T) {
			gate, g, k, good := authorizationFixture(t)
			if _, err := gate.request(context.Background(), good, invite.NetworkRequestBody{Action: "grant_clock", Grant: g}); err != nil {
				t.Fatal(err)
			}
			bad := requestFunc(func(ctx context.Context, b invite.NetworkRequestBody) (invite.NetworkReply, error) {
				if scenario == "transport" {
					return invite.NetworkReply{}, invite.ErrDenied
				}
				r, _ := good(ctx, b)
				switch scenario {
				case "inactive":
					r.State.Claims.Active = false
					r.State.Signature = testSignature(k, "remoteapp-network-state-v1\n", r.State.Claims)
				case "signature":
					r.State.Signature = strings.Repeat("0", 128)
				case "lease":
					r.State.Claims.ExpiresAt += int64(time.Second)
					r.State.Signature = testSignature(k, "remoteapp-network-state-v1\n", r.State.Claims)
				case "different_grant":
					r.State.Claims.Grant.Claims.ID = strings.Repeat("5", 32)
					r.State.Signature = testSignature(k, "remoteapp-network-state-v1\n", r.State.Claims)
				}
				return r, nil
			})
			r, err := gate.request(context.Background(), bad, invite.NetworkRequestBody{Action: "state", Grant: g})
			if err == nil || r.Active || gate.clock != nil || r.Passed != (scenario == "inactive") {
				t.Fatal("state fault not retired")
			}
		})
	}
}

func TestLocalNetworkCommandsCannotInjectCredentialsOrAuthority(t *testing.T) {
	_, g, _, _ := authorizationFixture(t)
	for _, action := range []string{"grant_clock", "state"} {
		if !validCommand(Request{Op: "network", Network: invite.NetworkRequestBody{Action: action, Grant: g}}) {
			t.Fatal("read-only action rejected")
		}
	}
	for _, action := range []string{"enroll", "bind", "publish_tls", "tls_identity", "session_identity", "clock"} {
		if validCommand(Request{Op: "network", Network: invite.NetworkRequestBody{Action: action, Grant: g}}) {
			t.Fatal("secret-bearing/unbound action accepted")
		}
	}
	for _, field := range []string{"Nonce", "Challenge", "Ticket", "Certificate"} {
		b := invite.NetworkRequestBody{Action: "grant_clock", Grant: g}
		switch field {
		case "Nonce":
			b.Nonce = "parent"
		case "Challenge":
			b.Challenge = "parent"
		case "Ticket":
			b.Ticket = "parent"
		case "Certificate":
			b.Certificate = "parent"
		}
		if validCommand(Request{Op: "network", Network: b}) {
			t.Fatal("parent injection accepted")
		}
	}
	if validCommand(Request{Op: "stop", Network: invite.NetworkRequestBody{Action: "state", Grant: g}}) || ValidOpen(Request{Op: "open", VaultScope: strings.Repeat("a", 32), ExpectedPublic: strings.Repeat("b", 64), Network: invite.NetworkRequestBody{Action: "state", Grant: g}}) {
		t.Fatal("mixed open/stop")
	}
	raw, _ := json.Marshal(Request{Op: "open", VaultScope: strings.Repeat("a", 32), ExpectedPublic: strings.Repeat("b", 64)})
	if strings.Contains(string(raw), "Network") {
		t.Fatal("zero network field broke legacy device frames")
	}
}
