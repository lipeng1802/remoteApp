//go:build linux || darwin

package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"remoteapp.local/selfhost-poc/headscale"
	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/verifier"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type fixtureVerifierDriver struct{}

func (fixtureVerifierDriver) Mint(context.Context, invite.ProvisionRecord) (invite.RegistrationCredential, error) {
	return invite.RegistrationCredential{UserID: "12", KeyID: "13", Secret: "test-only"}, nil
}
func (fixtureVerifierDriver) Observe(_ context.Context, r invite.ProvisionRecord) (invite.ProvisionObservation, error) {
	return invite.ProvisionObservation{UserID: r.UserID, NodeID: "14", NodeKey: r.NodeKey, IP: "100.120.0.2"}, nil
}
func (fixtureVerifierDriver) Cleanup(context.Context, invite.ProvisionRecord) error { return nil }

type fixtureNode struct{ key string }

func (n *fixtureNode) Up(context.Context, string) (verifier.Info, error) {
	return verifier.Info{Key: n.key, IP: "100.120.0.2", Healthy: true}, nil
}
func (n *fixtureNode) Status(context.Context) (verifier.Info, error) {
	return n.Up(context.Background(), "")
}
func (n *fixtureNode) Close() error { return nil }

func TestSupervisedFixtureKernelHTTPSExplicitRecoveryAndStoreRestart(t *testing.T) {
	dir, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	if e = os.Chmod(dir, 0700); e != nil {
		t.Fatal(e)
	}
	state := filepath.Join(dir, "authority")
	st, e := invite.Open(state, time.Now)
	if e != nil {
		t.Fatal(e)
	}
	defer func() { st.Close() }()
	runtimeDir := filepath.Join(dir, "verifier")
	r, e := verifier.Open(context.Background(), runtimeDir, true, fixtureVerifierDriver{}, func(_ string, key string) verifier.Node { return &fixtureNode{key: key} })
	if e != nil {
		t.Fatal(e)
	}
	var fault atomic.Bool
	runner := func(ctx context.Context, args ...string) ([]byte, error) {
		if ctx.Err() != nil || fault.Load() {
			return nil, headscale.ErrDriver
		}
		if len(args) > 1 && args[0] == "policy" {
			if args[1] == "get" {
				return []byte(`{"acls":[]}`), nil
			}
			return []byte("null"), nil
		}
		if len(args) > 1 && args[0] == "nodes" && args[1] == "list" {
			return []byte(`[]`), nil
		}
		if len(args) > 1 && args[0] == "preauthkeys" && args[1] == "list" {
			return []byte(`[]`), nil
		}
		if len(args) > 1 && args[0] == "users" && args[1] == "list" {
			return []byte(`[]`), nil
		}
		return nil, headscale.ErrDriver
	}
	f, e := startFixtureBroker(&supervisedFixture{store: st, runtime: r, runtimeDir: runtimeDir, dir: dir, runner: runner, dispatch: &fixtureDispatch{}})
	if e != nil {
		t.Fatal(e)
	}
	defer func() {
		fault.Store(false)
		if e := f.Close(); e != nil {
			t.Error(e)
		}
	}()
	if _, _, e = f.Start(st, 100*time.Millisecond); e != nil {
		t.Fatal(e)
	}
	api := httptest.NewTLSServer(f.Handler())
	defer api.Close()
	post := func(path, raw string) int {
		t.Helper()
		res, e := api.Client().Post(api.URL+path, "application/json", strings.NewReader(raw))
		if e != nil {
			t.Fatal(e)
		}
		defer res.Body.Close()
		io.Copy(io.Discard, res.Body)
		return res.StatusCode
	}
	_, key, _ := ed25519.GenerateKey(rand.Reader)
	nonce := make([]byte, 16)
	rand.Read(nonce)
	env := invite.Sign(key, invite.Body{Action: "register", Nonce: hex.EncodeToString(nonce), IssuedAt: time.Now().Unix()})
	raw, _ := json.Marshal(env)
	if post("/v1/request", string(raw)) != 200 {
		t.Fatal("OS signed device path")
	}
	if post("/v1/request", string(raw)) != 403 {
		t.Fatal("replay")
	}
	fault.Store(true)
	end := time.Now().Add(3 * time.Second)
	for f.supervisor.Ready() {
		if time.Now().After(end) {
			t.Fatal("no fault latch")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if post("/v1/network", "{}") != 503 {
		t.Fatal("fault not denied")
	}
	fault.Store(false)
	if f.supervisor.Ready() {
		t.Fatal("automatic recovery")
	}
	if _, _, e = f.Start(st, 100*time.Millisecond); e != nil {
		t.Fatal(e)
	}
	if post("/v1/network", "{}") != 403 {
		t.Fatal("explicit recovery")
	}
	ctx, c := context.WithTimeout(context.Background(), 3*time.Second)
	defer c()
	if f.Stop(ctx) != nil {
		t.Fatal("join")
	}
	if st.Close() != nil {
		t.Fatal("store close")
	}
	st, e = invite.Open(state, time.Now)
	if e != nil {
		t.Fatal(e)
	}
	if _, _, e = f.Start(st, 100*time.Millisecond); e != nil {
		t.Fatal(e)
	}
	if post("/v1/request", string(raw)) != 403 {
		t.Fatal("cold store lost replay")
	}
}
