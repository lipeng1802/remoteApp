package main

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"remoteapp.local/selfhost-poc/installrelease"
	"remoteapp.local/selfhost-poc/productprofile"
	"remoteapp.local/selfhost-poc/trustinstall"
	"remoteapp.local/selfhost-poc/trustprofile"
	"testing"
	"time"
)

// Optional offline artifact audit. No executable is started or OS state opened.
func TestBuiltLiveBundle(t *testing.T) {
	dir := os.Getenv("REMOTEAPP_LIVE_BUILD_AUDIT")
	if dir == "" {
		t.Skip("explicit fresh live TEST build required")
	}
	raw, e := os.ReadFile(filepath.Join(dir, "fixture.json"))
	if e != nil {
		t.Fatal(e)
	}
	var meta struct {
		Version  int    `json:"version"`
		Release  string `json:"release"`
		Platform string `json:"platform"`
		Product  string `json:"product_sha256"`
	}
	if json.Unmarshal(raw, &meta) != nil {
		t.Fatal("metadata")
	}
	r, e := trustinstall.DecodeCompiled(meta.Release)
	if e != nil {
		t.Fatal(e)
	}
	s := r.Specification()
	root, e := hex.DecodeString(s.Root)
	if e != nil || !s.Test || s.Service != "remoteapp-prerelease" {
		t.Fatal("wrong build trust")
	}
	if _, e = trustprofile.Verify(s.Bundled, root, s.Service, s.MinimumEpoch, time.Now()); e != nil {
		t.Fatal(e)
	}
	source, e := os.ReadFile(filepath.Join(dir, "verified-service-profile.json"))
	if e != nil {
		t.Fatal(e)
	}
	original, e := productprofile.Claims(source, time.Now())
	if e != nil {
		t.Fatal(e)
	}
	c := s.Bundled.Claims
	if c.Endpoint != original.Endpoint || c.Certificate != original.Certificate || c.Issuer != original.Issuer || c.Service != original.Service || c.Epoch != original.Epoch || c.ExpiresAt > original.ExpiresAt {
		t.Fatal("TEST trust differs from verified public source")
	}
	v, e := installrelease.VerifyDirectory(filepath.Join(dir, "payload"), root, s.Channel, meta.Platform, 1)
	if e != nil {
		t.Fatal(e)
	}
	if v.Manifest.Epoch != s.Bundled.Claims.Epoch {
		t.Fatal("manifest/profile epoch mismatch")
	}
	if _, e = installrelease.VerifyBootstrapDirectory(filepath.Join(dir, "bootstrap-payload"), root, s.Channel, meta.Platform, 1); e != nil {
		t.Fatal(e)
	}
	suffix := ""
	if meta.Platform == "windows" {
		suffix = ".exe"
	}
	for _, p := range []string{filepath.Join(dir, "payload", "trust-install"+suffix), filepath.Join(dir, "payload", "autonomous-enrollment-install"+suffix), filepath.Join(dir, "bootstrap-payload", "installed-release-launcher"+suffix)} {
		b, e := os.ReadFile(p)
		if e != nil || len(b) < 4096 {
			t.Fatal("missing actual compiled tool")
		}
		if meta.Platform == "windows" && string(b[:2]) != "MZ" {
			t.Fatal("wrong target executable")
		}
	}
	t.Log("PASS verified TEST public profile + signed manifests + actual offline tools; NOT desktop installation or network acceptance")
}
