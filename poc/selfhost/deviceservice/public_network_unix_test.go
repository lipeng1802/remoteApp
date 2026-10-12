//go:build linux || darwin

package deviceservice

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/broker"
	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/prerelease"
	"remoteapp.local/selfhost-poc/trustprofile"
)

func directFixture(t *testing.T) Config {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := l.Addr().String()
	l.Close()
	_, port, _ := net.SplitHostPort(address)
	dir := filepath.Join(base, "direct")
	if err = prerelease.Create(dir, "https://device-direct.test:"+port, "direct-test", time.Now()); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "client-public/installation-anchor.json"))
	if err != nil {
		t.Fatal(err)
	}
	var anchor prerelease.Anchor
	if json.Unmarshal(b, &anchor) != nil {
		t.Fatal("anchor")
	}
	return Config{Directory: dir, ReleaseRoot: anchor.ReleaseRoot, Service: anchor.Service, MinimumEpoch: 1, Listen: address}
}

func directHTTPClient(t *testing.T, c Config) *http.Client {
	t.Helper()
	p, _, err := load(c)
	if err != nil {
		t.Fatal(err)
	}
	root, _ := hex.DecodeString(c.ReleaseRoot)
	der, err := trustprofile.Verify(p, root, c.Service, 1, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	ca, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(ca)
	u, _ := url.Parse(p.Claims.Endpoint)
	tr := &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool, ServerName: u.Hostname(), MinVersion: tls.VersionTLS13}, Proxy: nil}
	t.Cleanup(tr.CloseIdleConnections)
	return &http.Client{Transport: tr, Timeout: 3 * time.Second}
}

func localBroker(t *testing.T, executor broker.Executor) (*broker.Client, context.CancelFunc, <-chan error) {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "ra-direct-")
	if err != nil {
		t.Fatal(err)
	}
	dir, err = filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(dir) })
	if os.Chown(dir, os.Geteuid(), os.Getegid()) != nil {
		t.Fatal("group")
	}
	path := filepath.Join(dir, "broker.sock")
	ctx, cancel := context.WithCancel(context.Background())
	done, ready := make(chan error, 1), make(chan struct{}, 1)
	go func() {
		done <- broker.Serve(ctx, broker.Config{Socket: path, PeerUID: uint32(os.Geteuid()), GroupID: uint32(os.Getegid()), Executor: executor}, func() { ready <- struct{}{} })
	}()
	select {
	case <-ready:
	case err := <-done:
		cancel()
		t.Fatal(err)
	case <-time.After(3 * time.Second):
		cancel()
		t.Fatal("broker ready deadline")
	}
	t.Cleanup(cancel)
	return &broker.Client{Socket: path, ServerUID: uint32(os.Geteuid())}, cancel, done
}

func joinService(t *testing.T, cancel context.CancelFunc, done <-chan error) {
	t.Helper()
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(8 * time.Second):
		t.Fatal("join deadline")
	}
}

func TestDirectNetworkModeRequiresBrokerNamedProfileAndExplicitFlag(t *testing.T) {
	c := directFixture(t)
	c.PublicNetwork = true
	if _, _, err := load(c); err == nil {
		t.Fatal("public standalone Store accepted")
	}
	c.Broker = &broker.Client{Socket: "/tmp/not-running.sock", ServerUID: uint32(os.Geteuid())}
	if _, _, err := load(c); err != nil {
		t.Fatal("explicit named-profile mode rejected")
	}
	_, port, _ := net.SplitHostPort(c.Listen)
	c.Listen = "0.0.0.0:" + port
	if _, _, err := load(c); err != nil {
		t.Fatal("explicit wildcard mode rejected")
	}
	c.PrivateNetwork = true
	if _, _, err := load(c); err == nil {
		t.Fatal("combined public/private modes accepted")
	}
	c.PrivateNetwork = false
	c.Broker = &broker.Client{Socket: "relative.sock"}
	if _, _, err := load(c); err == nil {
		t.Fatal("relative broker accepted")
	}
	c = fixture(t)
	c.PublicNetwork = true
	c.Broker = &broker.Client{Socket: "/tmp/not-running.sock"}
	if _, _, err := load(c); err == nil {
		t.Fatal("loopback bootstrap advertised as public profile")
	}
}

