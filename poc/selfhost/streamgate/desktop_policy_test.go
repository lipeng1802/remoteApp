package streamgate

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/session"
)

func TestDesktopGatePinsLocalScopeAndOriginalStateLease(t *testing.T) {
	issuer, key, _ := ed25519.GenerateKey(rand.Reader)
	tp, tk, _ := ed25519.GenerateKey(rand.Reader)
	cp, _, _ := ed25519.GenerateKey(rand.Reader)
	now := time.Unix(1800000000, 0)
	g := invite.Grant{Claims: invite.Claims{Version: 1, ID: strings.Repeat("a", 32), Target: session.Identity(tp), Controller: session.Identity(cp), Permission: string(invite.DesktopControl), IssuedAt: now.Unix(), ExpiresAt: now.Unix() + 300}}
	raw, _ := json.Marshal(g.Claims)
	g.Signature = hex.EncodeToString(ed25519.Sign(key, append([]byte("remoteapp-invite-grant-v1\n"), raw...)))
	b := invite.NetworkBinding{TargetNode: "nodekey:" + strings.Repeat("a", 64), ControllerNode: "nodekey:" + strings.Repeat("b", 64), TargetIP: "100.120.0.42", ControllerIP: "100.120.0.93"}
	state := func(active bool) invite.NetworkState {
		n := invite.NetworkState{Claims: invite.NetworkClaims{Version: 1, Grant: g, Binding: b, Active: active, IssuedAt: now.UnixNano(), ExpiresAt: now.Add(3 * time.Second).UnixNano()}}
		r, _ := json.Marshal(n.Claims)
		n.Signature = hex.EncodeToString(ed25519.Sign(key, append([]byte("remoteapp-network-state-v1\n"), r...)))
		return n
	}
	signer, _ := invite.NewMemorySigner(tk)
	p, _ := invite.DesktopGrantPolicy(invite.DesktopControl)
	wrong, _ := invite.DesktopGrantPolicy(invite.DesktopView)
	for _, policy := range []invite.GrantPolicy{wrong, {}} {
		if gate, e := NewWithPolicy(policy, issuer, tp, cp, signer, state(true), true, func() time.Time { return now }); e == nil || gate != nil {
			t.Fatal("gate inferred scope from state")
		}
	}
	gate, e := NewWithPolicy(p, issuer, tp, cp, signer, state(true), true, func() time.Time { return now })
	if e != nil || !gate.active(context.Background()) {
		t.Fatal("scoped gate rejected")
	}
	old := state(true)
	if !gate.Update(state(false)) || gate.Update(old) || gate.active(context.Background()) {
		t.Fatal("revoke replay restored gate")
	}
	gate, e = NewWithPolicy(p, issuer, tp, cp, signer, state(true), true, func() time.Time { return now })
	if e != nil {
		t.Fatal(e)
	}
	now = now.Add(4 * time.Second)
	if gate.active(context.Background()) || gate.Update(old) {
		t.Fatal("gate extended state lease")
	}
}

func TestDesktopManagedPolicyCalibratesAndDeniesBeforeWatchJoin(t *testing.T) {
	issuer, key, _ := ed25519.GenerateKey(rand.Reader)
	tp, tk, _ := ed25519.GenerateKey(rand.Reader)
	cp, ck, _ := ed25519.GenerateKey(rand.Reader)
	now := time.Now()
	g := invite.Grant{Claims: invite.Claims{Version: 1, ID: strings.Repeat("a", 32), Target: session.Identity(tp), Controller: session.Identity(cp), Permission: string(invite.DesktopView), IssuedAt: now.Unix(), ExpiresAt: now.Unix() + 300}}
	raw, _ := json.Marshal(g.Claims)
	g.Signature = hex.EncodeToString(ed25519.Sign(key, append([]byte("remoteapp-invite-grant-v1\n"), raw...)))
	b := invite.NetworkBinding{TargetNode: "nodekey:" + strings.Repeat("a", 64), ControllerNode: "nodekey:" + strings.Repeat("b", 64), TargetIP: "100.120.0.42", ControllerIP: "100.120.0.93"}
	var active atomic.Bool
	active.Store(true)
	var calls atomic.Int64
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req invite.NetworkRequest
		if json.NewDecoder(r.Body).Decode(&req) != nil || req.Body.Grant != g {
			http.Error(w, "denied", 403)
			return
		}
		body, _ := json.Marshal(req.Body)
		sig, _ := hex.DecodeString(req.Signature)
		if req.PublicKey != hex.EncodeToString(cp) || !ed25519.Verify(cp, append([]byte("remoteapp-network-api-v1\nPOST /v1/network\n"), body...), sig) {
			http.Error(w, "denied", 403)
			return
		}
		calls.Add(1)
		var reply invite.NetworkReply
		stamp := time.Now()
		switch req.Body.Action {
		case "grant_clock":
			proof := invite.ClockProof{Claims: invite.ClockClaims{Version: 1, GrantID: g.Claims.ID, Nonce: req.Body.Challenge, IssuedAt: stamp.UnixNano()}}
			raw, _ := json.Marshal(proof.Claims)
			proof.Signature = hex.EncodeToString(ed25519.Sign(key, append([]byte("remoteapp-clock-proof-v1\n"), raw...)))
			reply.Proof = &proof
		case "state":
			n := invite.NetworkState{Claims: invite.NetworkClaims{Version: 1, Grant: g, Binding: b, Active: active.Load(), IssuedAt: stamp.UnixNano(), ExpiresAt: stamp.Add(3 * time.Second).UnixNano()}}
			raw, _ := json.Marshal(n.Claims)
			n.Signature = hex.EncodeToString(ed25519.Sign(key, append([]byte("remoteapp-network-state-v1\n"), raw...)))
			reply.State = &n
		default:
			http.Error(w, "denied", 403)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(reply)
	}))
	defer server.Close()
	client, err := invite.NewNetworkClient(server.URL, server.Certificate().Raw, ck)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	policy, _ := invite.DesktopGrantPolicy(invite.DesktopView)
	wrong, _ := invite.DesktopGrantPolicy(invite.DesktopControl)
	if clock, e := Calibrate(context.Background(), client, issuer, g); e == nil || clock != nil {
		t.Fatal("legacy calibration accepted desktop grant")
	}
	if clock, e := CalibrateForPolicy(context.Background(), client, wrong, issuer, g); e == nil || clock != nil || calls.Load() != 0 {
		t.Fatal("wrong scope reached calibration API")
	}
	clock, err := CalibrateForPolicy(context.Background(), client, policy, issuer, g)
	if err != nil {
		t.Fatal(err)
	}
	signer, _ := invite.NewMemorySigner(tk)
	managed, err := StartManagedForPolicy(context.Background(), client, policy, issuer, tp, cp, signer, g, b, true, clock)
	if err != nil {
		t.Fatal(err)
	}
	defer managed.Stop()
	if ok, done := managed.Status(); !ok || done {
		t.Fatal("scoped managed gate not active")
	}
	active.Store(false)
	select {
	case <-managed.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("scoped managed watch did not join")
	}
	if ok, done := managed.Status(); ok || !done {
		t.Fatal("managed gate retained authorization after join")
	}
}
