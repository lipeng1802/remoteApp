//go:build !windows

package supervisedcontrol

import (
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestUnixBusinessSocketPrivateAndJoined(t *testing.T) {
	ln, err := ListenPrivateUnix()
	if err != nil {
		t.Fatal(err)
	}
	address := ln.Address()
	for path, want := range map[string]os.FileMode{filepath.Dir(address): 0700, address: 0600} {
		info, e := os.Lstat(path)
		if e != nil || info.Mode().Perm() != want {
			t.Fatal("private permissions", e)
		}
	}
	exercise(t, ln, func() (net.Conn, error) { return net.Dial("unix", address) })
	if err = ln.Cleanup(); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Lstat(filepath.Dir(address)); !os.IsNotExist(err) {
		t.Fatal("known socket root retained")
	}
}

func TestUnixCleanupPreservesReplacedSocket(t *testing.T) {
	ln, err := ListenPrivateUnix()
	if err != nil {
		t.Fatal(err)
	}
	ln.Close()
	if err = os.Rename(ln.Address(), ln.Address()+".original"); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(ln.Address(), []byte("unknown"), 0600); err != nil {
		t.Fatal(err)
	}
	if ln.Cleanup() == nil {
		t.Fatal("replacement was accepted")
	}
	data, err := os.ReadFile(ln.Address())
	if err != nil || string(data) != "unknown" {
		t.Fatal("unknown replacement deleted")
	}
	// Only the exact files this test created, after the negative check.
	os.Remove(ln.Address())
	os.Rename(ln.Address()+".original", ln.Address())
	if ln.Cleanup() != nil {
		t.Fatal("original socket not cleaned")
	}
}
