//go:build linux || darwin

package deviceservice

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/broker"
	"remoteapp.local/selfhost-poc/invite"
)

func TestSplitAuthorityHTTPSLifecycle(t *testing.T) {
	c := fixture(t)
	base := filepath.Dir(c.Directory)
	state := filepath.Join(base, "authority")
	if e := os.Rename(filepath.Join(c.Directory, "server-private/authority"), state); e != nil {
		t.Fatal(e)
	}
	profile := filepath.Join(base, "authority-profile.json")
	b, e := os.ReadFile(filepath.Join(c.Directory, "client-public/service-profile.json"))
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(profile, b, 0600); e != nil {
		t.Fatal(e)
	}
	// Unix sockets have a short platform pathname limit; use a private short dir.
	socketDir, e := os.MkdirTemp("/tmp", "ra-ipc-")
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { os.Remove(socketDir) })
	socketDir, e = filepath.EvalSymlinks(socketDir)
	if e != nil {
		t.Fatal(e)
	}
	// macOS temporary directories may inherit a different group.
	for _, dir := range []string{base, state, socketDir} {
		if e = os.Chown(dir, os.Geteuid(), os.Getegid()); e != nil {
			t.Fatal(e)
		}
	}
	dc := broker.DaemonConfig{Profile: profile, ReleaseRoot: c.ReleaseRoot, Service: c.Service, MinimumEpoch: 1, StateDirectory: state, Socket: filepath.Join(socketDir, "authority.sock"), PeerUID: uint32(os.Geteuid()), GroupID: uint32(os.Getegid())}
	for _, bad := range []broker.DaemonConfig{
		func() broker.DaemonConfig { v := dc; v.ReleaseRoot = strings.Repeat("00", 32); return v }(),
		func() broker.DaemonConfig { v := dc; v.MinimumEpoch = 2; return v }(),
		func() broker.DaemonConfig { v := dc; v.Service = "wrong-service"; return v }(),
	} {
		if err := broker.RunDaemon(context.Background(), bad, nil); err == nil {
			t.Fatal("untrusted daemon bootstrap accepted")
		}
	}
	startDaemon := func() (context.CancelFunc, <-chan error) {
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		ready := make(chan struct{}, 1)
		go func() { done <- broker.RunDaemon(ctx, dc, func() { ready <- struct{}{} }) }()
		select {
		case <-ready:
		case err := <-done:
			cancel()
			t.Fatal(err)
		case <-time.After(5 * time.Second):
			cancel()
			t.Fatal("daemon timeout")
		}
		return cancel, done
	}
	stop := func(cancel context.CancelFunc, done <-chan error) {
		t.Helper()
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(8 * time.Second):
			t.Fatal("join timeout")
		}
	}
	ac, ad := startDaemon()
	defer ac()
	if s, err := invite.Open(state, time.Now); err == nil {
		s.Close()
		t.Fatal("second Store owner accepted")
	}
	c.Broker = &broker.Client{Socket: dc.Socket, ServerUID: uint32(os.Geteuid())}
	hc := client(t, c)
	request := func(body string) int {
		t.Helper()
		r, err := hc.Post("https://"+c.Listen+"/v1/request", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		defer r.Body.Close()
		return r.StatusCode
	}
	fc, fd := start(t, c)
	defer fc()
	_, key, _ := ed25519.GenerateKey(rand.Reader)
	nonce := make([]byte, 16)
	rand.Read(nonce)
	env := invite.Sign(key, invite.Body{Action: "register", Nonce: hex.EncodeToString(nonce), IssuedAt: time.Now().Unix()})
	raw, _ := json.Marshal(env)
	if request(string(raw)) != http.StatusOK {
		t.Fatal("split register denied")
	}
	if request(string(raw)) != 403 {
		t.Fatal("split replay accepted")
	}
	if request(`{"body":{"action":"register","Action":"invite"}}`) != 400 {
		t.Fatal("alias accepted")
	}
	stop(fc, fd)
	if s, err := invite.Open(state, time.Now); err == nil {
		s.Close()
		t.Fatal("frontend released authority lock")
	}
	fc, fd = start(t, c)
	defer fc()
	stop(ac, ad)
	if request(string(raw)) != 503 {
		t.Fatal("missing authority did not fail closed")
	}
	if _, err := os.Lstat(filepath.Join(c.Directory, "server-private/authority")); !os.IsNotExist(err) {
		t.Fatal("frontend recreated issuer")
	}
	ac, ad = startDaemon()
	defer ac()
	if request(string(raw)) != 403 {
		t.Fatal("daemon restart lost replay")
	}
	stop(fc, fd)
	stop(ac, ad)
	if s, err := invite.Open(state, time.Now); err != nil {
		t.Fatal(err)
	} else {
		s.Close()
	}
	bad := c
	wrong := *c.Broker
	wrong.ServerUID++
	bad.Broker = &wrong
	ac, ad = startDaemon()
	defer ac()
	if err := Run(context.Background(), bad, nil); err == nil {
		t.Fatal("wrong broker UID accepted")
	}
	stop(ac, ad)
	if e = os.Rename(filepath.Join(state, "state.json"), filepath.Join(state, "state.saved")); e != nil {
		t.Fatal(e)
	}
	if err := broker.RunDaemon(context.Background(), dc, nil); err == nil {
		t.Fatal("missing authority state accepted")
	}
	if _, err := os.Lstat(filepath.Join(state, "state.json")); !os.IsNotExist(err) {
		t.Fatal("daemon silently rekeyed")
	}
}
