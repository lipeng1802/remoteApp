package verifier

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"remoteapp.local/selfhost-poc/invite"
	"sync"
	"testing"
	"time"
)

type driver struct {
	mu              sync.Mutex
	mints, cleanups int
	unknown         bool
	record          invite.ProvisionRecord
	ip              string
}

func (d *driver) Mint(_ context.Context, r invite.ProvisionRecord) (invite.RegistrationCredential, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.mints++
	d.record = r
	if d.unknown {
		return invite.RegistrationCredential{}, errors.New("unknown")
	}
	return invite.RegistrationCredential{UserID: "12", KeyID: "13", Secret: "TEST_REGISTRATION_SECRET"}, nil
}
func (d *driver) Observe(_ context.Context, r invite.ProvisionRecord) (invite.ProvisionObservation, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	ip := d.ip
	if ip == "" {
		ip = "100.120.0.2"
	}
	return invite.ProvisionObservation{UserID: r.UserID, NodeID: "14", NodeKey: r.NodeKey, IP: ip}, nil
}
func (d *driver) Cleanup(_ context.Context, r invite.ProvisionRecord) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if r.ID != d.record.ID {
		return ErrVerifier
	}
	d.cleanups++
	return nil
}

type node struct {
	mu     sync.Mutex
	key    string
	secret string
	bad    bool
	closed bool
}

