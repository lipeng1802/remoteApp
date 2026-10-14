package enrollmentstore

import (
	"os"
	"path/filepath"
	"remoteapp.local/selfhost-poc/nodeidentity"
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

func TestSDKCleanupPlanBindsCurrentProfileAndEmptyCache(t *testing.T) {
	dir := t.TempDir()
	if _, e := nodeidentity.Prepare(dir, "http://127.0.0.1:18443"); e != nil {
		t.Fatal("offline TEST prepare")
	}
	id, e := nodeidentity.ReadProfileID(dir)
	if e != nil {
		t.Fatal("read profile metadata")
	}
	cache := filepath.Join(dir, "profile-data", id, "netmap-cache")
	if os.MkdirAll(cache, 0700) != nil {
		t.Fatal("cache fixture")
	}
	plan, e := sdkCleanupPlan(dir)
	if e != nil || len(plan) != 4 || !plan[1].directory || plan[1].path != filepath.Join("profile-data", id, "netmap-cache") {
		t.Fatal("known empty cache plan")
	}
	if os.WriteFile(filepath.Join(cache, "unknown"), []byte("public fixture"), 0600) != nil {
		t.Fatal("cache fixture")
	}
	if _, e = sdkCleanupPlan(dir); e == nil {
		t.Fatal("nonempty cache accepted")
	}
	if os.Remove(filepath.Join(cache, "unknown")) != nil {
		t.Fatal("remove exact fixture")
	}
	other := "0000"
	if id == other {
		other = "0001"
	}
	if os.Mkdir(filepath.Join(dir, "profile-data", other), 0700) != nil {
		t.Fatal("profile fixture")
	}
	if _, e = sdkCleanupPlan(dir); e == nil {
		t.Fatal("extra profile accepted")
	}
	if os.Remove(filepath.Join(dir, "profile-data", other)) != nil || os.Rename(filepath.Join(dir, "profile-data", id), filepath.Join(dir, "profile-data", other)) != nil {
		t.Fatal("profile fixture")
	}
	if _, e = sdkCleanupPlan(dir); e == nil {
		t.Fatal("wrong current profile accepted")
	}
}