func TestDirectHTTPSDeviceReplayAndAuthorityOutageRemainFailClosed(t *testing.T) {
	c := directFixture(t)
	store, err := invite.Open(filepath.Join(c.Directory, "server-private/authority"), time.Now)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	b, bc, bd := localBroker(t, &broker.Authority{Store: store})
	c.Broker = b
	c.PublicNetwork = true
	hc := directHTTPClient(t, c)
	post := func(path, body string) int {
		t.Helper()
		r, err := hc.Post("https://"+c.Listen+path, "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		defer r.Body.Close()
		return r.StatusCode
	}
	fc, fd := start(t, c)
	defer fc()
	_, key, _ := ed25519.GenerateKey(rand.Reader)
	var nonce [16]byte
	rand.Read(nonce[:])
	env := invite.Sign(key, invite.Body{Action: "register", Nonce: hex.EncodeToString(nonce[:]), IssuedAt: time.Now().Unix()})
	raw, _ := json.Marshal(env)
	if post("/v1/request", string(raw)) != 200 || post("/v1/request", string(raw)) != 403 {
		t.Fatal("signed register/replay boundary")
	}
	for _, route := range []string{"/v1/request", "/v1/network"} {
		r, err := http.NewRequest("POST", "https://"+c.Listen+route, strings.NewReader(`{}`))
		if err != nil {
			t.Fatal(err)
		}
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", "https://untrusted.example")
		r.Header.Set("X-Forwarded-For", "127.0.0.1")
		response, err := hc.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != 400 {
			t.Fatal("browser/proxy headers bypassed ingress boundary")
		}
	}
	for _, bad := range []struct {
		path, body string
		status     int
	}{{"/v1/network", `{}`, 503}, {"/v1/network", `{"body":{"action":"state","Action":"enroll"}}`, 400}, {"/v1/network", strings.Repeat("x", 8193), 400}, {"/health", `{}`, 404}, {"/v1/network?x=1", `{}`, 400}} {
		if post(bad.path, bad.body) != bad.status {
			t.Fatal("direct negative boundary", bad.path)
		}
	}
	joinService(t, fc, fd)
	c.PublicNetwork = false
	fc, fd = start(t, c)
	defer fc()
	if post("/v1/network", `{}`) != 404 {
		t.Fatal("default frontend enabled network route")
	}
	joinService(t, fc, fd)
	c.PublicNetwork = true
	fc, fd = start(t, c)
	defer fc()
	if post("/v1/request", string(raw)) != 403 {
		t.Fatal("frontend restart erased durable replay")
	}
	joinService(t, bc, bd)
	if post("/v1/request", string(raw)) != 503 || post("/v1/network", `{}`) != 503 {
		t.Fatal("authority unavailable fallback")
	}
	joinService(t, fc, fd)
}

type sharedBudgetExecutor struct {
	issuer  string
	entered chan struct{}
	release chan struct{}
	calls   atomic.Int32
}

func (b *sharedBudgetExecutor) Execute(ctx context.Context, r broker.Request) broker.Reply {
	if r.Operation == "identity" {
		return broker.Reply{Status: 200, Issuer: b.issuer}
	}
	b.calls.Add(1)
	b.entered <- struct{}{}
	select {
	case <-b.release:
	case <-ctx.Done():
	}
	return broker.Reply{Status: 503}
}

func TestDirectDeviceAndNetworkShareFourWorkSlots(t *testing.T) {
	c := directFixture(t)
	profile, _, err := load(c)
	if err != nil {
		t.Fatal(err)
	}
	executor := &sharedBudgetExecutor{issuer: profile.Claims.Issuer, entered: make(chan struct{}, 4), release: make(chan struct{})}
	b, bc, bd := localBroker(t, executor)
	c.Broker = b
	c.PublicNetwork = true
	fc, fd := start(t, c)
	defer fc()
	hc := directHTTPClient(t, c)
	done := make(chan error, 4)
	for i := 0; i < 4; i++ {
		go func() {
			r, err := hc.Post("https://"+c.Listen+"/v1/network", "application/json", strings.NewReader(`{}`))
			if err == nil {
				r.Body.Close()
			}
			done <- err
		}()
	}
	for i := 0; i < 4; i++ {
		select {
		case <-executor.entered:
		case <-time.After(3 * time.Second):
			close(executor.release)
			t.Fatal("work slots not entered")
		}
	}
	r, err := hc.Post("https://"+c.Listen+"/v1/request", "application/json", strings.NewReader(`{}`))
	if err != nil {
		close(executor.release)
		t.Fatal(err)
	}
	r.Body.Close()
	if r.StatusCode != 503 || executor.calls.Load() != 4 {
		close(executor.release)
		t.Fatal("device route bypassed shared budget")
	}
	close(executor.release)
	for i := 0; i < 4; i++ {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
	joinService(t, fc, fd)
	joinService(t, bc, bd)
}
