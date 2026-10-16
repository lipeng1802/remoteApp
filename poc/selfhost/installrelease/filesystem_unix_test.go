//go:build !windows

package installrelease

import (
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"
)

func protectedRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "install")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "releases"), 0700); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestProtectedImportCreatesSingleLinkSameVolumeStage(t *testing.T) {
	public, private, _ := ed25519.GenerateKey(rand.Reader)
	source := makeStage(t, t.TempDir(), "protected-import", "darwin", 3, private, 7)
	root := protectedRoot(t)
	verified, err := ImportStaged(source, root, ".staging-0123456789abcdef0123456789abcdef", public, "protected-import", "darwin", 1, true)
	if err != nil || verified.Manifest.Epoch != 3 || filepath.Dir(verified.Directory) != filepath.Join(root, "releases") {
		t.Fatal("protected import", verified, err)
	}
	for _, name := range append([]string{ManifestName}, mustExpected("darwin")...) {
		file, identity, err := openChecked(filepath.Join(verified.Directory, name), false, true)
		if err != nil || identity.links != 1 {
			t.Fatal("imported file is not exclusive", name, identity, err)
		}
		file.Close()
	}
	if err := ValidateProtectedRelease(verified.Directory, root, true); err != nil {
		t.Fatal("protected release rejected", err)
	}
	if err := os.Chmod(verified.Directory, 0777); err != nil {
		t.Fatal(err)
	}
	if err := ValidateProtectedRelease(verified.Directory, root, true); err == nil {
		t.Fatal("writable release directory accepted")
	}
}

func TestProtectedImportRejectsHardlinksAndUnsafeRoots(t *testing.T) {
	public, private, _ := ed25519.GenerateKey(rand.Reader)
	sourceParent := t.TempDir()
	source := makeStage(t, sourceParent, "protected-reject", "darwin", 1, private, 8)
	target := filepath.Join(source, "tls-product")
	outside := filepath.Join(sourceParent, "linked-payload")
	if err := os.Rename(target, outside); err != nil || os.Link(outside, target) != nil {
		t.Fatal("hardlink fixture", err)
	}
	if _, err := ImportStaged(source, protectedRoot(t), ".staging-1123456789abcdef0123456789abcdef", public, "protected-reject", "darwin", 1, true); err == nil {
		t.Fatal("hard-linked source accepted")
	}

	unsafe := protectedRoot(t)
	if err := os.Chmod(unsafe, 0777); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateProtectedLayout(unsafe, true); err == nil {
		t.Fatal("world-writable install root accepted")
	}
	if os.Geteuid() != 0 {
		if _, err := ValidateProtectedLayout(protectedRoot(t), false); err == nil {
			t.Fatal("user-owned production root accepted")
		}
	}

	linkRoot := filepath.Join(t.TempDir(), "linked-install")
	if err := os.Mkdir(linkRoot, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(protectedRoot(t), "releases"), filepath.Join(linkRoot, "releases")); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateProtectedLayout(linkRoot, true); err == nil {
		t.Fatal("linked releases root accepted")
	}
}
