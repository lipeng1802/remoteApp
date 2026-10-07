//go:build linux || darwin

package broker

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http/httptest"
	"os"
	"remoteapp.local/selfhost-poc/invite"
	"strings"
	"testing"
	"time"
)

func TestPrivateNetworkHTTPSViaKernelBroker(t *testing.T) {
	s, st, _ := supervisorFixture(t, &supervisorDriver{})
	path := socket(t)
	cancel, done := start(t, Config{Socket: path, PeerUID: uint32(os.Geteuid()), GroupID: uint32(os.Getegid()), Executor: s.Authority()})
	defer cancel()
	h := httptest.NewTLSServer(invite.NewAdmission().Wrap(PrivateNetworkHandler(Client{Socket: path, ServerUID: uint32(os.Geteuid())}, hex.EncodeToString(st.PublicKey()))))
	defer h.Close()
	post := func(path, raw string) (int, []byte) {
		t.Helper()
		r, e := h.Client().Post(h.URL+path, "application/json", strings.NewReader(raw))
		if e != nil {
			t.Fatal(e)
		}
		defer r.Body.Close()
		b, e := io.ReadAll(r.Body)
		if e != nil {
			t.Fatal(e)
		}
		return r.StatusCode, b
	}
	_, key, _ := ed25519.GenerateKey(rand.Reader)
	nonce := make([]byte, 16)
	rand.Read(nonce)
	env := invite.Sign(key, invite.Body{Action: "register", Nonce: hex.EncodeToString(nonce), IssuedAt: time.Now().Unix()})
	raw, _ := json.Marshal(env)
	if code, _ := post("/v1/request", string(raw)); code != 200 {
		t.Fatal("device registration")
	}
	req := invite.SignNetworkRequest(key, invite.NetworkRequestBody{Action: "state", Nonce: hex.EncodeToString(nonce), IssuedAt: time.Now().Unix()})
	raw, _ = json.Marshal(req)
	if code, _ := post("/v1/network", string(raw)); code != 403 {
		t.Fatal("unsigned/unknown grant bypass")
	}
	for _, bad := range []string{`{"body":{"action":"state","Action":"bind"}}`, `{} {}`, strings.Repeat("x", 8193)} {
		if code, _ := post("/v1/network", bad); code != 400 {
			t.Fatal("noncanonical JSON admitted")
		}
	}
	if code, _ := post("/health", "{}"); code != 404 {
		t.Fatal("unknown route exposed")
	}
	ctx, c := context.WithTimeout(context.Background(), 3*time.Second)
	defer c()
	if s.Stop(ctx) != nil {
		t.Fatal("supervisor stop")
	}
	if code, b := post("/v1/network", string(raw)); code != 503 || strings.Contains(string(b), "credential") {
		t.Fatal("closed network gate")
	}
	stop(t, cancel, done)
	if code, _ := post("/v1/network", string(raw)); code != 503 {
		t.Fatal("missing broker fallback")
	}
}

type deadlineExecutor struct{ remaining chan time.Duration }

func (d *deadlineExecutor) Execute(ctx context.Context, _ Request) Reply {
	v, _ := ctx.Deadline()
	d.remaining <- time.Until(v)
	return Reply{Status: 503}
}
func TestNetworkBudgetBoundedWithoutChangingDeviceBudget(t *testing.T) {
	path := socket(t)
	d := &deadlineExecutor{remaining: make(chan time.Duration, 1)}
	cancel, done := start(t, Config{Socket: path, PeerUID: uint32(os.Geteuid()), GroupID: uint32(os.Getegid()), Executor: d})
	defer cancel()
	c := Client{Socket: path, ServerUID: uint32(os.Geteuid())}
	for _, op := range []string{"network", "device", "forbidden"} {
		if _, e := c.Call(context.Background(), Request{Operation: op}); e != nil {
			t.Fatal(e)
		}
		remaining := <-d.remaining
		if op == "network" {
			if remaining < 13*time.Second || remaining > 14*time.Second {
				t.Fatal("network budget")
			}
		} else if remaining < 3*time.Second || remaining > 4*time.Second {
			t.Fatal("device/unknown budget widened")
		}
	}
	stop(t, cancel, done)
}
