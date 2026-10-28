//go:build windows

package installrelease

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRemovalHandlesWindowsPinsParentsAndExactReadonlyFile(t *testing.T) {
	root, _, p := removalFixture(t)
	var o removalObject
	for _, candidate := range p.Objects {
		if !candidate.Directory && strings.HasPrefix(candidate.Path, "releases/") {
			o = candidate
			break
		}
	}
	if o.Path == "" {
		t.Fatal("no signed file")
	}
	target := filepath.Join(root, filepath.FromSlash(o.Path))
	if err := os.Chmod(target, 0400); err != nil {
		t.Fatal(err)
	}
	h, err := openRemovalHandles(root, p)
	if err != nil {
		t.Fatal(err)
	}
	defer h.close()
	f, err := h.openFile(o)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	for _, dir := range []string{root, filepath.Join(root, "releases"), filepath.Dir(target)} {
		if err = os.Rename(dir, dir+".replaced"); err == nil {
			t.Fatal("pinned parent renamed", dir)
		}
	}
	if err = os.Rename(target, target+".replaced"); err == nil {
		t.Fatal("pinned file renamed")
	}
	if writable, e := os.OpenFile(target, os.O_WRONLY, 0); e == nil {
		writable.Close()
		t.Fatal("pinned file writable")
	}
	if err = h.remove(f, o); err != nil {
		t.Fatal("exact handle deletion", err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Lstat(target); !os.IsNotExist(err) {
		t.Fatal("readonly file not removed", err)
	}
	h.close()
	if err = os.Rename(root, root+".released"); err != nil {
		t.Fatal("directory handle leaked", err)
	}
	if err = os.Rename(root+".released", root); err != nil {
		t.Fatal(err)
	}
}

func TestRemovalHandlesWindowsRefusesExistingWriter(t *testing.T) {
	root, _, p := removalFixture(t)
	var o removalObject
	for _, candidate := range p.Objects {
		if !candidate.Directory && strings.HasPrefix(candidate.Path, "releases/") {
			o = candidate
			break
		}
	}
	target := filepath.Join(root, filepath.FromSlash(o.Path))
	f, err := os.OpenFile(target, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	h, err := openRemovalHandles(root, p)
	if err != nil {
		t.Fatal(err)
	}
	defer h.close()
	if deleting, e := h.openFile(o); e == nil {
		deleting.Close()
		t.Fatal("accepted existing write handle")
	}
}
