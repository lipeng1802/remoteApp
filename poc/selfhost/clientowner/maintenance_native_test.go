package clientowner

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/trustinstall"
	"remoteapp.local/selfhost-poc/trustprofile"
)

func TestNativeMaintenanceRetainsLeaseAndEpochFloor(t *testing.T) {
	helper := os.Getenv("REMOTEAPP_JOURNAL_TEST_EXECUTABLE")
	if helper == "" {
		t.Skip("native helper required")
	}
	helper, err := filepath.EvalSymlinks(helper)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(helper)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(raw)
	release, next := fixture(t, hex.EncodeToString(digest[:]))
	factory, err := release.NativeFactory(helper)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := trustprofile.DeleteTestDesktopJournal(helper, release.Scope()); err != nil {
			t.Error("native TEST cleanup", err)
		}
	})
	offline, err := trustinstall.NewOfflineOwner(release, factory, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = offline.Initialize(); err != nil {
		t.Fatal(err)
	}
	if _, err = offline.Upgrade(context.Background(), next); err != nil {
		t.Fatal(err)
	}
	owner, err := Open(release, factory, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close(context.Background())
	before, err := owner.journal.Load()
	if err != nil {
		t.Fatal(err)
	}
	finish := make(chan struct{})
	var once sync.Once
	finishRunner := func() { once.Do(func() { close(finish) }) }
	defer finishRunner()
	h := mustStart(t, owner, func(ctx context.Context, _ trustprofile.Profile) error { <-ctx.Done(); <-finish; return nil })
	marker := filepath.Join(t.TempDir(), "program-fixture")
	if err = os.WriteFile(marker, []byte("TEST ONLY"), 0600); err != nil {
		t.Fatal(err)
	}
	action := func(_ context.Context, epoch uint64) error {
		if epoch != 2 || !h.State().Joined {
			return trustprofile.ErrTrust
		}
		if competing, err := factory(false); err == nil {
			competing.Close()
			return trustprofile.ErrTrust
		}
		return os.Remove(marker)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	err = owner.Maintain(ctx, action)
	cancel()
	if err == nil {
		t.Fatal("unjoined native maintenance succeeded")
	}
	if _, err = os.Stat(marker); err != nil {
		t.Fatal("fixture removed before join")
	}
	if competing, err := factory(false); err == nil {
		competing.Close()
		t.Fatal("lease lost on join timeout")
	}
	finishRunner()
	if err = h.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err = owner.Maintain(context.Background(), action); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("joined fixture not removed")
	}
	after, err := owner.journal.Load()
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("maintenance changed committed native state")
	}
	if err = owner.Maintain(context.Background(), action); err == nil {
		t.Fatal("maintenance action repeated")
	}
	if err = owner.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	fresh, err := Open(release, factory, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	defer fresh.Close(context.Background())
	if epoch, err := fresh.InstalledEpoch(context.Background()); err != nil || epoch != 2 {
		t.Fatal("native floor not retained", err)
	}
	if fresh.Upgrade(context.Background(), release.Specification().Bundled) == nil {
		t.Fatal("maintenance permitted rollback")
	}
	t.Log("CHECK native lease spans timeout and joined action; exact journal unchanged; explicit reopen retains epoch 2 and rejects rollback PASS")
}
