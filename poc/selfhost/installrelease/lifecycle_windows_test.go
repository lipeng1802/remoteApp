//go:build windows

package installrelease

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWindowsFreshImportAndCleanupWithoutRecovery(t *testing.T) {
	public, key, _ := ed25519.GenerateKey(rand.Reader)
	root := filepath.Join(t.TempDir(), "install")
	if _, err := PrepareProtectedLayout(root, true); err != nil {
		t.Fatal(err)
	}
	source := makeStage(t, t.TempDir(), "windows-fresh", "windows", 1, key, 1)
	v, err := ImportStaged(source, root, ".staging-"+strings.Repeat("1", 32), public, "windows-fresh", "windows", 1, true)
	if err != nil {
		t.Fatal("first signed import must succeed without resume", err)
	}
	v, err = PromoteStaged(v.Directory, filepath.Join(root, "releases"), public, "windows-fresh", "windows", 1)
	if err != nil {
		t.Fatal(err)
	}
	if err = CleanupVerifiedTestRelease(v.Directory, root, v.ManifestDigest, 1, public, "windows-fresh", "windows", 1, true); err != nil {
		t.Fatal("first cleanup must succeed without finish-files", err)
	}
	if _, err = os.Lstat(v.Directory); !os.IsNotExist(err) {
		t.Fatal("release still present", err)
	}
}

func TestWindowsRemovalBatchReceiptsWithoutRecovery(t *testing.T) {
	root, public, plan := removalFixture(t)
	if err := PersistRemovalTest(root, plan, true); err != nil {
		t.Fatal("persist receipt", err)
	}
	done, err := RemoveProgramBatchTest(context.Background(), root, plan, public, "removal-test", "windows", 1, true)
	if err != nil || done {
		t.Fatal("partial batch receipt", done, err)
	}
	done, err = RemoveProgramBatchTest(context.Background(), root, plan, public, "removal-test", "windows", 256, true)
	if err != nil || !done {
		t.Fatal("final batch receipt", done, err)
	}
}
