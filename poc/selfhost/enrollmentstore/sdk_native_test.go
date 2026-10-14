//go:build darwin || windows

package enrollmentstore

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
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
