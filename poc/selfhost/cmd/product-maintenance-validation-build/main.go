// Builds a single-release TEST fixture for actual product process-exit tests.
// The throwaway signing key is never written. Legacy idle mode uses marked
// placeholders; verified-live-profile mode compiles actual offline tools.
// Both modes remain TEST-only, not a signed product installer.
package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"remoteapp.local/selfhost-poc/installrelease"
	"remoteapp.local/selfhost-poc/productprofile"
	"remoteapp.local/selfhost-poc/trustinstall"
	"remoteapp.local/selfhost-poc/trustprofile"
)

func write(path string, raw []byte, mode os.FileMode) error {
	f, e := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if e != nil {
		return e
	}
	_, e = f.Write(raw)
	if c := f.Close(); e == nil {
		e = c
	}
	return e
}
func digest(raw []byte) string { h := sha256.Sum256(raw); return hex.EncodeToString(h[:]) }
func run() error {
	if len(os.Args) != 8 && ((len(os.Args) != 10 && len(os.Args) != 11 && len(os.Args) != 13) || os.Args[8] != "--verified-live-profile") {
		return os.ErrInvalid
	}
	goexe, module, target, dest, journal, vault, tlsProduct := os.Args[1], os.Args[2], os.Args[3], os.Args[4], os.Args[5], os.Args[6], os.Args[7]
	live := len(os.Args) >= 10
	tunnel := len(os.Args) == 11 || len(os.Args) == 13
	if tunnel && os.Args[10] != "--test-tunnel" {
		return os.ErrInvalid
	}
	if len(os.Args) == 13 && os.Args[11] != "--test-install-root" {
		return os.ErrInvalid
	}
	var liveClaims trustprofile.Claims
	var liveSource []byte
	if live {
		if !filepath.IsAbs(os.Args[9]) || filepath.Clean(os.Args[9]) != os.Args[9] {
			return os.ErrInvalid
		}
		raw, e := os.ReadFile(os.Args[9])
		if e != nil {
			return e
		}
		liveClaims, e = productprofile.Claims(raw, time.Now())
		if e != nil {
			return e
		}
		liveSource = raw
	}
	for _, p := range []string{goexe, module, dest, journal, vault, tlsProduct} {
		if !filepath.IsAbs(p) || filepath.Clean(p) != p {
			return os.ErrInvalid
		}
	}
	if target != "darwin" && target != "windows" {
		return os.ErrInvalid
	}
	if target == "darwin" && runtime.GOOS != "darwin" {
		return os.ErrInvalid
	}
	if e := os.Mkdir(dest, 0700); e != nil {
		return e
	}
	if live {
		if e := write(filepath.Join(dest, "verified-service-profile.json"), liveSource, 0400); e != nil {
			return e
		}
	}
	journalRaw, e := os.ReadFile(journal)
	if e != nil {
		return e
	}
	vaultRaw, e := os.ReadFile(vault)
	if e != nil {
		return e
	}
	tlsRaw, e := os.ReadFile(tlsProduct)
	if e != nil {
		return e
	}
	public, key, e := ed25519.GenerateKey(rand.Reader)
	if e != nil {
		return e
	}
	defer func() {
		for i := range key {
			key[i] = 0
		}
	}()
	cp, ck, _ := ed25519.GenerateKey(rand.Reader)
	issuer, _, _ := ed25519.GenerateKey(rand.Reader)
	now := time.Now()
	ca := &x509.Certificate{SerialNumber: big.NewInt(1), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(24 * time.Hour)}
	der, e := x509.CreateCertificate(rand.Reader, ca, ca, cp, ck)
	if e != nil {
		return e
	}
	claims := trustprofile.Claims{Version: 1, Service: "product-exit-test", Epoch: 2, Endpoint: "https://product-exit.example:8444", Certificate: base64.StdEncoding.EncodeToString(der), Issuer: hex.EncodeToString(issuer), IssuedAt: now.Add(-time.Minute).Unix(), ExpiresAt: now.Add(12 * time.Hour).Unix()}
	if live {
		claims = liveClaims
	}
	profile, e := trustprofile.Sign(key, claims)
	if e != nil {
		return e
	}
	nonce := make([]byte, 8)
	rand.Read(nonce)
	spec := trustinstall.Specification{Version: 1, Channel: "product-exit-" + hex.EncodeToString(nonce), Root: hex.EncodeToString(public), Service: claims.Service, MinimumEpoch: 1, HelperDigest: digest(journalRaw), Bundled: profile, Test: true}
	encoded, e := trustinstall.EncodeForTrustedBuild(spec, now)
	if e != nil {
		return e
	}
	payload := filepath.Join(dest, "payload")
	bootstrap := filepath.Join(dest, "bootstrap-payload")
	if os.Mkdir(payload, 0700) != nil || os.Mkdir(bootstrap, 0700) != nil {
		return os.ErrInvalid
	}
	suffix := ""
	if target == "windows" {
		suffix = ".exe"
	}
	for name, raw := range map[string][]byte{"trust-journal": journalRaw, "identity-vault": vaultRaw, "tls-product": tlsRaw, "trust-install": []byte("TEST unused installer placeholder\n"), "autonomous-enrollment-install": []byte("TEST unused installer placeholder\n")} {
		if e = write(filepath.Join(payload, name+suffix), raw, 0500); e != nil {
			return e
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	product := filepath.Join(payload, "installed-autonomous-product"+suffix)
	if live {
		// Replace TEST placeholders with real offline native provisioning tools.
		for _, name := range []string{"trust-install", "autonomous-enrollment-install"} {
			if e = os.Remove(filepath.Join(payload, name+suffix)); e != nil {
				return e
			}
		}
		if e = trustinstall.Compile(ctx, goexe, module, target, filepath.Join(payload, "trust-install"+suffix), spec, now); e != nil {
			return e
		}
		if e = trustinstall.CompileAutonomousEnrollmentInstall(ctx, goexe, module, target, filepath.Join(payload, "autonomous-enrollment-install"+suffix), spec, digest(vaultRaw), now); e != nil {
			return e
		}
	}
	compileProduct := trustinstall.CompileAutonomousProductClient
	if tunnel {
		compileProduct = trustinstall.CompileAutonomousProductTestTunnel
	}
	if e = compileProduct(ctx, goexe, module, target, product, spec, digest(vaultRaw), digest(tlsRaw), now); e != nil {
		return e
	}
	if target == "darwin" {
		if out, e := exec.Command("/usr/bin/codesign", "--force", "--sign", "-", product).CombinedOutput(); e != nil {
			return fmt.Errorf("TEST signing: %v %s", e, out)
		}
	}
	m, e := installrelease.BuildManifest(payload, spec.Channel, target, claims.Epoch)
	if e != nil {
		return e
	}
	raw, e := installrelease.SignForBuild(key, m)
	if e != nil {
		return e
	}
	if e = write(filepath.Join(payload, installrelease.ManifestName), raw, 0400); e != nil {
		return e
	}
	if e = write(filepath.Join(bootstrap, "trust-journal"+suffix), journalRaw, 0500); e != nil {
		return e
	}
	if live {
		e = trustinstall.CompileInstalledReleaseLauncher(ctx, goexe, module, target, filepath.Join(bootstrap, "installed-release-launcher"+suffix), spec, now)
	} else {
		e = write(filepath.Join(bootstrap, "installed-release-launcher"+suffix), []byte("TEST unused launcher placeholder\n"), 0500)
	}
	if e != nil {
		return e
	}
	bm, e := installrelease.BuildBootstrapManifest(bootstrap, spec.Channel, target, claims.Epoch)
	if e != nil {
		return e
	}
	raw, e = installrelease.SignBootstrapForBuild(key, bm)
	if e != nil {
		return e
	}
	if e = write(filepath.Join(bootstrap, installrelease.BootstrapManifestName), raw, 0400); e != nil {
		return e
	}
	productRaw, e := os.ReadFile(product)
	if e != nil {
		return e
	}
	if len(os.Args) == 13 {
		installRoot := os.Args[12]
		verified, e := installrelease.VerifyDirectory(payload, public, spec.Channel, target, 1)
		if e != nil {
			return e
		}
		name, e := installrelease.ReleaseDirectoryName(verified)
		if e != nil {
			return e
		}
		productPath := filepath.Join(installRoot, "releases", name, "installed-autonomous-product"+suffix)
		if target == "windows" {
			productPath = strings.TrimRight(installRoot, `\`) + `\releases\` + name + `\installed-autonomous-product.exe`
		}
		for name, raw := range map[string][]byte{"trust-journal": journalRaw, "identity-vault": vaultRaw} {
			if e = write(filepath.Join(dest, name+suffix), raw, 0500); e != nil {
				return e
			}
		}
		if e = trustinstall.CompileTestProductSetup(ctx, goexe, module, target, filepath.Join(dest, "test-setup"+suffix), spec, installRoot, now); e != nil {
			return e
		}
		if e = trustinstall.CompileProductSupervisorTest(ctx, goexe, module, target, filepath.Join(dest, "supervisor"+suffix), spec, productPath, digest(productRaw), digest(vaultRaw), now); e != nil {
			return e
		}
		if e = trustinstall.CompileIdleTestRetirement(ctx, goexe, module, target, filepath.Join(dest, "retire-idle-test"+suffix), spec, installRoot, digest(vaultRaw), now); e != nil {
			return e
		}
	}
	meta := struct {
		Version       int    `json:"version"`
		Release       string `json:"release"`
		Platform      string `json:"platform"`
		ProductSHA256 string `json:"product_sha256"`
	}{1, encoded, target, digest(productRaw)}
	raw, e = json.Marshal(meta)
	if e != nil {
		return e
	}
	return write(filepath.Join(dest, "fixture.json"), raw, 0600)
}
func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
