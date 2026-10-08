// Generates isolated PUBLIC fixtures and compiled test-channel tools.
// Signing keys exist only in memory, never exported or read from release files.
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
	"time"

	"remoteapp.local/selfhost-poc/trustinstall"
	"remoteapp.local/selfhost-poc/trustprofile"
)

func write(path string, b []byte, mode os.FileMode) error {
	f, e := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if e != nil {
		return e
	}
	_, e = f.Write(b)
	if e == nil {
		e = f.Sync()
	}
	if closed := f.Close(); e == nil {
		e = closed
	}
	return e
}
func main() {
	if e := run(); e != nil {
		fmt.Println("BUILD_FAILED public test bundle preserved, no installation")
		os.Exit(1)
	}
	fmt.Println("BUILD_READY isolated compiled-anchor native validation bundle")
}
func run() error {
	// Explicit Go executable, module, target, new directory, trusted helper.
	if len(os.Args) != 6 {
		return os.ErrInvalid
	}
	goexe, module, target, dest, helperPath := os.Args[1], os.Args[2], os.Args[3], os.Args[4], os.Args[5]
	for _, p := range []string{goexe, module, dest, helperPath} {
		if !filepath.IsAbs(p) || filepath.Clean(p) != p {
			return os.ErrInvalid
		}
	}
	if target != "darwin" && target != "windows" {
		return os.ErrInvalid
	}
	helper, e := os.ReadFile(helperPath)
	if e != nil || len(helper) == 0 || len(helper) > 100<<20 {
		return os.ErrInvalid
	}
	if os.Mkdir(dest, 0700) != nil {
		return os.ErrInvalid
	}
	suffix := ""
	if target == "windows" {
		suffix = ".exe"
	}
	if write(filepath.Join(dest, "trust-journal"+suffix), helper, 0755) != nil {
		return os.ErrInvalid
	}
	now := time.Now().Truncate(time.Second)
	root, key, e := ed25519.GenerateKey(rand.Reader)
	if e != nil {
		return e
	}
	other, otherKey, e := ed25519.GenerateKey(rand.Reader)
	if e != nil {
		return e
	}
	pub, caKey, e := ed25519.GenerateKey(rand.Reader)
	if e != nil {
		return e
	}
	issuer, _, e := ed25519.GenerateKey(rand.Reader)
	if e != nil {
		return e
	}
	ca := &x509.Certificate{SerialNumber: big.NewInt(1), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(24 * time.Hour)}
	der, e := x509.CreateCertificate(rand.Reader, ca, ca, pub, caKey)
	if e != nil {
		return e
	}
	claims := trustprofile.Claims{Version: 1, Service: "remoteapp-install-validation", Epoch: 1, Endpoint: "https://install-validation.example:8444", Certificate: base64.StdEncoding.EncodeToString(der), Issuer: hex.EncodeToString(issuer), IssuedAt: now.Add(-time.Minute).Unix(), ExpiresAt: now.Add(23 * time.Hour).Unix()}
	p1, e := trustprofile.Sign(key, claims)
	if e != nil {
		return e
	}
	claims.Epoch = 2
	p2, _ := trustprofile.Sign(key, claims)
	var nonce [16]byte
	if _, e = rand.Read(nonce[:]); e != nil {
		return e
	}
	h := sha256.Sum256(helper)
	s := trustinstall.Specification{Version: 1, Channel: "validation-" + hex.EncodeToString(nonce[:]), Root: hex.EncodeToString(root), Service: claims.Service, MinimumEpoch: 1, HelperDigest: hex.EncodeToString(h[:]), Bundled: p1, Test: true}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	if e = trustinstall.Compile(ctx, goexe, module, target, filepath.Join(dest, "trust-install-v1"+suffix), s, now); e != nil {
		return e
	}
	s.MinimumEpoch = 2
	s.Bundled = p2
	if e = trustinstall.Compile(ctx, goexe, module, target, filepath.Join(dest, "trust-install-v2"+suffix), s, now); e != nil {
		return e
	}
	otherProfile, _ := trustprofile.Sign(otherKey, claims)
	s.Root = hex.EncodeToString(other)
	s.Bundled = otherProfile
	if e = trustinstall.Compile(ctx, goexe, module, target, filepath.Join(dest, "trust-install-other-root"+suffix), s, now); e != nil {
		return e
	}
	profiles := map[string]trustprofile.Profile{"old-profile.json": p1, "other-root-profile.json": otherProfile}
	mutation := claims
	mutation.Endpoint = "https://mutated.example:8444"
	profiles["same-epoch-profile.json"], _ = trustprofile.Sign(key, mutation)
	mutation = claims
	mutation.Service = "other-service"
	profiles["other-service-profile.json"], _ = trustprofile.Sign(key, mutation)
	mutation = claims
	mutation.Epoch = 3
	mutation.IssuedAt = now.Add(-2 * time.Minute).Unix()
	mutation.ExpiresAt = now.Add(-time.Minute).Unix()
	profiles["expired-profile.json"], _ = trustprofile.Sign(key, mutation)
	for name, p := range profiles {
		b, e := json.Marshal(p)
		if e != nil || write(filepath.Join(dest, name), b, 0600) != nil {
			return os.ErrInvalid
		}
	}
	// Runtime MUST ignore this untrusted sidecar root. It has no authority.
	b, _ := json.Marshal(trustinstall.PublicAnchor{Root: hex.EncodeToString(other), Service: claims.Service, MinimumEpoch: 1})
	if write(filepath.Join(dest, "installation-anchor.json"), b, 0600) != nil {
		return os.ErrInvalid
	}
	if os.Mkdir(filepath.Join(dest, "tampered"), 0700) != nil {
		return os.ErrInvalid
	}
	program, e := os.ReadFile(filepath.Join(dest, "trust-install-v1"+suffix))
	if e != nil {
		return e
	}
	if write(filepath.Join(dest, "tampered", "trust-install-v1"+suffix), program, 0755) != nil {
		return os.ErrInvalid
	}
	if write(filepath.Join(dest, "tampered", "trust-journal"+suffix), append(append([]byte(nil), helper...), 0), 0755) != nil {
		return os.ErrInvalid
	}
	env := append(os.Environ(), "GOENV=off", "GOTOOLCHAIN=local", "GOPROXY=off", "GOOS="+target, "CGO_ENABLED=0")
	if target == "windows" {
		env = append(env, "GOARCH=amd64")
	} else {
		env = append(env, "GOARCH="+runtime.GOARCH)
	}
	check := exec.CommandContext(ctx, goexe, "build", "-trimpath", "-o", filepath.Join(dest, "trust-install-validation"+suffix), "./cmd/trust-install-validation")
	check.Dir = module
	check.Env = env
	if check.Run() != nil {
		return os.ErrInvalid
	}
	if target == "windows" {
		script, e := os.ReadFile(filepath.Join(module, "public-server", "run-trust-install-validation.ps1"))
		if e != nil {
			return e
		}
		if write(filepath.Join(dest, "run-trust-install-validation.ps1"), script, 0600) != nil {
			return os.ErrInvalid
		}
	}
	checks := map[string]string{}
	e = filepath.WalkDir(dest, func(path string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() {
			return nil
		}
		b, e := os.ReadFile(path)
		if e != nil {
			return e
		}
		sum := sha256.Sum256(b)
		name, e := filepath.Rel(dest, path)
		if e != nil {
			return e
		}
		checks[filepath.ToSlash(name)] = hex.EncodeToString(sum[:])
		return nil
	})
	if e != nil {
		return e
	}
	b, e = json.Marshal(checks)
	if e != nil {
		return e
	}
	return write(filepath.Join(dest, "SHA256.json"), b, 0600)
}
