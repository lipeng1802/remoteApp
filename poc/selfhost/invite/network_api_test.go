package invite

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func apiEnvelope(f *fixture, k ed25519.PrivateKey, g Grant, action string) NetworkRequest {
	nonce, _ := randomHex(16)
	return SignNetworkRequest(k, NetworkRequestBody{Action: action, Nonce: nonce, IssuedAt: f.now.Unix(), Grant: g})
}
func TestNetworkAPIAuthenticationOwnershipReplayAndRestart(t *testing.T) {
	f, g, b, d := provisionFixture(t)
	ids := bindPair(t, f, g, b)
	_, unknown, _ := ed25519.GenerateKey(rand.Reader)
	before := len(f.s.state.Replay)
	if _, err := b.ExecuteNetworkRequest(context.Background(), apiEnvelope(f, unknown, g, "state")); err != ErrDenied || len(f.s.state.Replay) != before {
		t.Fatal("unregistered key consumed persistent capacity")
	}
	e := apiEnvelope(f, f.owner, g, "state")
	n, err := b.ExecuteNetworkRequest(context.Background(), e)
	if err != nil || n.State == nil || !n.State.Claims.Active {
		t.Fatal("device state unavailable")
	}
	if _, err = b.ExecuteNetworkRequest(context.Background(), e); err != ErrDenied {
		t.Fatal("replay")
	}
	f.restart(t)
	b = &ProvisionBackend{Store: f.s, Driver: d}
	if _, err = b.ExecuteNetworkRequest(context.Background(), e); err != ErrDenied {
		t.Fatal("restart replay")
	}
	if b.Reconcile(context.Background()) != nil {
		t.Fatal("reconcile")
	}
	for _, mode := range []string{"foreign", "action", "grant", "ticket", "intent", "time", "signature", "extra"} {
		req := apiEnvelope(f, f.owner, g, "state")
		switch mode {
		case "foreign":
			req = apiEnvelope(f, f.other, g, "state")
		case "action":
			req.Body.Action = "enroll"
		case "grant":
			req.Body.Grant.Claims.Controller = g.Claims.Target
		case "ticket":
			req = apiEnvelope(f, f.owner, g, "bind")
			req.Body.Ticket = ids[1]
			req = SignNetworkRequest(f.owner, req.Body)
		case "intent":
			req = apiEnvelope(f, f.owner, g, "enroll")
			req.Body.Intent = intent(f, g, f.client, "c")
			req = SignNetworkRequest(f.owner, req.Body)
		case "time":
			req.Body.IssuedAt -= 31
			req = SignNetworkRequest(f.owner, req.Body)
		case "signature":
			req.Signature = strings.Repeat("0", 128)
		case "extra":
			req.Body.Ticket = ids[0]
			req = SignNetworkRequest(f.owner, req.Body)
		}
		if _, err = b.ExecuteNetworkRequest(context.Background(), req); err == nil {
			t.Fatal("unauthorized API", mode)
		}
	}
	if d.minted != 2 {
		t.Fatal("denied request invoked mint")
	}
	f.exec(t, f.owner, Body{Action: "revoke", GrantID: g.Claims.ID})
	n, err = b.ExecuteNetworkRequest(context.Background(), apiEnvelope(f, f.client, g, "state"))
	if err != nil || n.State.Claims.Active {
		t.Fatal("revocation must deliver inactive to participant")
	}
	req := apiEnvelope(f, f.owner, g, "clock")
	req.Body.Challenge = strings.Repeat("a", 64)
	req = SignNetworkRequest(f.owner, req.Body)
	if _, err = b.ExecuteNetworkRequest(context.Background(), req); err == nil {
		t.Fatal("revoked clock proof")
	}
}

