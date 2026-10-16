//go:build !windows

package installrelease

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPrepareReceiptReopenAndExactTestCleanup(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "prepared")
	receipt, err := PrepareProtectedLayout(root, true)
	if err != nil || receipt.Version != 1 || receipt.Status != "prepared" || receipt.Platform == "" || receipt.Policy != "private-current-user-v1" || !receipt.Test {
		t.Fatal("prepare", receipt, err)
	}
	for _, path := range []string{root, filepath.Join(root, "releases")} {
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm() != 0700 {
			t.Fatal("private mode", path, info, err)
		}
	}
	second, err := PrepareProtectedLayout(root, true)
	if err != nil || second != receipt || ValidateLayoutReceipt(receipt, root, true) != nil {
		t.Fatal("idempotent receipt", second, err)
	}
	tampered := receipt
	tampered.RootObject = "0000000000000000"
	if ValidateLayoutReceipt(tampered, root, true) == nil {
		t.Fatal("tampered receipt accepted")
	}
	if err := CleanupProtectedTestLayout(receipt, root); err != nil {
		t.Fatal("cleanup", err)
	}
	if _, err := os.Lstat(root); !os.IsNotExist(err) {
		t.Fatal("test root retained", err)
	}
}

func TestPrepareRejectsUnsafeParentAndCleanupRejectsContent(t *testing.T) {
	unsafeParent := filepath.Join(t.TempDir(), "unsafe")
	if err := os.Mkdir(unsafeParent, 0777); err != nil || os.Chmod(unsafeParent, 0777) != nil {
		t.Fatal(err)
	}
	if _, err := PrepareProtectedLayout(filepath.Join(unsafeParent, "install"), true); err == nil {
		t.Fatal("unsafe parent accepted")
	}

	root := filepath.Join(t.TempDir(), "nonempty")
	receipt, err := PrepareProtectedLayout(root, true)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root, "releases", "unexpected"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	if CleanupProtectedTestLayout(receipt, root) == nil {
		t.Fatal("nonempty layout removed")
	}
}

func TestPrepareProductionRequiresPrivilege(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("non-root rejection requires an unprivileged test process")
	}
	if _, err := PrepareProtectedLayout(filepath.Join(t.TempDir(), "production"), false); err == nil {
		t.Fatal("unprivileged production prepare accepted")
	}
}
