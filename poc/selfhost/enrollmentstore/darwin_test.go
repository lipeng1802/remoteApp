//go:build darwin

package enrollmentstore

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestMacNativeDirectoryRejectsPermissionsLinksAndPathReplacement(t *testing.T) {
	var raw [16]byte
	rand.Read(raw[:])
	d, e := openDirectory(hex.EncodeToString(raw[:]), true, true)
	if e != nil {
		t.Fatal("native TEST directory create failed")
	}
	defer d.Close()
	path := filepath.Join(d.Path(), "tailscaled.state")
	if os.WriteFile(path, []byte("fixture-no-private-key"), 0600) != nil || d.Check(false) != nil {
		t.Fatal("private fixture rejected")
	}
	if os.Chmod(path, 0644) != nil || d.Check(false) == nil {
		t.Fatal("public node file accepted")
	}
	os.Chmod(path, 0600) // exact TEST-only permissions restored, no journal repair
	link := filepath.Join(d.Path(), "link")
	if os.Link(path, link) != nil || d.Check(false) == nil {
		t.Fatal("hardlink accepted")
	}
	os.Remove(link)
	if os.Symlink(path, link) != nil || d.Check(false) == nil {
		t.Fatal("symlink accepted")
	}
	os.Remove(link)
	if os.Chmod(d.Path(), 0777) != nil || d.Check(false) == nil {
		t.Fatal("open directory accepted")
	}
	os.Chmod(d.Path(), 0700)
	scope := filepath.Dir(d.Path())
	moved := scope + "-moved"
	if os.Rename(scope, moved) != nil {
		t.Fatal("test rename failed")
	}
	if d.Check(false) == nil {
		t.Fatal("renamed namespace accepted")
	}
	if os.Rename(moved, scope) != nil || d.Check(false) != nil {
		t.Fatal("TEST namespace restoration failed")
	}
	if d.ClearTestMaterials() != nil || d.Check(true) != nil {
		t.Fatal("exact TEST material cleanup failed")
	}
}