func TestNetworkAPIRealHTTPSAndEnrollment(t *testing.T) {
	f, g, b, _ := provisionFixture(t)
	server := httptest.NewTLSServer(NetworkHandler(b))
	defer server.Close()
	client, err := NewNetworkClient(server.URL, server.Certificate().Raw, f.owner)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	client.now = func() time.Time { return f.now }
	registered, err := client.DeviceRequest(context.Background(), Body{Action: "register"})
	if err != nil || registered.DeviceID != f.target.DeviceID || registered.DeviceCode != f.target.DeviceCode {
		t.Fatal("stable device registration over TLS")
	}
	req := NetworkRequestBody{Action: "enroll", Grant: g, Intent: intent(f, g, f.owner, "a")}
	enrolled, err := client.Request(context.Background(), req)
	if err != nil || enrolled.Credential == nil || enrolled.Credential.Secret == "" || enrolled.Ticket == "" {
		t.Fatal("authenticated enrollment")
	}
	b.VerifierSource = func(context.Context) (string, error) { return "nodekey:" + strings.Repeat("c", 64), nil }
	challenge := strings.Repeat("d", 64)
	permit, err := client.Request(context.Background(), NetworkRequestBody{Action: "verifier", Grant: g, Ticket: enrolled.Ticket, Challenge: challenge})
	if err != nil || permit.Verifier == nil || !VerifyVerifierPermit(f.s.PublicKey(), *permit.Verifier, g, f.target.DeviceID, enrolled.Ticket, "nodekey:"+strings.Repeat("a", 64), challenge, f.now) {
		t.Fatal("authenticated verifier permit over actual HTTPS")
	}
	if _, err = client.Request(context.Background(), NetworkRequestBody{Action: "bind", Grant: g, Ticket: enrolled.Ticket}); err != nil {
		t.Fatal("device bind")
	}
	if _, err = client.Request(context.Background(), req); err == nil {
		t.Fatal("credential reissued")
	}
	peer, err := NewNetworkClient(server.URL, server.Certificate().Raw, f.client)
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()
	peer.now = client.now
	other, err := peer.Request(context.Background(), NetworkRequestBody{Action: "enroll", Grant: g, Intent: intent(f, g, f.client, "b")})
	if err != nil {
		t.Fatal("peer enroll")
	}
	if _, err = peer.Request(context.Background(), NetworkRequestBody{Action: "bind", Grant: g, Ticket: other.Ticket}); err != nil {
		t.Fatal("peer bind")
	}
	if b.Reconcile(context.Background()) != nil {
		t.Fatal("projection")
	}
	state, err := peer.Request(context.Background(), NetworkRequestBody{Action: "state", Grant: g})
	if err != nil || state.State == nil || !VerifyNetworkState(f.s.PublicKey(), *state.State, f.now) || !state.State.Claims.Active {
		t.Fatal("signed state over HTTPS")
	}
	identityChallenge := strings.Repeat("e", 64)
	identity, err := peer.Request(context.Background(), NetworkRequestBody{Action: "session_identity", Grant: g, Challenge: identityChallenge})
	if err != nil || identity.Session == nil || !VerifySessionIdentity(f.s.PublicKey(), *identity.Session, g, state.State.Claims.Binding, identityChallenge, f.now) {
		t.Fatal("signed session identity over HTTPS")
	}
	proof, err := client.Request(context.Background(), NetworkRequestBody{Action: "clock", Grant: g, Challenge: strings.Repeat("a", 64)})
	if err != nil || proof.Proof == nil || proof.Proof.Claims.Nonce != strings.Repeat("a", 64) {
		t.Fatal("bound clock proof over HTTPS")
	}
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	template := &x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	foreignDER, _ := x509.CreateCertificate(rand.Reader, template, template, priv.Public(), priv)
	foreign, _ := NewNetworkClient(server.URL, foreignDER, f.owner)
	defer foreign.Close()
	foreign.now = client.now
	if _, err = foreign.Request(context.Background(), NetworkRequestBody{Action: "state", Grant: g}); err == nil {
		t.Fatal("wrong TLS root accepted")
	}
	if _, err = NewNetworkClient(server.URL, []byte("bad root"), f.owner); err == nil {
		t.Fatal("invalid trust anchor")
	}
	for _, endpoint := range []string{strings.Replace(server.URL, "https:", "http:", 1), server.URL + "?x=y", server.URL + "/v1/network", "https://example.com:443"} {
		if _, err = NewNetworkClient(endpoint, server.Certificate().Raw, f.owner); err == nil {
			t.Fatal("unsafe endpoint")
		}
	}
}

func TestNetworkHTTPStrictBoundsAndDurableBudget(t *testing.T) {
	f, g, b, _ := provisionFixture(t)
	bindPair(t, f, g, b)
	server := httptest.NewTLSServer(NetworkHandler(b))
	defer server.Close()
	send := func(path string, raw []byte, origin string) int {
		req, _ := http.NewRequest("POST", server.URL+path, bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", origin)
		res, err := server.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		return res.StatusCode
	}
	for _, mode := range []string{"query", "origin", "unknown", "trailing", "oversize"} {
		req := apiEnvelope(f, f.owner, g, "state")
		raw, _ := json.Marshal(req)
		path := "/v1/network"
		origin := ""
		switch mode {
		case "query":
			path += "?secret=x"
		case "origin":
			origin = "https://foreign.example"
		case "unknown":
			raw = []byte(`{"private":"SECRET"}`)
		case "trailing":
			raw = append(raw, []byte("{}")...)
		case "oversize":
			raw = bytes.Repeat([]byte("x"), 8193)
		}
		if send(path, raw, origin) != 400 {
			t.Fatal("HTTP boundary", mode)
		}
	}
	for i := 0; i < 7; i++ {
		req := apiEnvelope(f, f.owner, g, "bind")
		req.Body.Ticket = strings.Repeat("0", 32)
		req = SignNetworkRequest(f.owner, req.Body)
		if _, err := b.ExecuteNetworkRequest(context.Background(), req); err == nil {
			t.Fatal("foreign ticket")
		}
	}
	f.restart(t)
	b = &ProvisionBackend{Store: f.s, Driver: b.Driver}
	b.Reconcile(context.Background())
	req := apiEnvelope(f, f.owner, g, "bind")
	for _, r := range f.s.ProvisionRecords() {
		if r.Device == g.Claims.Target {
			req.Body.Ticket = r.ID
		}
	}
	req = SignNetworkRequest(f.owner, req.Body)
	if _, err := b.ExecuteNetworkRequest(context.Background(), req); err == nil {
		t.Fatal("budget reset on restart")
	}
}
