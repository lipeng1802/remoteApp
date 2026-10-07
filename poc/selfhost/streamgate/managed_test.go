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
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/privatepipe"
	"remoteapp.local/selfhost-poc/session"
)

type managedFixture struct {
	store               *invite.Store
	target, controller  ed25519.PrivateKey
	signer              invite.DeviceSigner
	grant               invite.Grant
	binding             invite.NetworkBinding
	client              *invite.NetworkClient
	fail, freeze, stall atomic.Bool
	queries             atomic.Int64
	initial             invite.NetworkState
}

func managedSetup(t *testing.T) *managedFixture {
	return managedSetupWithSigner(t, nil)
}
func managedSetupWithSigner(t *testing.T, signer invite.DeviceSigner) *managedFixture {
	t.Helper()
	f := &managedFixture{binding: invite.NetworkBinding{TargetNode: "nodekey:" + strings.Repeat("a", 64), ControllerNode: "nodekey:" + strings.Repeat("b", 64), TargetIP: "100.120.0.42", ControllerIP: "100.120.0.93"}}
	var e error
	f.store, e = invite.Open(filepath.Join(t.TempDir(), "private"), time.Now)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { f.store.Close() })
	_, f.target, _ = ed25519.GenerateKey(rand.Reader)
	_, f.controller, _ = ed25519.GenerateKey(rand.Reader)
	if signer == nil {
		signer, _ = invite.NewMemorySigner(f.controller)
	}
	f.signer = signer
	target := f.exec(t, f.target, invite.Body{Action: "register"})
	f.device(t, signer, invite.Body{Action: "register"})
	invitation := f.exec(t, f.target, invite.Body{Action: "invite"})
	pending := f.device(t, signer, invite.Body{Action: "claim", TargetCode: target.DeviceCode, Code: invitation.Code})
	f.grant = *f.exec(t, f.target, invite.Body{Action: "approve", RequestID: pending.RequestID, Allow: true}).Grant
	f.initial, e = f.store.NetworkState(f.grant, f.binding)
	if e != nil {
		t.Fatal(e)
	}
	// A private HTTPS fixture issues real Store-signed states/clock proofs. It is
	// not a production API or a test of Headscale provisioning/HTTP admission.
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request invite.NetworkRequest
		if r.Method != "POST" || r.URL.Path != "/v1/network" || json.NewDecoder(r.Body).Decode(&request) != nil || request.Body.Grant != f.grant {
			http.Error(w, "denied", 403)
			return
		}
		body, _ := json.Marshal(request.Body)
		sig, se := hex.DecodeString(request.Signature)
		if se != nil || request.PublicKey != hex.EncodeToString(signer.PublicKey()) || !ed25519.Verify(signer.PublicKey(), append([]byte("remoteapp-network-api-v1\nPOST /v1/network\n"), body...), sig) {
			http.Error(w, "denied", 403)
			return
		}
		if f.fail.Load() {
			http.Error(w, "unavailable", 503)
			return
		}
		var reply invite.NetworkReply
		if request.Body.Action == "grant_clock" {
			proof, e := (&invite.ProvisionBackend{Store: f.store}).GrantClockProof(f.grant, request.Body.Challenge)
			if e != nil {
				http.Error(w, "denied", 403)
				return
			}
			reply.Proof = &proof
		} else if request.Body.Action == "state" {
			f.queries.Add(1)
			if f.stall.Load() {
				<-r.Context().Done()
				return
			}
			state := f.initial
			if !f.freeze.Load() {
				var err error
				state, err = f.store.NetworkState(f.grant, f.binding)
				if err != nil {
					http.Error(w, "denied", 403)
					return
				}
			}
			reply.State = &state
		} else {
			http.Error(w, "denied", 403)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(reply)
	}))
	t.Cleanup(server.Close)
	f.client, e = invite.NewNetworkClientWithSigner(server.URL, server.Certificate().Raw, signer)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(f.client.Close)
	return f
}
func (f *managedFixture) exec(t *testing.T, key ed25519.PrivateKey, b invite.Body) invite.Result {
	signer, e := invite.NewMemorySigner(key)
	if e != nil {
		t.Fatal(e)
	}
	return f.device(t, signer, b)
}
func (f *managedFixture) device(t *testing.T, signer invite.DeviceSigner, b invite.Body) invite.Result {
	t.Helper()
	var n [16]byte
	rand.Read(n[:])
	b.Nonce = hex.EncodeToString(n[:])
	b.IssuedAt = time.Now().Unix()
	envelope, e := invite.SignRequestWithSigner(signer, b)
	if e != nil {
		t.Fatal(e)
	}
	r, e := f.store.Execute(envelope)
	if e != nil {
		t.Fatal(e)
	}
	return r
}
func (f *managedFixture) start(t *testing.T, ctx context.Context) *ManagedGate {
	t.Helper()
	clock, e := Calibrate(ctx, f.client, f.store.PublicKey(), f.grant)
	if e != nil {
		t.Fatal(e)
	}
	m, e := StartManaged(ctx, f.client, f.store.PublicKey(), f.target.Public().(ed25519.PublicKey), f.signer.PublicKey(), f.signer, f.grant, f.binding, false, clock)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(m.Stop)
	return m
}

