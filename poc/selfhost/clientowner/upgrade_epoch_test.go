package clientowner

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"remoteapp.local/selfhost-poc/trustinstall"
	"remoteapp.local/selfhost-poc/trustprofile"
	"sync/atomic"
	"testing"
	"time"
)

func TestInstalledEpochRejectsBrokenOwnerAndInvalidCandidateKeepsRunner(t *testing.T) {
	o, j, next := setup(t)
	defer o.Close(context.Background())
	started := make(chan struct{})
	var cancelled atomic.Bool
	h := mustStart(t, o, func(ctx context.Context, _ trustprofile.Profile) error {
		close(started)
		<-ctx.Done()
		cancelled.Store(true)
		return nil
	})
	<-started
	bad := next
	bad.Signature = "invalid"
	if o.Upgrade(context.Background(), bad) == nil {
		t.Fatal("bad profile accepted")
	}
	if epoch, e := o.InstalledEpoch(context.Background()); e != nil || epoch != 1 || cancelled.Load() || h.State().Joined {
		t.Fatal("invalid candidate changed live installation", e)
	}
	j.fail = true
	if o.Upgrade(context.Background(), next) == nil {
		t.Fatal("failed commit accepted")
	}
	if _, e := o.InstalledEpoch(context.Background()); e == nil {
		t.Fatal("broken owner emitted usable epoch")
	}
	if !h.State().Joined || h.State().Fault {
		t.Fatal("old runner not normally joined")
	}
	if _, e := o.Start(context.Background(), func(context.Context, trustprofile.Profile) error { return nil }); e == nil {
		t.Fatal("failed commit reused owner")
	}
}

// Models a lost/uncertain adapter acknowledgement AFTER a real native durable
// commit, not an actual OS write failure or a cross-machine TLS acceptance.
type uncertainNativeCommit struct {
	trustinstall.Journal
	armed bool
}

func (j *uncertainNativeCommit) Commit(old, next []byte) error {
	if e := j.Journal.Commit(old, next); e != nil {
		return e
	}
	if j.armed {
		j.armed = false
		return trustprofile.ErrTrust
	}
	return nil
}
func TestNativeUpgradeUncertainReceiptLatchesClosedAndPreservesFloor(t *testing.T) {
	exe := os.Getenv("REMOTEAPP_JOURNAL_TEST_EXECUTABLE")
	if exe == "" {
		t.Skip("native helper required")
	}
	real, e := filepath.EvalSymlinks(exe)
	if e != nil {
		t.Fatal(e)
	}
	raw, e := os.ReadFile(real)
	if e != nil {
		t.Fatal(e)
	}
	digest := sha256.Sum256(raw)
	r, next := fixture(t, hex.EncodeToString(digest[:]))
	f, e := r.NativeFactory(real)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		if e := trustprofile.DeleteTestDesktopJournal(real, r.Scope()); e != nil {
			t.Error("exact test cleanup", e)
		}
	})
	offline, e := trustinstall.NewOfflineOwner(r, f, time.Now)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = offline.Initialize(); e != nil {
		t.Fatal(e)
	}
	wrapped := func(create bool) (trustinstall.Journal, error) {
		j, e := f(create)
		if e != nil {
			return nil, e
		}
		return &uncertainNativeCommit{Journal: j, armed: true}, nil
	}
	o, e := Open(r, wrapped, time.Now)
	if e != nil {
		t.Fatal(e)
	}
	defer o.Close(context.Background())
	started := make(chan struct{})
	h := mustStart(t, o, func(ctx context.Context, _ trustprofile.Profile) error { close(started); <-ctx.Done(); return nil })
	<-started
	if o.Upgrade(context.Background(), next) == nil {
		t.Fatal("uncertain commit accepted")
	}
	if !h.State().Joined || h.State().Fault {
		t.Fatal("runner not joined before uncertain commit")
	}
	if _, e = o.InstalledEpoch(context.Background()); e == nil {
		t.Fatal("uncertain owner remained usable")
	}
	if _, e = o.Start(context.Background(), func(context.Context, trustprofile.Profile) error { return nil }); e == nil {
		t.Fatal("uncertain owner auto-reused")
	}
	if other, e := f(false); e == nil {
		other.Close()
		t.Fatal("native lease released before explicit close")
	}
	if e = o.Close(context.Background()); e != nil {
		t.Fatal(e)
	}
	fresh, e := Open(r, f, time.Now)
	if e != nil {
		t.Fatal(e)
	}
	defer fresh.Close(context.Background())
	if epoch, e := fresh.InstalledEpoch(context.Background()); e != nil || epoch != 2 {
		t.Fatal("native committed floor lost", e)
	}
	if fresh.Upgrade(context.Background(), r.Specification().Bundled) == nil {
		t.Fatal("native floor downgraded")
	}
	if fresh.running != nil {
		t.Fatal("reopen automatically started a runner")
	}
	t.Log("CHECK actual native commit + uncertain receipt latch + explicit idle reopen + persisted epoch 2 floor PASS")
}
