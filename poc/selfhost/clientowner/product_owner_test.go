package clientowner

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/privatepipe"
	"remoteapp.local/selfhost-poc/processowner"
	"remoteapp.local/selfhost-poc/trustprofile"
)

// Real dedicated child, NOT the C#/Swift product or OS credentials.
func TestOwnedProductFixture(t *testing.T) {
	args := os.Args
	if len(args) < 5 || args[len(args)-3] != "owner-fixture" {
		return
	}
	release, ack := args[len(args)-2], args[len(args)-1]
	json.NewEncoder(os.Stdout).Encode(map[string]string{"status": "ready"})
	var cmd struct{ Op string }
	if privatepipe.NewReader(os.Stdin).Read(&cmd) != nil || cmd.Op != "stop" {
		os.Exit(7)
	}
	json.NewEncoder(os.Stdout).Encode(map[string]string{"status": "closed"})
	if os.WriteFile(ack, []byte("closed-not-exited"), 0600) != nil {
		os.Exit(7)
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if raw, e := os.ReadFile(release); e == nil {
			if string(raw) == "fail" {
				os.Exit(7)
			}
			os.Exit(0)
		}
		time.Sleep(10 * time.Millisecond)
	}
	os.Exit(7)
}
func productChild(t *testing.T) (*processowner.Child, string, string) {
	t.Helper()
	exe, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	exe, e = filepath.EvalSymlinks(exe)
	if e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(exe)
	if e != nil {
		t.Fatal(e)
	}
	digest := sha256.Sum256(b)
	dir := t.TempDir()
	release, ack := filepath.Join(dir, "release"), filepath.Join(dir, "ack")
	c, e := processowner.Start(processowner.Spec{Executable: exe, SHA256: hex.EncodeToString(digest[:]), Args: []string{"-test.run=^TestOwnedProductFixture$", "owner-fixture", release, ack}})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		os.WriteFile(release, []byte("ok"), 0600)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		c.Stop(ctx)
	})
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	if _, e := c.Exchange(ctx, nil); e != nil {
		t.Fatal(e)
	}
	return c, release, ack
}
func awaitAck(t *testing.T, path string) {
	t.Helper()
	end := time.Now().Add(4 * time.Second)
	for time.Now().Before(end) {
		if _, e := os.Stat(path); e == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("child did not acknowledge stop")
}
func releaseProduct(t *testing.T, path, value string) {
	t.Helper()
	if e := os.WriteFile(path, []byte(value), 0600); e != nil {
		t.Fatal(e)
	}
}
func TestOwnedProductUpgradeWaitsForProductAndNetworkJoin(t *testing.T) {
	o, j, next := setup(t)
	child, release, ack := productChild(t)
	started, networkCancelled, networkRelease := make(chan struct{}), make(chan struct{}), make(chan struct{})
	defer func() {
		select {
		case <-networkRelease:
		default:
			close(networkRelease)
		}
	}()
	h := mustStart(t, o, func(ctx context.Context, _ trustprofile.Profile) error {
		close(started)
		return processowner.Own(ctx, child, func(ctx context.Context) error { <-ctx.Done(); close(networkCancelled); <-networkRelease; return nil })
	})
	<-started
	upgraded := make(chan error, 1)
	go func() { upgraded <- o.Upgrade(context.Background(), next) }()
	<-networkCancelled
	awaitAck(t, ack)
	j.mu.Lock()
	commits, closed := j.commits, j.closed
	j.mu.Unlock()
	if commits != 1 || closed {
		t.Fatal("journal committed/released before product and network join")
	}
	releaseProduct(t, release, "ok")
	select {
	case <-h.Done():
		t.Fatal("network still unjoined")
	case <-time.After(150 * time.Millisecond):
	}
	close(networkRelease)
	select {
	case e := <-upgraded:
		if e != nil {
			t.Fatal(e)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("upgrade did not join")
	}
	j.mu.Lock()
	commits = j.commits
	j.mu.Unlock()
	if commits != 2 {
		t.Fatal("new profile not committed")
	}
}
func TestOwnedProductCloseTimeoutRetainsJournalUntilActualExit(t *testing.T) {
	o, j, _ := setup(t)
	child, release, ack := productChild(t)
	started := make(chan struct{})
	mustStart(t, o, func(ctx context.Context, _ trustprofile.Profile) error {
		close(started)
		return processowner.Own(ctx, child, func(ctx context.Context) error { <-ctx.Done(); return nil })
	})
	<-started
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if o.Close(ctx) == nil {
		t.Fatal("timeout released owner")
	}
	awaitAck(t, ack)
	j.mu.Lock()
	closed := j.closed
	j.mu.Unlock()
	if closed {
		t.Fatal("lease released on closed ack")
	}
	releaseProduct(t, release, "ok")
	ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()
	if e := o.Close(ctx2); e != nil {
		t.Fatal(e)
	}
	j.mu.Lock()
	closed = j.closed
	j.mu.Unlock()
	if !closed {
		t.Fatal("joined lease not released")
	}
}
func TestOwnedProductNonzeroExitRejectsUpgrade(t *testing.T) {
	o, j, next := setup(t)
	child, release, ack := productChild(t)
	started := make(chan struct{})
	mustStart(t, o, func(ctx context.Context, _ trustprofile.Profile) error {
		close(started)
		return processowner.Own(ctx, child, func(ctx context.Context) error { <-ctx.Done(); return nil })
	})
	<-started
	result := make(chan error, 1)
	go func() { result <- o.Upgrade(context.Background(), next) }()
	awaitAck(t, ack)
	releaseProduct(t, release, "fail")
	select {
	case e := <-result:
		if e == nil {
			t.Fatal("nonzero product exit committed upgrade")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("failure did not join")
	}
	j.mu.Lock()
	commits := j.commits
	j.mu.Unlock()
	if commits != 1 {
		t.Fatal("failed product committed new profile")
	}
}
