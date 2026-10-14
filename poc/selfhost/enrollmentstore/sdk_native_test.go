//go:build darwin || windows

package enrollmentstore

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"remoteapp.local/selfhost-poc/nodeidentity"
	"testing"
)

func TestNativeSDKCleanupRefusesUnknownBeforeAnyDeletion(t *testing.T) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		t.Fatal("random scope")
	}
	s, err := Initialize(hex.EncodeToString(raw[:]), true)
	if err != nil {
		t.Fatal("native TEST initialize")
	}
	defer s.Close()
	for _, name := range []string{"tailscaled.state", "tailscaled.log.conf", "tailscaled.log1.txt", "tailscaled.log2.txt", "unknown"} {
		if os.WriteFile(filepath.Join(s.NodeDir(), name), []byte("public fixture"), 0600) != nil {
			t.Fatal("fixture setup")
		}
	}
	if s.CleanupTestAfterJoin() == nil {
		t.Fatal("unknown file cleanup accepted")
	}
	entries, err := os.ReadDir(s.NodeDir())
	if err != nil || len(entries) != 5 {
		t.Fatal("rejected cleanup deleted evidence")
	}
	if os.Remove(filepath.Join(s.NodeDir(), "unknown")) != nil {
		t.Fatal("remove exact public fixture")
	}
	if s.CleanupTestAfterJoin() != nil {
		t.Fatal("known SDK TEST cleanup")
	}
	entries, err = os.ReadDir(s.NodeDir())
	if err != nil || len(entries) != 0 {
		t.Fatal("SDK files left after success")
	}
}

func TestNativeSDKCleanupRejectsCacheContentBeforeDeletingState(t *testing.T) {
	var raw [16]byte
	if _, e := rand.Read(raw[:]); e != nil {
		t.Fatal("random scope")
	}
	s, e := Initialize(hex.EncodeToString(raw[:]), true)
	if e != nil {
		t.Fatal("TEST initialize")
	}
	defer s.Close()
	if _, e = nodeidentity.Prepare(s.NodeDir(), "http://127.0.0.1:18443"); e != nil {
		t.Fatal("offline TEST state")
	}
	id, e := nodeidentity.ReadProfileID(s.NodeDir())
	if e != nil {
		t.Fatal("profile metadata")
	}
	cache := filepath.Join(s.NodeDir(), "profile-data", id, "netmap-cache")
	if os.MkdirAll(cache, 0700) != nil || os.WriteFile(filepath.Join(cache, "unknown"), []byte("public fixture"), 0600) != nil {
		t.Fatal("cache fixture")
	}
	if s.CleanupTestAfterJoin() == nil {
		t.Fatal("nonempty cache cleanup accepted")
	}
	if _, e = os.Stat(filepath.Join(s.NodeDir(), "tailscaled.state")); e != nil {
		t.Fatal("rejected cleanup deleted state")
	}
	if os.Remove(filepath.Join(cache, "unknown")) != nil {
		t.Fatal("remove exact fixture")
	}
	if s.CleanupTestAfterJoin() != nil {
		t.Fatal("known empty cache cleanup")
	}
	entries, e := os.ReadDir(s.NodeDir())
	if e != nil || len(entries) != 0 {
		t.Fatal("SDK materials remain")
	}
}
