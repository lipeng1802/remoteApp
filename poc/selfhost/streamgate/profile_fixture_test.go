package streamgate

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/session"
)

// Test-only ephemeral authority. Unlike the Linux server Store, it makes no
// durability/registration/replay/admission claim and writes no issuer key.
type profileFixture struct {
	issuer      ed25519.PublicKey
	key, target ed25519.PrivateKey
	signer      invite.DeviceSigner
	grant       invite.Grant
	binding     invite.NetworkBinding
	endpoint    string
	certificate []byte
	stall       atomic.Bool
	queries     atomic.Int64
}

func (f *profileFixture) sign(domain string, v any) string {
	b, _ := json.Marshal(v)
	return hex.EncodeToString(ed25519.Sign(f.key, append([]byte(domain+"\n"), b...)))
}
func (f *profileFixture) networkState() invite.NetworkState {
	now := time.Now().UnixNano()
	c := invite.NetworkClaims{Version: 1, Grant: f.grant, Binding: f.binding, Active: true, IssuedAt: now, ExpiresAt: now + int64(3*time.Second)}
	return invite.NetworkState{Claims: c, Signature: f.sign("remoteapp-network-state-v1", c)}
}
func portableProfileSetup(t *testing.T) *profileFixture {
	t.Helper()
	f := &profileFixture{}
	f.issuer, f.key, _ = ed25519.GenerateKey(rand.Reader)
	_, f.target, _ = ed25519.GenerateKey(rand.Reader)
	_, controller, _ := ed25519.GenerateKey(rand.Reader)
	f.signer, _ = invite.NewMemorySigner(controller)
	claims := invite.Claims{Version: 1, ID: strings.Repeat("a", 32), Target: session.Identity(f.target.Public().(ed25519.PublicKey)), Controller: session.Identity(f.signer.PublicKey()), Permission: "fixed-payload", IssuedAt: time.Now().Unix(), ExpiresAt: time.Now().Add(5 * time.Minute).Unix()}
	f.grant = invite.Grant{Claims: claims, Signature: f.sign("remoteapp-invite-grant-v1", claims)}
	f.binding = invite.NetworkBinding{TargetNode: "nodekey:" + strings.Repeat("a", 64), ControllerNode: "nodekey:" + strings.Repeat("b", 64), TargetIP: "100.120.0.42", ControllerIP: "100.120.0.93"}
	if !invite.VerifyGrant(f.issuer, f.grant, claims.Target, claims.Controller, time.Now()) {
		t.Fatal("fixture grant")
	}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request invite.NetworkRequest
		if r.Method != "POST" || r.URL.Path != "/v1/network" || json.NewDecoder(io.LimitReader(r.Body, 8193)).Decode(&request) != nil || request.Body.Grant != f.grant {
			http.Error(w, "denied", 403)
			return
		}
		raw, _ := json.Marshal(request.Body)
		sig, e := hex.DecodeString(request.Signature)
		if e != nil || request.PublicKey != hex.EncodeToString(f.signer.PublicKey()) || !ed25519.Verify(f.signer.PublicKey(), append([]byte("remoteapp-network-api-v1\nPOST /v1/network\n"), raw...), sig) {
			http.Error(w, "denied", 403)
			return
		}
		var reply invite.NetworkReply
		switch request.Body.Action {
		case "grant_clock":
			c := invite.ClockClaims{Version: 1, GrantID: f.grant.Claims.ID, Nonce: request.Body.Challenge, IssuedAt: time.Now().UnixNano()}
			reply.Proof = &invite.ClockProof{Claims: c, Signature: f.sign("remoteapp-clock-proof-v1", c)}
		case "state":
			f.queries.Add(1)
			if f.stall.Load() {
				<-r.Context().Done()
				return
			}
			state := f.networkState()
			reply.State = &state
		default:
			http.Error(w, "denied", 403)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(reply)
	}))
	t.Cleanup(server.Close)
	f.endpoint, f.certificate = server.URL, server.Certificate().Raw
	return f
}
