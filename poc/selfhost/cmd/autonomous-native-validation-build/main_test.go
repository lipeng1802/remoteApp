package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrivateOutputOutsideBundleWithoutParentAliases(t *testing.T) {
	dir, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	dest := filepath.Join(dir, "client")
	if !privateOutputAllowed(dest, filepath.Join(dir, "server-private.json")) {
		t.Fatal("separate output")
	}
	if privateOutputAllowed(dest, filepath.Join(dest, "private.json")) {
		t.Fatal("inside output accepted")
	}
	alias := filepath.Join(dir, "alias")
	if os.Symlink(dir, alias) != nil {
		t.Fatal("symlink fixture")
	}
	if privateOutputAllowed(filepath.Join(alias, "client"), filepath.Join(dir, "server-private.json")) {
		t.Fatal("dest parent alias accepted")
	}
	if privateOutputAllowed(dest, filepath.Join(alias, "private.json")) {
		t.Fatal("TLS parent alias accepted")
	}
}

func TestTargetBuild(t *testing.T) {
	suffix, cgo, arch, e := targetBuild("windows")
	if e != nil || suffix != ".exe" || cgo != "0" || arch != "amd64" {
		t.Fatal("Windows target")
	}
	if _, _, _, e := targetBuild("linux"); e == nil {
		t.Fatal("unsupported target")
	}
	if suffix, cgo, _, e := targetBuild("darwin"); e != nil || suffix != "" || cgo != "1" {
		t.Fatal("Mac target")
	}
}
func TestPublicIssuerRejectsMalformedOrPrivateBootstrap(t *testing.T) {
	issuer := strings.Repeat("ab", 32)
	valid := `{"Version":1,"Issuer":"` + issuer + `"}`
	if _, e := publicIssuer([]byte(valid)); e != nil {
		t.Fatal(e)
	}
	for _, b := range []string{
		valid + `{}`, `{"Version":2,"Issuer":"` + issuer + `"}`,
		`{"Version":1,"Issuer":"` + strings.Repeat("00", 32) + `"}`,
		`{"Version":1,"Issuer":"AB"}`, `{"Version":1,"Issuer":"` + issuer + `","PrivateKey":"fixture"}`,
	} {
		if _, e := publicIssuer([]byte(b)); e == nil {
			t.Fatal("invalid bootstrap accepted")
		}
	}
}
