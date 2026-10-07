//go:build linux || darwin

package broker

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/prerelease"
)

func networkDaemonFixture(t *testing.T) (DaemonConfig, DaemonNetworkConfig) {
	t.Helper()
	base, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	material := filepath.Join(base, "material")
	if e = prerelease.Create(material, "https://test.invalid:8444", "daemon-test", time.Now()); e != nil {
		t.Fatal(e)
	}
	var anchor prerelease.Anchor
	b, e := os.ReadFile(filepath.Join(material, "client-public/installation-anchor.json"))
	if e != nil {
		t.Fatal(e)
	}
	if json.Unmarshal(b, &anchor) != nil {
		t.Fatal("anchor")
	}
	state := filepath.Join(base, "authority")
	if e = os.Rename(filepath.Join(material, "server-private/authority"), state); e != nil {
		t.Fatal(e)
	}
	config := filepath.Join(base, "config")
	verifier := filepath.Join(base, "verifier")
	policy := filepath.Join(base, "policy")
	for _, dir := range []string{base, state, config, verifier, policy} {
		if dir != base && dir != state {
			if e = os.Mkdir(dir, 0700); e != nil {
				t.Fatal(e)
			}
		}
		if e = os.Chown(dir, os.Geteuid(), os.Getegid()); e != nil {
			t.Fatal(e)
		}
	}
	b, e = os.ReadFile(filepath.Join(material, "client-public/service-profile.json"))
	if e != nil {
		t.Fatal(e)
	}
	profile := filepath.Join(config, "profile.json")
	if e = os.WriteFile(profile, b, 0600); e != nil {
		t.Fatal(e)
	}
	n := DaemonNetworkConfig{1, verifier, policy}
	b, _ = json.Marshal(n)
	path := filepath.Join(config, "network.json")
	if e = os.WriteFile(path, b, 0600); e != nil {
		t.Fatal(e)
	}
	return DaemonConfig{Profile: profile, ReleaseRoot: anchor.ReleaseRoot, Service: anchor.Service, MinimumEpoch: 1, StateDirectory: state, Socket: socket(t), PeerUID: uint32(os.Geteuid()), GroupID: uint32(os.Getegid()), NetworkConfig: path}, n
}

func TestDaemonNetworkConfigStrictAndSeparated(t *testing.T) {
	c, n := networkDaemonFixture(t)
	if _, e := loadDaemonNetworkConfig(c); e != nil {
		t.Fatal(e)
	}
	for _, raw := range []string{`{"version":1,"Version":1}`, `{"version":1,"unexpected":true}`, `{}\n{}`, strings.Repeat("x", 8192)} {
		os.WriteFile(c.NetworkConfig, []byte(raw), 0600)
		if _, e := loadDaemonNetworkConfig(c); e == nil {
			t.Fatal("bad config accepted")
		}
	}
	b, _ := json.Marshal(n)
	os.WriteFile(c.NetworkConfig, b, 0600)
	os.Chmod(c.NetworkConfig, 0644)
	if _, e := loadDaemonNetworkConfig(c); e == nil {
		t.Fatal("public config accepted")
	}
	os.Chmod(c.NetworkConfig, 0600)
	n.PolicyDirectory = c.StateDirectory
	b, _ = json.Marshal(n)
	os.WriteFile(c.NetworkConfig, b, 0600)
	if _, e := loadDaemonNetworkConfig(c); e == nil {
		t.Fatal("Store/policy overlap accepted")
	}
	n.PolicyDirectory = filepath.Join(filepath.Dir(n.VerifierDirectory), "policy")
	os.Chmod(n.PolicyDirectory, 0770)
	b, _ = json.Marshal(n)
	os.WriteFile(c.NetworkConfig, b, 0600)
	if _, e := loadDaemonNetworkConfig(c); e == nil {
		t.Fatal("writable policy accepted")
	}
	os.Chmod(n.PolicyDirectory, 0700)
	link := filepath.Join(filepath.Dir(n.VerifierDirectory), "link")
	os.Symlink(n.VerifierDirectory, link)
	n.VerifierDirectory = link
	b, _ = json.Marshal(n)
	os.WriteFile(c.NetworkConfig, b, 0600)
	if _, e := loadDaemonNetworkConfig(c); e == nil {
		t.Fatal("symlink accepted")
	}
}

