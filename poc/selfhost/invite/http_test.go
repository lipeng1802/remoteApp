package invite

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPBoundariesAndReplay(t *testing.T) {
	f := newFixture(t)
	handler := Handler(f.s)
	env := f.envelope(f.owner, Body{Action: "invite"})
	raw, _ := json.Marshal(env)
	call := func(method, path, body, origin string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", origin)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	good := call("POST", "/v1/request", string(raw), "")
	if good.Code != 200 || good.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("signed request failed")
	}
	if call("POST", "/v1/request", string(raw), "").Code != 403 {
		t.Fatal("HTTP replay accepted")
	}
	cases := []struct {
		method, path, body, origin string
		status                     int
	}{
		{"GET", "/v1/request", "", "", 405},
		{"POST", "/unknown", string(raw), "", 404},
		{"POST", "/v1/request?key=SECRET", string(raw), "", 400},
		{"POST", "/v1/request", string(raw), "https://untrusted.example", 400},
		{"POST", "/v1/request", string(raw) + "{}", "", 400},
		{"POST", "/v1/request", `{"unknown":"SECRET"}`, "", 400},
		{"POST", "/v1/request", strings.Repeat("x", 5000), "", 400},
	}
	for _, c := range cases {
		w := call(c.method, c.path, c.body, c.origin)
		if w.Code != c.status || bytes.Contains(w.Body.Bytes(), []byte("SECRET")) {
			t.Fatal("unsafe HTTP boundary")
		}
	}
	if call(http.MethodGet, "/health", "", "").Code != 200 {
		t.Fatal("health failed")
	}
	f.s.Close()
	if call(http.MethodGet, "/health", "", "").Code != 503 {
		t.Fatal("closed store healthy")
	}
}

func TestHTTPRealLifecycleAndRestart(t *testing.T) {
	f := newFixture(t)
	server := httptest.NewServer(Handler(f.s))
	defer func() { server.Close() }()
	issuer := f.s.PublicKey()
	call := func(key ed25519.PrivateKey, b Body) Result {
		t.Helper()
		raw, _ := json.Marshal(f.envelope(key, b))
		response, err := server.Client().Post(server.URL+"/v1/request", "application/json", bytes.NewReader(raw))
		if err != nil {
			t.Fatal("loopback_request_failed")
		}
		defer response.Body.Close()
		if response.StatusCode != 200 {
			t.Fatal("loopback_request_denied")
		}
		var result Result
		if json.NewDecoder(response.Body).Decode(&result) != nil {
			t.Fatal("invalid_response")
		}
		return result
	}
	inv := call(f.owner, Body{Action: "invite"})
	request := call(f.client, Body{Action: "claim", TargetCode: f.target.DeviceCode, Code: inv.Code})
	if call(f.client, Body{Action: "status", RequestID: request.RequestID}).Grant != nil {
		t.Fatal("HTTP granted without consent")
	}
	call(f.owner, Body{Action: "approve", RequestID: request.RequestID, Allow: true})
	granted := call(f.client, Body{Action: "status", RequestID: request.RequestID})
	grant := *granted.Grant
	if !VerifyGrant(issuer, grant, f.target.DeviceID, f.controller.DeviceID, f.now) {
		t.Fatal("HTTP signed grant invalid")
	}
	server.Close()
	f.restart(t)
	server = httptest.NewServer(Handler(f.s))
	if call(f.owner, Body{Action: "register"}).DeviceCode != f.target.DeviceCode {
		t.Fatal("HTTP device code changed")
	}
	call(f.owner, Body{Action: "revoke", GrantID: grant.Claims.ID})
	server.Close()
	f.restart(t)
	server = httptest.NewServer(Handler(f.s))
	status := call(f.client, Body{Action: "status", RequestID: request.RequestID})
	if status.Status != "revoked" || status.Grant != nil || f.s.Authorized(grant, f.target.DeviceID, f.controller.DeviceID) {
		t.Fatal("HTTP restart revived grant")
	}
}