func (n *node) Up(_ context.Context, secret string) (Info, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.secret = secret
	return Info{n.key, "100.120.0.2", true}, nil
}
func (n *node) Status(_ context.Context) (Info, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return Info{n.key, "100.120.0.2", !n.bad}, nil
}
func (n *node) Close() error { n.mu.Lock(); defer n.mu.Unlock(); n.closed = true; return nil }
func path(t *testing.T) string {
	t.Helper()
	p, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	return filepath.Join(p, "owner")
}
func stop(t *testing.T, r *Runtime) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if r.Stop(ctx) != nil {
		t.Fatal("stop")
	}
}
func TestPersistentVerifierResumeAndScopedRetire(t *testing.T) {
	dir := path(t)
	d := &driver{}
	var n *node
	f := func(_ string, key string) Node { n = &node{key: key}; return n }
	r, e := Open(context.Background(), dir, true, d, f)
	if e != nil || !r.Live() {
		t.Fatal(e)
	}
	if _, e = Open(context.Background(), dir, false, d, f); e == nil {
		t.Fatal("concurrent owner")
	}
	if Retire(context.Background(), dir, d) == nil {
		t.Fatal("retired live owner")
	}
	first := r.Info()
	if n.secret != "TEST_REGISTRATION_SECRET" {
		t.Fatal("mint credential absent")
	}
	stop(t, r)
	if r.Live() || !n.closed {
		t.Fatal("not closed")
	}
	b, _ := os.ReadFile(filepath.Join(dir, "journal.json"))
	if string(b) == "" || containsSecret(b) {
		t.Fatal("credential journaled")
	}
	r, e = Open(context.Background(), dir, false, d, f)
	if e != nil || !r.Live() || r.Info() != first || n.secret != "" || d.mints != 1 {
		t.Fatal("resume rekey/remint", e)
	}
	stop(t, r)
	if Retire(context.Background(), dir, d) != nil || Retire(context.Background(), dir, d) != nil || d.cleanups != 1 {
		t.Fatal("scoped cleanup")
	}
	if _, e = Open(context.Background(), dir, false, d, f); e == nil {
		t.Fatal("retired resumed")
	}
	if _, e = Open(context.Background(), dir, true, d, f); e == nil {
		t.Fatal("recreated existing state")
	}
}
func containsSecret(b []byte) bool {
	for i := 0; i+len("TEST_REGISTRATION_SECRET") <= len(b); i++ {
		if string(b[i:i+len("TEST_REGISTRATION_SECRET")]) == "TEST_REGISTRATION_SECRET" {
			return true
		}
	}
	return false
}
func TestUnknownMintIsNotRetriedAndRetirePreservesUncertainty(t *testing.T) {
	dir := path(t)
	d := &driver{unknown: true}
	f := func(_ string, key string) Node { return &node{key: key} }
	if _, e := Open(context.Background(), dir, true, d, f); e == nil {
		t.Fatal("unknown accepted")
	}
	if _, e := Open(context.Background(), dir, false, d, f); e == nil || d.mints != 1 {
		t.Fatal("unknown retried")
	}
	if Retire(context.Background(), dir, d) != nil {
		t.Fatal("retire unknown")
	}
	s, e := load(dir)
	if e != nil || s.Phase != "retired" || s.Record.UserID != "" || s.Record.KeyID != "" {
		t.Fatal("invented handles", e)
	}
}
func TestFaultLatchesClosedNoAutomaticRecovery(t *testing.T) {
	dir := path(t)
	d := &driver{}
	n := &node{}
	r, e := Open(context.Background(), dir, true, d, func(_ string, key string) Node { n.key = key; return n })
	if e != nil {
		t.Fatal(e)
	}
	n.mu.Lock()
	n.bad = true
	n.mu.Unlock()
	select {
	case <-r.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("fault not joined")
	}
	if r.Live() {
		t.Fatal("fault live")
	}
	n.mu.Lock()
	n.bad = false
	closed := n.closed
	n.mu.Unlock()
	if !closed || r.Live() || d.mints != 1 {
		t.Fatal("automatic revival")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if r.Stop(ctx) == nil {
		t.Fatal("fault suppressed")
	}
}
func TestResumeMissingIdentityAndChangedBindingFailClosed(t *testing.T) {
	dir := path(t)
	d := &driver{}
	f := func(_ string, key string) Node { return &node{key: key} }
	r, e := Open(context.Background(), dir, true, d, f)
	if e != nil {
		t.Fatal(e)
	}
	stop(t, r)
	d.mu.Lock()
	d.ip = "100.120.0.3"
	d.mu.Unlock()
	if _, e = Open(context.Background(), dir, false, d, f); e == nil {
		t.Fatal("binding changed")
	}
	state := filepath.Join(dir, "node/tailscaled.state")
	if os.Rename(state, state+".saved") != nil {
		t.Fatal("rename")
	}
	if _, e = Open(context.Background(), dir, false, d, f); e == nil {
		t.Fatal("missing identity regenerated")
	}
	if _, e = os.Lstat(state); !os.IsNotExist(e) || d.mints != 1 {
		t.Fatal("reminted")
	}
}

type blockedNode struct {
	*node
	closing, release chan struct{}
}

func (n *blockedNode) Close() error { close(n.closing); <-n.release; return n.node.Close() }

func TestStopTimeoutRetainsExclusiveLeaseUntilJoin(t *testing.T) {
	dir := path(t)
	d := &driver{}
	n := &blockedNode{node: &node{}, closing: make(chan struct{}), release: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	r, e := Open(ctx, dir, true, d, func(_ string, key string) Node { n.key = key; return n })
	if e != nil {
		cancel()
		t.Fatal(e)
	}
	cancel()
	select {
	case <-n.closing:
	case <-time.After(3 * time.Second):
		t.Fatal("close not started")
	}
	deadline, stopCancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	if r.Stop(deadline) == nil || r.Live() {
		stopCancel()
		t.Fatal("stop timeout hidden")
	}
	stopCancel()
	if _, e = Open(context.Background(), dir, false, d, func(_ string, key string) Node { return &node{key: key} }); e == nil {
		t.Fatal("lease released before close joined")
	}
	close(n.release)
	stop(t, r)
	if _, e = os.Lstat(filepath.Join(dir, "owner.lock")); !os.IsNotExist(e) {
		t.Fatal("joined lock retained")
	}
}
