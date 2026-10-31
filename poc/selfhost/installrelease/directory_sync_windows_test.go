//go:build windows

package installrelease

import (
	"golang.org/x/sys/windows"
	"os"
	"path/filepath"
	"testing"
)

func TestWindowsDirectorySyncUsesWritableHandle(t *testing.T) {
	path := t.TempDir()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if f.Sync() != nil {
		t.Log("read-only directory flush rejected by Windows")
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
	f, err = openDirectoryForSync(path)
	if err != nil {
		t.Fatalf("writable directory open: %v", err)
	}
	if err = f.Sync(); err != nil {
		f.Close()
		t.Fatalf("writable directory flush: %v", err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
	if f.Sync() == nil {
		t.Fatal("closed handle flush accepted")
	}
}

func TestWindowsDirectorySyncRejectsFilesMissingAndSharingViolation(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{file, filepath.Join(dir, "missing")} {
		if f, err := openDirectoryForSync(path); err == nil {
			f.Close()
			t.Fatal("invalid directory accepted")
		}
	}
	p, _ := windows.UTF16PtrFromString(dir)
	h, err := windows.CreateFile(p, windows.GENERIC_READ, 0, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		t.Fatal(err)
	}
	if f, err := openDirectoryForSync(dir); err == nil {
		f.Close()
		windows.CloseHandle(h)
		t.Fatal("exclusive handle bypassed")
	}
	if err = windows.CloseHandle(h); err != nil {
		t.Fatal(err)
	}
	f, err := openDirectoryForSync(dir)
	if err != nil {
		t.Fatal("handle not released", err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
}