func TestNativeVaultManagedHTTPSAndStop(t *testing.T) {
	exe := os.Getenv("REMOTEAPP_VAULT_TEST_EXECUTABLE")
	if exe == "" {
		t.Skip("native executable required; no memory substitute")
	}
	var raw [16]byte
	rand.Read(raw[:])
	s, err := privatepipe.NewSigner(exe, hex.EncodeToString(raw[:]), "", true)
	if err != nil {
		t.Fatal("native creation", err)
	}
	t.Cleanup(func() {
		if err := s.DeleteCreatedIdentity(); err != nil {
			t.Error("native cleanup", err)
		}
		s.Close()
	})
	f := managedSetupWithSigner(t, s)
	m := f.start(t, context.Background())
	time.Sleep(1100 * time.Millisecond)
	if f.queries.Load() < 3 || !m.Gate().active(context.Background()) {
		t.Fatal("OS signer autonomous HTTPS refresh")
	}
	m.Stop()
	terminal(t, m)
}

func TestManagedFaultClosesAdmittedOpaqueTunnel(t *testing.T) {
	f := managedSetup(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m := f.start(t, ctx)
	targetSigner, _ := invite.NewMemorySigner(f.target)
	state, _ := f.store.NetworkState(f.grant, f.binding)
	target, e := NewWithSigner(f.store.PublicKey(), f.target.Public().(ed25519.PublicKey), f.signer.PublicKey(), targetSigner, state, true, time.Now)
	if e != nil {
		t.Fatal(e)
	}
	a, b := net.Pipe()
	// Private fixture supplies authenticated endpoint metadata; this tests the
	// session/watchdog coupling, NOT real Headscale WhoIs or product Schannel.
	targetAddr := &net.TCPAddr{IP: net.ParseIP(f.binding.TargetIP), Port: 47476}
	controllerAddr := &net.TCPAddr{IP: net.ParseIP(f.binding.ControllerIP), Port: 50000}
	serverConn := overlayConn{a, targetAddr, controllerAddr}
	clientConn := overlayConn{b, controllerAddr, targetAddr}
	upstream, echo := net.Pipe()
	local, app := net.Pipe()
	defer app.Close()
	defer echo.Close()
	go func() { io.Copy(echo, echo) }()
	done := make(chan error, 2)
	go func() {
		done <- session.Tunnel(ctx, serverConn, target.cfg, func(c context.Context, _ invite.Grant, _, _ string) bool { return target.active(c) }, true, func(context.Context) (net.Conn, error) { return upstream, nil })
	}()
	go func() {
		done <- session.Tunnel(ctx, clientConn, m.gate.cfg, func(c context.Context, _ invite.Grant, _, _ string) bool { return m.gate.active(c) }, false, func(context.Context) (net.Conn, error) { return local, nil })
	}()
	app.SetDeadline(time.Now().Add(2 * time.Second))
	written := make(chan error, 1)
	go func() { _, e := app.Write([]byte("PING")); written <- e }()
	var response [4]byte
	if _, e := io.ReadFull(app, response[:]); e != nil || string(response[:]) != "PING" {
		t.Fatal("admitted tunnel exchange", e)
	}
	if e := <-written; e != nil {
		t.Fatal(e)
	}
	f.fail.Store(true)
	terminal(t, m)
	for i := 0; i < 2; i++ {
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("tunnel workers did not leave")
		}
	}
	if _, e := app.Write([]byte("PING")); e == nil {
		t.Fatal("opaque data survived authority failure")
	}
}
func terminal(t *testing.T, m *ManagedGate) {
	t.Helper()
	select {
	case <-m.Done():
	case <-time.After(4 * time.Second):
		t.Fatal("managed lifecycle timeout")
	}
	if m.Gate().active(context.Background()) {
		t.Fatal("terminal lifecycle still authorized")
	}
	m.Stop()
	m.Stop()
}
func TestManagedHTTPSRefreshAndTerminalCauses(t *testing.T) {
	for _, mode := range []string{"revoke", "fault", "cancel", "stop", "lease_expiry", "pending_query_cancel"} {
		t.Run(mode, func(t *testing.T) {
			f := managedSetup(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			m := f.start(t, ctx)
			time.Sleep(1100 * time.Millisecond)
			if f.queries.Load() < 3 || !m.Gate().active(ctx) {
				t.Fatal("helper did not refresh independently")
			}
			switch mode {
			case "revoke":
				f.exec(t, f.target, invite.Body{Action: "revoke", GrantID: f.grant.Claims.ID})
			case "fault":
				f.fail.Store(true)
			case "cancel":
				cancel()
			case "stop":
				m.Stop()
			case "lease_expiry":
				f.freeze.Store(true)
			case "pending_query_cancel":
				f.stall.Store(true)
				before := f.queries.Load()
				end := time.Now().Add(time.Second)
				for f.queries.Load() == before && time.Now().Before(end) {
					time.Sleep(10 * time.Millisecond)
				}
				if f.queries.Load() == before {
					t.Fatal("pending query not entered")
				}
				m.Stop()
			}
			terminal(t, m)
			// Even an independently valid signed positive fetched by a parent after
			// cancellation/Stop cannot reopen this same gate.
			positive, _ := f.store.NetworkState(f.grant, f.binding)
			m.Gate().Update(positive)
			if m.Gate().active(context.Background()) {
				t.Fatal("signed replay revived managed gate")
			}
			before := f.queries.Load()
			time.Sleep(600 * time.Millisecond)
			if f.queries.Load() != before {
				t.Fatal("query worker was not joined")
			}
		})
	}
}
func TestManagedStartupRejectsWrongBindingAndInactive(t *testing.T) {
	for _, mode := range []string{"wrong_binding", "inactive", "wrong_issuer"} {
		t.Run(mode, func(t *testing.T) {
			f := managedSetup(t)
			binding := f.binding
			issuer := f.store.PublicKey()
			if mode == "wrong_binding" {
				binding.TargetIP = "100.120.0.44"
			}
			if mode == "inactive" {
				f.exec(t, f.target, invite.Body{Action: "revoke", GrantID: f.grant.Claims.ID})
			}
			if mode == "wrong_issuer" {
				issuer = make([]byte, 32)
			}
			signer, _ := invite.NewMemorySigner(f.controller)
			if m, e := StartManaged(context.Background(), f.client, issuer, f.target.Public().(ed25519.PublicKey), f.controller.Public().(ed25519.PublicKey), signer, f.grant, binding, false, time.Now); e == nil {
				m.Stop()
				t.Fatal("invalid managed startup accepted")
			}
		})
	}
	f := managedSetup(t)
	if _, e := Calibrate(context.Background(), f.client, make([]byte, 32), f.grant); e == nil {
		t.Fatal("wrong clock issuer accepted")
	}
}
