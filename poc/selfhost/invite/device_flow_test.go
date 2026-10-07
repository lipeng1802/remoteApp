package invite

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestDeviceHTTPSInvitationDiscoveryConsent(t *testing.T) {
	f := newFixture(t)
	b := &ProvisionBackend{Store: f.s}
	server := httptest.NewTLSServer(NewAdmission().Wrap(NetworkHandler(b)))
	defer server.Close()
	clients := make([]*NetworkClient, 0, 3)
	for _, key := range []ed25519.PrivateKey{f.owner, f.client, f.other} {
		c, e := NewNetworkClient(server.URL, server.Certificate().Raw, key)
		if e != nil {
			t.Fatal(e)
		}
		c.now = func() time.Time { return f.now }
		defer c.Close()
		clients = append(clients, c)
	}
	call := func(i int, b Body) Result {
		t.Helper()
		r, e := clients[i].DeviceRequest(context.Background(), b)
		if e != nil {
			t.Fatal(b.Action, e)
		}
		return r
	}
	inv := call(0, Body{Action: "invite"})
	if inv.Code == "" {
		t.Fatal("owner code missing")
	}
	claimed := call(1, Body{Action: "claim", TargetCode: f.target.DeviceCode, Code: inv.Code})
	if claimed.Grant != nil || claimed.Status != "pending" {
		t.Fatal("automatic approval")
	}
	incoming := call(0, Body{Action: "incoming"})
	if incoming.RequestID != claimed.RequestID || incoming.ControllerID != f.controller.DeviceID || incoming.Code != "" || incoming.Grant != nil {
		t.Fatal("discovery scope")
	}
	if call(2, Body{Action: "incoming"}).Status != "idle" {
		t.Fatal("third device enumeration")
	}
	if _, e := clients[2].DeviceRequest(context.Background(), Body{Action: "approve", RequestID: claimed.RequestID, Allow: true}); e == nil {
		t.Fatal("third approval")
	}
	if _, e := clients[2].DeviceRequest(context.Background(), Body{Action: "status", RequestID: claimed.RequestID}); e == nil {
		t.Fatal("third status")
	}
	if _, e := clients[2].DeviceRequest(context.Background(), Body{Action: "claim", TargetCode: f.target.DeviceCode, Code: inv.Code}); e == nil {
		t.Fatal("code reused")
	}
	call(0, Body{Action: "approve", RequestID: incoming.RequestID, Allow: false})
	if call(1, Body{Action: "status", RequestID: claimed.RequestID}).Status != "denied" {
		t.Fatal("reject missing")
	}
	inv = call(0, Body{Action: "invite"})
	claimed = call(1, Body{Action: "claim", TargetCode: f.target.DeviceCode, Code: inv.Code})
	approved := call(0, Body{Action: "approve", RequestID: claimed.RequestID, Allow: true})
	status := call(1, Body{Action: "status", RequestID: claimed.RequestID})
	if approved.Grant == nil || status.Grant == nil || *status.Grant != *approved.Grant || !VerifyGrant(f.s.PublicKey(), *status.Grant, f.target.DeviceID, f.controller.DeviceID, f.now) {
		t.Fatal("approved binding")
	}
	if call(0, Body{Action: "incoming"}).Status != "idle" {
		t.Fatal("already decided remains pending")
	}
}

func TestDeviceBudgetsDurableAndUnknownNonRegisterNoGrowth(t *testing.T) {
	f := newFixture(t)
	_, key, _ := ed25519.GenerateKey(rand.Reader)
	before := len(f.s.state.Replay)
	if _, e := f.s.Execute(f.envelope(key, Body{Action: "incoming"})); e != ErrDenied || len(f.s.state.Replay) != before {
		t.Fatal("unknown key persistent fill")
	}
	for i := 0; i < 3; i++ {
		f.invitation(t)
	}
	f.restart(t)
	if _, e := f.s.Execute(f.envelope(f.owner, Body{Action: "invite"})); e != ErrDenied {
		t.Fatal("rotation budget reset")
	}
	// Fixture registered three identities, leaving seven this minute.
	for i := 0; i < 7; i++ {
		_, key, _ = ed25519.GenerateKey(rand.Reader)
		f.exec(t, key, Body{Action: "register"})
	}
	f.restart(t)
	_, key, _ = ed25519.GenerateKey(rand.Reader)
	if _, e := f.s.Execute(f.envelope(key, Body{Action: "register"})); e != ErrDenied {
		t.Fatal("global registration budget reset")
	}
	f.now = f.now.Add(61 * time.Second)
	f.exec(t, key, Body{Action: "register"})
	f.invitation(t)
}

func TestIncomingSurvivesRestartAndExpiresWithoutGrant(t *testing.T) {
	f := newFixture(t)
	inv := f.invitation(t)
	r := f.claim(t, inv)
	f.restart(t)
	incoming := f.exec(t, f.owner, Body{Action: "incoming"})
	if incoming.RequestID != r.RequestID || incoming.ControllerID != f.controller.DeviceID || incoming.Grant != nil {
		t.Fatal("pending restart")
	}
	f.now = f.now.Add(121 * time.Second)
	if f.exec(t, f.owner, Body{Action: "incoming"}).Status != "idle" {
		t.Fatal("expired request discovered")
	}
	if _, e := f.s.Execute(f.envelope(f.owner, Body{Action: "approve", RequestID: r.RequestID, Allow: true})); e != ErrDenied {
		t.Fatal("expired approval")
	}
	if f.exec(t, f.client, Body{Action: "status", RequestID: r.RequestID}).Grant != nil {
		t.Fatal("expired grant")
	}
}

