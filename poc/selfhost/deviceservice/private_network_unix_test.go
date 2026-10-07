//go:build linux || darwin

package deviceservice

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

	"remoteapp.local/selfhost-poc/broker"
	"remoteapp.local/selfhost-poc/invite"
)

func TestPrivateFrontendRequiresLoopbackAndBroker(t *testing.T) {
	c := fixture(t)
	c.PrivateNetwork = true
	if _, _, e := load(c); e == nil {
		t.Fatal("standalone private network accepted")
	}
	c.Broker = &broker.Client{Socket: "/tmp/unused.sock", ServerUID: uint32(os.Geteuid())}
	if _, _, e := load(c); e != nil {
		t.Fatal("loopback config rejected", e)
	}
	c.Listen = "0.0.0.0:" + strings.Split(c.Listen, ":")[1]
	if _, _, e := load(c); e == nil {
		t.Fatal("public private-network listener accepted")
	}
}

type privateBudgetExecutor struct {
	issuer string
	budget chan time.Duration
}

func (b *privateBudgetExecutor) Execute(ctx context.Context, r broker.Request) broker.Reply {
	if r.Operation == "identity" {
		return broker.Reply{Status: 200, Issuer: b.issuer}
	}
	if r.Operation == "network" {
		end, _ := ctx.Deadline()
		b.budget <- time.Until(end)
		select {
		case <-time.After(6 * time.Second):
		case <-ctx.Done():
		}
	}
	return broker.Reply{Status: 503}
}

func TestPrivateFrontendNetworkBudgetAndNoFallback(t *testing.T) {
	c := fixture(t)
	p, _, e := load(c)
	if e != nil {
		t.Fatal(e)
	}
	dir, e := os.MkdirTemp("/tmp", "ra-net-http-")
	if e != nil {
		t.Fatal(e)
	}
	dir, e = filepath.EvalSymlinks(dir)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	if os.Chown(dir, os.Geteuid(), os.Getegid()) != nil {
		t.Fatal("socket group")
	}
	path := filepath.Join(dir, "broker.sock")
	executor := &privateBudgetExecutor{issuer: p.Claims.Issuer, budget: make(chan time.Duration, 1)}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ready := make(chan struct{}, 1)
	bd := make(chan error, 1)
	go func() {
		bd <- broker.Serve(ctx, broker.Config{Socket: path, PeerUID: uint32(os.Geteuid()), GroupID: uint32(os.Getegid()), Executor: executor}, func() { ready <- struct{}{} })
	}()
	select {
	case <-ready:
	case e := <-bd:
		t.Fatal(e)
	case <-time.After(3 * time.Second):
		t.Fatal("broker startup")
	}
	c.Broker = &broker.Client{Socket: path, ServerUID: uint32(os.Geteuid())}
	c.PrivateNetwork = true
	fc, fd := start(t, c)
	defer fc()
	hc := client(t, c)
	hc.Timeout = 18 * time.Second
	post := func(route, body string) int {
		t.Helper()
		r, e := hc.Post("https://"+c.Listen+route, "application/json", strings.NewReader(body))
		if e != nil {
			t.Fatal(e)
		}
		defer r.Body.Close()
		return r.StatusCode
	}
	_, key, _ := ed25519.GenerateKey(rand.Reader)
	raw, _ := json.Marshal(invite.SignNetworkRequest(key, invite.NetworkRequestBody{Action: "state", Nonce: hex.EncodeToString(make([]byte, 16)), IssuedAt: time.Now().Unix()}))
	if post("/v1/network", string(raw)) != 503 {
		t.Fatal("network result")
	}
	budget := <-executor.budget
	if budget < 13*time.Second || budget > 14*time.Second {
		t.Fatal("network budget changed")
	}
	for _, body := range []string{`{"body":{"action":"state","Action":"bind"}}`, `{} {}`, strings.Repeat("x", 8193)} {
		if post("/v1/network", body) != 400 {
			t.Fatal("strict request boundary")
		}
	}
	if post("/health", "{}") != 404 {
		t.Fatal("unexpected route")
	}
	cancel()
	select {
	case e := <-bd:
		if e != nil {
			t.Fatal(e)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("broker join")
	}
	if post("/v1/network", string(raw)) != 503 {
		t.Fatal("missing broker fallback")
	}
	fc()
	select {
	case e := <-fd:
		if e != nil {
			t.Fatal(e)
		}
	case <-time.After(8 * time.Second):
		t.Fatal("frontend join")
	}
}
