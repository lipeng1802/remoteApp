package streamgate

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/privatepipe"
	"remoteapp.local/selfhost-poc/session"
)

// This portable fixture keeps ONLY its ephemeral issuer key in memory. Both
// application identities MUST use OS vaults. It never weakens Store permissions.
func TestNativeVaultPortableLifecycle(t *testing.T) {
	exe := os.Getenv("REMOTEAPP_VAULT_TEST_EXECUTABLE")
	if exe == "" {
		t.Skip("native executable required; acceptance wrapper rejects SKIP")
	}
	create := func() *privatepipe.Signer {
		var raw [16]byte
		rand.Read(raw[:])
		scope := hex.EncodeToString(raw[:])
		s, e := privatepipe.NewSigner(exe, scope, "", true)
		if e != nil {
			t.Fatal("native creation", e)
		}
		t.Cleanup(func() {
			if e := s.DeleteCreatedIdentity(); e != nil {
				t.Error("test identity cleanup", e)
			}
			s.Close()
		})
		reopened, e := privatepipe.NewSigner(exe, scope, hex.EncodeToString(s.PublicKey()), false)
		if e != nil {
			t.Fatal("native reopen", e)
		}
		reopened.Close()
		return s
	}
	target, controller := create(), create()
	issuer, key, _ := ed25519.GenerateKey(rand.Reader)
	sign := func(domain string, value any) string {
		raw, _ := json.Marshal(value)
		return hex.EncodeToString(ed25519.Sign(key, append([]byte(domain+"\n"), raw...)))
	}
	now := time.Now().Unix()
	claims := invite.Claims{Version: 1, ID: strings.Repeat("a", 32), Target: session.Identity(target.PublicKey()), Controller: session.Identity(controller.PublicKey()), Permission: "fixed-payload", IssuedAt: now, ExpiresAt: now + 300}
	grant := invite.Grant{Claims: claims, Signature: sign("remoteapp-invite-grant-v1", claims)}
	binding := invite.NetworkBinding{TargetNode: "nodekey:" + strings.Repeat("a", 64), ControllerNode: "nodekey:" + strings.Repeat("b", 64), TargetIP: "100.120.0.42", ControllerIP: "100.120.0.93"}
	if !invite.VerifyGrant(issuer, grant, claims.Target, claims.Controller, time.Now()) {
		t.Fatal("fixture grant")
	}
	var revoked, unavailable atomic.Bool
	var queries atomic.Int64
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request invite.NetworkRequest
		if r.Method != "POST" || r.URL.Path != "/v1/network" || json.NewDecoder(io.LimitReader(r.Body, 8192)).Decode(&request) != nil || request.Body.Grant != grant {
			http.Error(w, "denied", 403)
			return
		}
		pub, e := hex.DecodeString(request.PublicKey)
		sig, se := hex.DecodeString(request.Signature)
		raw, _ := json.Marshal(request.Body)
		if e != nil || se != nil || (request.PublicKey != hex.EncodeToString(target.PublicKey()) && request.PublicKey != hex.EncodeToString(controller.PublicKey())) || !ed25519.Verify(pub, append([]byte("remoteapp-network-api-v1\nPOST /v1/network\n"), raw...), sig) {
			http.Error(w, "denied", 403)
			return
		}
		if unavailable.Load() {
			http.Error(w, "unavailable", 503)
			return
		}
		reply := invite.NetworkReply{}
		switch request.Body.Action {
		case "grant_clock":
			c := invite.ClockClaims{Version: 1, GrantID: claims.ID, Nonce: request.Body.Challenge, IssuedAt: time.Now().UnixNano()}
			reply.Proof = &invite.ClockProof{Claims: c, Signature: sign("remoteapp-clock-proof-v1", c)}
		case "state":
			queries.Add(1)
			stamp := time.Now().UnixNano()
			c := invite.NetworkClaims{Version: 1, Grant: grant, Binding: binding, Active: !revoked.Load(), IssuedAt: stamp, ExpiresAt: stamp + int64(3*time.Second)}
			reply.State = &invite.NetworkState{Claims: c, Signature: sign("remoteapp-network-state-v1", c)}
		default:
			http.Error(w, "denied", 403)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(reply)
	}))
	defer server.Close()
	start := func(s *privatepipe.Signer, role bool) *ManagedGate {
		c, e := invite.NewNetworkClientWithSigner(server.URL, server.Certificate().Raw, s)
		if e != nil {
			t.Fatal(e)
		}
		t.Cleanup(c.Close)
		clock, e := Calibrate(context.Background(), c, issuer, grant)
		if e != nil {
			t.Fatal("native calibration", e)
		}
		m, e := StartManaged(context.Background(), c, issuer, target.PublicKey(), controller.PublicKey(), s, grant, binding, role, clock)
		if e != nil {
			t.Fatal("native managed startup", e)
		}
		t.Cleanup(m.Stop)
		return m
	}
	pair := func() (*ManagedGate, *ManagedGate) { return start(target, true), start(controller, false) }
	a, b := pair()
	time.Sleep(3500 * time.Millisecond)
	if active, _ := a.Status(); !active {
		t.Fatal("target autonomous lease refresh")
	}
	if active, _ := b.Status(); !active || queries.Load() < 10 {
		t.Fatal("controller autonomous lease refresh")
	}
	t.Log("CHECK OS identity reopen and both autonomous HTTPS watches PASS")
	revoked.Store(true)
	terminal(t, a)
	terminal(t, b)
	t.Log("CHECK signed revocation and watch join PASS")
	revoked.Store(false)
	if active, _ := a.Status(); active {
		t.Fatal("old gate revived")
	}
	a, b = pair()
	// Endpoint metadata is private fixture input: this is not Headscale WhoIs.
	left, right := net.Pipe()
	upstream, echo := net.Pipe()
	local, app := net.Pipe()
	defer app.Close()
	defer echo.Close()
	ta := &net.TCPAddr{IP: net.ParseIP(binding.TargetIP), Port: 47476}
	ca := &net.TCPAddr{IP: net.ParseIP(binding.ControllerIP), Port: 50000}
	go func() { io.Copy(echo, echo) }()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 2)
	go func() {
		done <- session.Tunnel(ctx, overlayConn{left, ta, ca}, a.gate.cfg, func(c context.Context, _ invite.Grant, _, _ string) bool { return a.gate.active(c) }, true, func(context.Context) (net.Conn, error) { return upstream, nil })
	}()
	go func() {
		done <- session.Tunnel(ctx, overlayConn{right, ca, ta}, b.gate.cfg, func(c context.Context, _ invite.Grant, _, _ string) bool { return b.gate.active(c) }, false, func(context.Context) (net.Conn, error) { return local, nil })
	}()
	app.SetDeadline(time.Now().Add(3 * time.Second))
	written := make(chan error, 1)
	go func() { _, e := app.Write([]byte("PING")); written <- e }()
	var response [4]byte
	if _, e := io.ReadFull(app, response[:]); e != nil || string(response[:]) != "PING" {
		t.Fatal("native tunnel exchange", e)
	}
	if e := <-written; e != nil {
		t.Fatal(e)
	}
	unavailable.Store(true)
	terminal(t, a)
	terminal(t, b)
	for i := 0; i < 2; i++ {
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Fatal("native tunnel worker join")
		}
	}
	if _, e := app.Write([]byte("PING")); e == nil {
		t.Fatal("data survived service failure")
	}
	t.Log("CHECK both OS session signatures, fault closure and tunnel worker join PASS")
	before := queries.Load()
	time.Sleep(600 * time.Millisecond)
	if queries.Load() != before {
		t.Fatal("queries continued after join")
	}
	t.Log("CHECK no post-close queries PASS; test credentials cleaned by teardown")
}