type testDaemonOwner struct {
	store     *invite.Store
	done      chan struct{}
	stopped   atomic.Bool
	stopError bool
}

func (o *testDaemonOwner) Authority() *Authority { return &Authority{Store: o.store} }
func (o *testDaemonOwner) Done() <-chan struct{} { return o.done }
func (o *testDaemonOwner) Stop(context.Context) error {
	if !o.store.Available() {
		return ErrBroker
	}
	o.stopped.Store(true)
	if o.stopError {
		return ErrBroker
	}
	return nil
}

func TestDaemonNetworkLifecycleAndFailedJoinRetainsStore(t *testing.T) {
	for _, mode := range []string{"normal", "fault", "startup_failure", "failed_join"} {
		t.Run(mode, func(t *testing.T) {
			c, _ := networkDaemonFixture(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			ownerReady := make(chan *testDaemonOwner, 1)
			ready := make(chan struct{}, 1)
			done := make(chan error, 1)
			start := func(ctx context.Context, s *invite.Store, n DaemonNetworkConfig) (daemonNetworkOwner, error) {
				o := &testDaemonOwner{store: s, done: make(chan struct{}), stopError: mode == "failed_join"}
				ownerReady <- o
				if mode == "startup_failure" {
					return o, ErrBroker
				}
				return o, nil
			}
			go func() { done <- runDaemon(ctx, c, func() { ready <- struct{}{} }, start) }()
			var owner *testDaemonOwner
			select {
			case owner = <-ownerReady:
			case <-time.After(5 * time.Second):
				t.Fatal("owner startup")
			}
			defer owner.store.Close()
			if mode != "startup_failure" {
				select {
				case <-ready:
				case <-time.After(5 * time.Second):
					t.Fatal("not ready")
				}
				if mode == "fault" {
					close(owner.done)
				} else {
					cancel()
				}
			}
			select {
			case e := <-done:
				if (e == nil) != (mode == "normal") {
					t.Fatal("unexpected shutdown result", e)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("daemon not joined")
			}
			if !owner.stopped.Load() {
				t.Fatal("owner not stopped before Store")
			}
			if mode == "startup_failure" {
				select {
				case <-ready:
					t.Fatal("false ready")
				default:
				}
			}
			st, e := invite.Open(c.StateDirectory, time.Now)
			if mode == "failed_join" {
				if e == nil {
					st.Close()
					t.Fatal("released Store under failed join")
				}
			} else {
				if e != nil {
					t.Fatal("Store lock not released", e)
				}
				st.Close()
			}
			if _, e = os.Lstat(c.Socket); !os.IsNotExist(e) {
				t.Fatal("broker socket retained")
			}
		})
	}
}

func TestDaemonMissingVerifierFailsWithoutCreation(t *testing.T) {
	c, n := networkDaemonFixture(t)
	before, _ := os.ReadFile(filepath.Join(c.StateDirectory, "state.json"))
	if RunDaemon(context.Background(), c, func() { t.Error("false ready") }) == nil {
		t.Fatal("missing verifier accepted")
	}
	after, _ := os.ReadFile(filepath.Join(c.StateDirectory, "state.json"))
	if string(before) != string(after) {
		t.Fatal("Store changed")
	}
	for _, name := range []string{"journal.json", "node"} {
		if _, e := os.Lstat(filepath.Join(n.VerifierDirectory, name)); !os.IsNotExist(e) {
			t.Fatal("created verifier material")
		}
	}
}
