package main

import (
	"crypto/sha256"
	"debug/buildinfo"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"remoteapp.local/selfhost-poc/installrelease"
	"remoteapp.local/selfhost-poc/productprofile"
	"remoteapp.local/selfhost-poc/trustinstall"
	"remoteapp.local/selfhost-poc/trustprofile"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestBuiltOfflineInstallationBundle(t *testing.T) {
	dir := os.Getenv("REMOTEAPP_OFFLINE_BUILD_AUDIT")
	if dir == "" {
		t.Skip("explicit offline TEST build required")
	}
	raw, err := os.ReadFile(filepath.Join(dir, "fixture.json"))
	if err != nil {
		t.Fatal(err)
	}
	var meta struct {
		Release  string `json:"release"`
		Platform string `json:"platform"`
		Product  string `json:"product_sha256"`
	}
	if json.Unmarshal(raw, &meta) != nil {
		t.Fatal("invalid metadata")
	}
	r, err := trustinstall.DecodeCompiled(meta.Release)
	if err != nil {
		t.Fatal(err)
	}
	s := r.Specification()
	if !s.Test || s.Service != "product-exit-test" || s.Bundled.Claims.Endpoint != "https://product-exit.example:8444" {
		t.Fatal("not offline TEST trust")
	}
	public, err := hex.DecodeString(s.Root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = installrelease.VerifyDirectory(filepath.Join(dir, "payload"), public, s.Channel, meta.Platform, 1); err != nil {
		t.Fatal(err)
	}
	if _, err = installrelease.VerifyBootstrapDirectory(filepath.Join(dir, "bootstrap-payload"), public, s.Channel, meta.Platform, 1); err != nil {
		t.Fatal(err)
	}
	suffix := ""
	if meta.Platform == "windows" {
		suffix = ".exe"
	}
	for _, name := range []string{"supervisor", "retire-node-test", "supervised-control-relay"} {
		if _, err = os.Stat(filepath.Join(dir, name+suffix)); !os.IsNotExist(err) {
			t.Fatal("offline fixture contains online orchestration")
		}
	}
	for _, name := range []string{"test-setup", "retire-idle-test", "payload/trust-install", "payload/autonomous-enrollment-install", "bootstrap-payload/installed-release-launcher"} {
		b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(name)+suffix))
		if err != nil || len(b) < 4096 || (meta.Platform == "windows" && string(b[:2]) != "MZ") {
			t.Fatal("missing actual native tool", name)
		}
	}
	b, err := os.ReadFile(filepath.Join(dir, "payload", "installed-autonomous-product"+suffix))
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(b)
	if hex.EncodeToString(h[:]) != meta.Product {
		t.Fatal("product pin mismatch")
	}
}

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
		Desktop  string `json:"desktop_permission"`
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
	expectedService := "remoteapp-prerelease"
	if meta.Desktop != "" {
		expectedService = productprofile.DesktopService
	}
	if e != nil || !s.Test || s.Service != expectedService {
		t.Fatal("wrong build trust")
	}
	if _, e = trustprofile.Verify(s.Bundled, root, s.Service, s.MinimumEpoch, time.Now()); e != nil {
		t.Fatal(e)
	}
	source, e := os.ReadFile(filepath.Join(dir, "verified-service-profile.json"))
	if e != nil {
		t.Fatal(e)
	}
	var original trustprofile.Claims
	if meta.Desktop == "" {
		original, e = productprofile.Claims(source, time.Now())
	} else {
		original, e = productprofile.DesktopClaims(source, os.Getenv("REMOTEAPP_DESKTOP_BUILD_AUDIT_ROOT"), time.Now())
	}
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
	productPath := filepath.Join(dir, "payload", "installed-autonomous-product"+suffix)
	product, e := os.ReadFile(productPath)
	if e != nil || digest(product) != meta.Product {
		t.Fatal("product pin mismatch")
	}
	if meta.Desktop != "" {
		var expectedTag string
		switch meta.Desktop {
		case "desktop-view":
			expectedTag = "remoteapp_desktop_view"
		case "desktop-control":
			expectedTag = "remoteapp_desktop_control"
		default:
			t.Fatal("unknown desktop scope")
		}
		for _, path := range []string{productPath, filepath.Join(dir, "supervisor"+suffix)} {
			info, err := buildinfo.ReadFile(path)
			if err != nil {
				t.Fatal("desktop compiler metadata missing")
			}
			found := false
			for _, v := range info.Settings {
				if v.Key == "-tags" && slices.Contains(strings.Split(v.Value, ","), expectedTag) {
					found = true
				}
			}
			if !found {
				t.Fatal("desktop compile scope missing")
			}
		}
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
