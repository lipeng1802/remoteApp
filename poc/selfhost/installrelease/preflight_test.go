package installrelease

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFreshInstallPreflightDoesNotCreateOrChangeParent(t *testing.T) {
	parent := t.TempDir()
	if err := platformProtectDirectory(parent, true); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(parent)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(parent, "install")
	if err := CheckFreshTestInstallRoot(root); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(parent)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(before, after) || before.Mode() != after.Mode() {
		t.Fatal("parent changed")
	}
	entries, err := os.ReadDir(parent)
	if err != nil || len(entries) != 0 {
		t.Fatal("preflight wrote files")
	}
	// Prove that the check released its directory handle.
	if err := os.Rename(parent, parent+"-moved"); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(parent+"-moved", parent); err != nil {
		t.Fatal(err)
	}
}

func TestFreshInstallPreflightRejectsExistingOrMissingTargets(t *testing.T) {
	parent := t.TempDir()
	if err := platformProtectDirectory(parent, true); err != nil {
		t.Fatal(err)
	}
	for _, directory := range []bool{false, true} {
		root := filepath.Join(parent, "existing")
		if directory {
			if err := os.Mkdir(root, 0700); err != nil {
				t.Fatal(err)
			}
		} else {
			if err := os.WriteFile(root, []byte("unchanged"), 0600); err != nil {
				t.Fatal(err)
			}
		}
		if reason := InstallPreflightReason(CheckFreshTestInstallRoot(root)); reason != "test_install_root_not_fresh" {
			t.Fatal(reason)
		}
		if err := os.Remove(root); err != nil {
			t.Fatal(err)
		}
	}
	for _, root := range []string{"relative", filepath.Join(parent, "missing", "install")} {
		if CheckFreshTestInstallRoot(root) == nil {
			t.Fatal("invalid root accepted")
		}
	}
}

func TestFreshInstallPreflightRejectsUnprotectedParent(t *testing.T) {
	parent := t.TempDir()
	makeBroadlyWritableParent(t, parent)
	if reason := InstallPreflightReason(CheckFreshTestInstallRoot(filepath.Join(parent, "install"))); reason != "test_install_parent_unprotected" {
		t.Fatal(reason)
	}
	entries, err := os.ReadDir(parent)
	if err != nil || len(entries) != 0 {
		t.Fatal("rejected check wrote files")
	}
}
