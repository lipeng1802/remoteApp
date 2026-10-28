//go:build !windows

package installrelease

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func removalTarget(t *testing.T, p RemovalPlan) removalObject {
	t.Helper()
	for _, o := range p.Objects {
		if !o.Directory && strings.HasPrefix(o.Path, "releases/") {
			return o
		}
	}
	t.Fatal("no signed file")
	return removalObject{}
}

func TestRemovalHandlesParentRedirectCannotDeleteOutside(t *testing.T) {
	root, _, p := removalFixture(t)
	h, err := openRemovalHandles(root, p)
	if err != nil {
		t.Fatal(err)
	}
	defer h.close()
	o := removalTarget(t, p)
	f, err := h.openFile(o)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	parent := filepath.Dir(filepath.Join(root, filepath.FromSlash(o.Path)))
	backup := filepath.Join(t.TempDir(), "pinned-original")
	outside := t.TempDir()
	outsideFile := filepath.Join(outside, filepath.Base(o.Path))
	if err = os.WriteFile(outsideFile, []byte("outside-must-remain"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.Rename(parent, backup); err != nil {
		t.Fatal(err)
	}
	if err = os.Symlink(outside, parent); err != nil {
		t.Fatal(err)
	}
	if err = h.remove(f, o); err != nil {
		t.Fatal(err)
	}
	if raw, e := os.ReadFile(outsideFile); e != nil || string(raw) != "outside-must-remain" {
		t.Fatal("followed redirected parent")
	}
	if _, e := os.Stat(filepath.Join(backup, filepath.Base(o.Path))); !os.IsNotExist(e) {
		t.Fatal("did not unlink pinned parent", e)
	}
}

func TestRemovalHandlesFileReplacementRejected(t *testing.T) {
	root, _, p := removalFixture(t)
	h, err := openRemovalHandles(root, p)
	if err != nil {
		t.Fatal(err)
	}
	defer h.close()
	o := removalTarget(t, p)
	f, err := h.openFile(o)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	target := filepath.Join(root, filepath.FromSlash(o.Path))
	raw, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Rename(target, target+".original"); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(target, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if h.remove(f, o) == nil {
		t.Fatal("deleted replacement inode")
	}
	if _, err = os.Stat(target); err != nil {
		t.Fatal("replacement removed")
	}
	if _, err = os.Stat(target + ".original"); err != nil {
		t.Fatal("original removed")
	}
}
