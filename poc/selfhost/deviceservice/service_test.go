package deviceservice

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/prerelease"
	"remoteapp.local/selfhost-poc/trustprofile"
	"strings"
	"testing"
	"time"
)

func fixture(t *testing.T) Config {
	t.Helper()
	dir, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	address := l.Addr().String()
	l.Close()
	dir = filepath.Join(dir, "new")
	if e = prerelease.Create(dir, "https://"+address, "test-prerelease", time.Now()); e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(filepath.Join(dir, "client-public/installation-anchor.json"))
	if e != nil {
		t.Fatal(e)
	}
	var a prerelease.Anchor
	if json.Unmarshal(b, &a) != nil {
		t.Fatal("anchor")
	}
	return Config{Directory: dir, ReleaseRoot: a.ReleaseRoot, Service: a.Service, Listen: address, MinimumEpoch: 1}
}

func start(t *testing.T, c Config) (context.CancelFunc, <-chan error) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	ready := make(chan struct{}, 1)
	go func() { done <- Run(ctx, c, func(net.Addr) { ready <- struct{}{} }) }()
	select {
	case <-ready:
	case e := <-done:
		cancel()
		t.Fatal(e)
	case <-time.After(5 * time.Second):
		cancel()
		t.Fatal("startup timeout")
	}
	return cancel, done
}

func client(t *testing.T, c Config) *http.Client {
	t.Helper()
	p, _, e := load(c)
	if e != nil {
		t.Fatal(e)
	}
	root, _ := hex.DecodeString(c.ReleaseRoot)
	der, e := trustprofile.Verify(p, root, c.Service, 1, time.Now())
	if e != nil {
		t.Fatal(e)
	}
	ca, e := x509.ParseCertificate(der)
	if e != nil {
		t.Fatal(e)
	}
	pool := x509.NewCertPool()
	pool.AddCert(ca)
	tr := &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS13}, Proxy: nil}
	t.Cleanup(tr.CloseIdleConnections)
	return &http.Client{Transport: tr, Timeout: 2 * time.Second}
}

func TestDeviceServiceTLSRestartAndDenial(t *testing.T) {
	c := fixture(t)
	httpClient := client(t, c)
	request := func(path, body string) int {
		t.Helper()
		r, e := httpClient.Post("https://"+c.Listen+path, "application/json", strings.NewReader(body))
		if e != nil {
			t.Fatal(e)
		}
		defer r.Body.Close()
		return r.StatusCode
	}
	_, key, _ := ed25519.GenerateKey(rand.Reader)
	nonce := make([]byte, 16)
	rand.Read(nonce)
	env := invite.Sign(key, invite.Body{Action: "register", Nonce: hex.EncodeToString(nonce), IssuedAt: time.Now().Unix()})
	raw, _ := json.Marshal(env)
	cancel, done := start(t, c)
	t.Cleanup(cancel)
	if request("/v1/request", string(raw)) != 200 {
		t.Fatal("register denied")
	}
	if request("/v1/request", string(raw)) != 403 {
		t.Fatal("replay accepted")
	}
	if request("/v1/network", "{}") != 404 {
		t.Fatal("privileged API exposed")
	}
	if request("/health", "{}") != 404 {
		t.Fatal("health exposed")
	}
	if request("/v1/request", `{"body":{"action":"register","Action":"invite"}}`) != 400 {
		t.Fatal("alias accepted")
	}
	if request("/v1/request", strings.Repeat("x", 4097)) != 400 {
		t.Fatal("oversize accepted")
	}
	// Pretty JSON remains valid; fresh nonce avoids replay.
	rand.Read(nonce)
	env = invite.Sign(key, invite.Body{Action: "register", Nonce: hex.EncodeToString(nonce), IssuedAt: time.Now().Unix()})
	raw, _ = json.MarshalIndent(env, "", "  ")
	if request("/v1/request", string(raw)) != 200 {
		t.Fatal("pretty request denied")
	}
	cancel()
	select {
	case e := <-done:
		if e != nil {
			t.Fatal(e)
		}
	case <-time.After(8 * time.Second):
		t.Fatal("stop did not join")
	}
	if r, e := httpClient.Post("https://"+c.Listen+"/v1/request", "application/json", bytes.NewReader(raw)); e == nil {
		r.Body.Close()
		t.Fatal("post-stop accepted")
	}
	cancel, done = start(t, c)
	t.Cleanup(cancel)
	if request("/v1/request", string(raw)) != 403 {
		t.Fatal("restart lost durable replay")
	}
	cancel()
	if e := <-done; e != nil {
		t.Fatal(e)
	}
}

func TestDeviceServiceRejectBootstrapAndNoRekey(t *testing.T) {
	c := fixture(t)
	bad := c
	bad.ReleaseRoot = strings.Repeat("00", 32)
	if _, _, e := load(bad); e == nil {
		t.Fatal("wrong root")
	}
	bad = c
	bad.MinimumEpoch = 2
	if _, _, e := load(bad); e == nil {
		t.Fatal("rollback")
	}
	bad = c
	bad.Listen = "127.0.0.1:443"
	if _, _, e := load(bad); e == nil {
		t.Fatal("wrong port")
	}
	keyPath := filepath.Join(c.Directory, "server-private/api-key.pem")
	if os.Chmod(keyPath, 0644) != nil {
		t.Fatal("chmod")
	}
	if _, _, e := load(c); e == nil {
		t.Fatal("public secret")
	}
	os.Chmod(keyPath, 0600)
	state := filepath.Join(c.Directory, "server-private/authority/state.json")
	if e := os.Rename(state, state+".saved"); e != nil {
		t.Fatal(e)
	}
	if e := Run(context.Background(), c, nil); e == nil {
		t.Fatal("missing state accepted")
	}
	if _, e := os.Lstat(state); !os.IsNotExist(e) {
		t.Fatal("silently rekeyed")
	}
}

func TestDeviceServiceProfileExpiryStopsListener(t *testing.T) {
	c := fixture(t)
	p, _, e := load(c)
	if e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(filepath.Join(c.Directory, "release-local-only/release-root.pem"))
	if e != nil {
		t.Fatal(e)
	}
	block, _ := pem.Decode(b)
	key, e := x509.ParsePKCS8PrivateKey(block.Bytes)
	if e != nil {
		t.Fatal(e)
	}
	p.Claims.ExpiresAt = time.Now().Add(3 * time.Second).Unix()
	p, e = trustprofile.Sign(key.(ed25519.PrivateKey), p.Claims)
	if e != nil {
		t.Fatal(e)
	}
	b, _ = json.Marshal(p)
	if e = os.WriteFile(filepath.Join(c.Directory, "client-public/service-profile.json"), b, 0600); e != nil {
		t.Fatal(e)
	}
	cancel, done := start(t, c)
	defer cancel()
	select {
	case e := <-done:
		if e != nil {
			t.Fatal(e)
		}
	case <-time.After(8 * time.Second):
		t.Fatal("expired service still running")
	}
}