func TestApprovedGrantClockBeforeBindingIsNotNetworkAuthorization(t *testing.T) {
	f, g, b, _ := provisionFixture(t)
	challenge := strings.Repeat("a", 64)
	e := apiEnvelope(f, f.client, g, "grant_clock")
	e.Body.Challenge = challenge
	e = SignNetworkRequest(f.client, e.Body)
	r, err := b.ExecuteNetworkRequest(context.Background(), e)
	if err != nil || r.Proof == nil {
		t.Fatal("unbound grant clock")
	}
	clock, err := AuthorityClock(f.s.PublicKey(), *r.Proof, g.Claims.ID, challenge, time.Now())
	if err != nil || !VerifyGrant(f.s.PublicKey(), g, g.Claims.Target, g.Claims.Controller, clock()) {
		t.Fatal("calibration")
	}
	if _, err = b.ClockProof(g, challenge); err != ErrDenied {
		t.Fatal("unbound network clock")
	}
	n, err := b.NetworkState(g)
	if err == nil && n.Claims.Active {
		t.Fatal("time proof enabled network")
	}
	f.exec(t, f.owner, Body{Action: "revoke", GrantID: g.Claims.ID})
	e = apiEnvelope(f, f.client, g, "grant_clock")
	e.Body.Challenge = challenge
	e = SignNetworkRequest(f.client, e.Body)
	if _, err = b.ExecuteNetworkRequest(context.Background(), e); err != ErrDenied {
		t.Fatal("revoked grant clock")
	}
}

func TestAdmissionGlobalBudgetAndHandlerReplacement(t *testing.T) {
	a := NewAdmission()
	for i := 0; i < 6000; i++ {
		if !a.allow("192.0.2." + strconv.Itoa(i/600) + ":1") {
			t.Fatal("premature global admission")
		}
	}
	if a.allow("198.51.100.1:1") {
		t.Fatal("global budget bypass")
	}
	called := false
	h := a.Wrap(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	r := httptest.NewRequest("POST", "https://localhost/v1/request", nil)
	r.RemoteAddr = "198.51.100.2:1"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 429 || called {
		t.Fatal("handler replacement reset quota")
	}
}

func TestHTTPSClaimFailuresDoNotEnumerateAndBucketCapacity(t *testing.T) {
	f := newFixture(t)
	inv := f.invitation(t)
	server := httptest.NewTLSServer(NewAdmission().Wrap(NetworkHandler(&ProvisionBackend{Store: f.s})))
	defer server.Close()
	for _, body := range []Body{
		{Action: "claim", TargetCode: "000000000000", Code: inv.Code},
		{Action: "claim", TargetCode: f.target.DeviceCode, Code: "xxxxxxxx"},
		{Action: "claim", TargetCode: f.target.DeviceCode, Code: ""},
	} {
		raw, _ := json.Marshal(f.envelope(f.client, body))
		res, e := server.Client().Post(server.URL+"/v1/request", "application/json", bytes.NewReader(raw))
		if e != nil {
			t.Fatal(e)
		}
		data, e := io.ReadAll(res.Body)
		res.Body.Close()
		if e != nil || res.StatusCode != 403 || string(data) != "{\"error\":\"denied\"}\n" {
			t.Fatal("enumerating response")
		}
	}
	// Fill only the synthetic test snapshot, ensuring claim cannot insert
	// two additional buckets past the persistent 1024 cap.
	f.s.mu.Lock()
	for len(f.s.state.Attempts) < 1023 {
		f.s.state.Attempts["test:"+strconv.Itoa(len(f.s.state.Attempts))] = bucket{Count: 1, Expires: f.now.Unix() + 60}
	}
	f.s.mu.Unlock()
	if _, e := f.s.Execute(f.envelope(f.other, Body{Action: "claim", TargetCode: f.target.DeviceCode, Code: inv.Code})); e != ErrDenied {
		t.Fatal("capacity bypass")
	}
	if len(f.s.state.Attempts) > 1024 {
		t.Fatal("bucket overflow")
	}
	f.restart(t)
}

func TestAdmissionBoundedPeerIPv6AndSpoofing(t *testing.T) {
	a := NewAdmission()
	now := time.Unix(1800000000, 0)
	a.now = func() time.Time { return now }
	for i := 0; i < 600; i++ {
		if !a.allow("[2001:db8::1]:1") {
			t.Fatal("premature limit")
		}
	}
	if a.allow("[2001:db8::2]:2") || a.allow("bad") {
		t.Fatal("ipv6 rotation bypass")
	}
	now = now.Add(61 * time.Second)
	if !a.allow("[2001:db8::2]:2") {
		t.Fatal("window expiry")
	}
	now = now.Add(-time.Second)
	if a.allow("192.0.2.1:1") {
		t.Fatal("clock rollback")
	}
	a = NewAdmission()
	called := 0
	h := a.Wrap(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called++ }))
	for i := 0; i < 601; i++ {
		r := httptest.NewRequest("POST", "https://localhost/v1/request", nil)
		r.RemoteAddr = "192.0.2.1:10"
		r.Header.Set("X-Forwarded-For", "198.51.100.2")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if i == 600 && (w.Code != 429 || w.Body.String() != "{\"error\":\"denied\"}\n") {
			t.Fatal("spoofed forwarding bypass")
		}
	}
	if called != 600 {
		t.Fatal("downstream admission")
	}
	a = NewAdmission()
	for i := 0; i < 256; i++ {
		if !a.allow("192.0.2." + strconv.Itoa(i) + ":1") {
			t.Fatal("peer capacity")
		}
	}
	if a.allow("198.51.100.1:1") || len(a.peers) != 256 {
		t.Fatal("unbounded table")
	}
}
