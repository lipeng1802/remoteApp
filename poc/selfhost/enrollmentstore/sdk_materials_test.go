package enrollmentstore

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPinnedSDKMaterialSet(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"tailscaled.state", "tailscaled.log.conf", "tailscaled.log1.txt", "tailscaled.log2.txt"} {
		if os.WriteFile(filepath.Join(dir, name), []byte("public fixture"), 0600) != nil {
			t.Fatal("fixture setup")
		}
	}
	entries, _ := os.ReadDir(dir)
	if !knownSDKMaterials(entries) {
		t.Fatal("known SDK set rejected")
	}
	if os.WriteFile(filepath.Join(dir, "unknown"), nil, 0600) != nil {
		t.Fatal("fixture setup")
	}
	entries, _ = os.ReadDir(dir)
	if knownSDKMaterials(entries) {
		t.Fatal("unknown file accepted")
	}
	if os.Remove(filepath.Join(dir, "unknown")) != nil {
		t.Fatal("fixture setup")
	}
	if os.Remove(filepath.Join(dir, "tailscaled.log.conf")) != nil || os.Mkdir(filepath.Join(dir, "tailscaled.log.conf"), 0700) != nil {
		t.Fatal("fixture setup")
	}
	entries, _ = os.ReadDir(dir)
	if knownSDKMaterials(entries) {
		t.Fatal("directory accepted as SDK file")
	}
}
