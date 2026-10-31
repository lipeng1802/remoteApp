package testretirement

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"remoteapp.local/selfhost-poc/trustinstall"
	"runtime"
	"testing"
)

func TestProductionRetirementRefused(t *testing.T) {
	if Registered(context.Background(), trustinstall.Release{}, "/no-test-root", "/no-helper", "/no-vault", "", false) == nil {
		t.Fatal("production registered teardown allowed")
	}
	if Idle(trustinstall.Release{}, "/no-test-root", "/no-helper", "/no-vault", "") == nil {
		t.Fatal("production/default release allowed teardown")
	}
	if FinishFiles(trustinstall.Release{}, "/no-test-root", "/no-helper") == nil {
		t.Fatal("production allowed file recovery")
	}
}
func TestNativeLeasePreventsRetirement(t *testing.T) {
	dir := os.Getenv("REMOTEAPP_IDLE_RETIRE_FIXTURE")
	if dir == "" {
		t.Skip("explicit retained TEST fixture required")
	}
	raw, e := os.ReadFile(filepath.Join(dir, "fixture.json"))
	if e != nil {
		t.Fatal(e)
	}
	var meta struct {
		Release string `json:"release"`
	}
	if json.Unmarshal(raw, &meta) != nil {
		t.Fatal("fixture")
	}
	r, e := trustinstall.DecodeCompiled(meta.Release)
	if e != nil {
		t.Fatal(e)
	}
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	helper := filepath.Join(dir, "trust-journal"+suffix)
	factory, e := r.NativeFactory(helper)
	if e != nil {
		t.Fatal(e)
	}
	lease, e := factory(false)
	if e != nil {
		t.Fatal(e)
	}
	defer lease.Close()
	root := os.Getenv("REMOTEAPP_IDLE_RETIRE_ROOT")
	vault := filepath.Join(dir, "identity-vault"+suffix)
	vaultRaw, e := os.ReadFile(vault)
	if e != nil {
		t.Fatal(e)
	}
	digest := sha256.Sum256(vaultRaw)
	if Idle(r, root, helper, vault, hex.EncodeToString(digest[:])) == nil {
		t.Fatal("retirement bypassed held trust lease")
	}
	if _, e = os.Stat(root); e != nil {
		t.Fatal("root changed despite live owner")
	}
	if _, e = lease.Load(); e != nil {
		t.Fatal("retirement affected live lease")
	}
}
