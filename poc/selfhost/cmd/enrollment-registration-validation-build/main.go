package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"remoteapp.local/selfhost-poc/devicevalidation"
	"remoteapp.local/selfhost-poc/trustinstall"
)

func main() {
	if run() != nil {
		fmt.Println("BUILD_FAILED native registration TEST bundle preserved")
		os.Exit(1)
	}
	fmt.Println("BUILD_READY fresh native registration combination TEST bundle")
}
func write(p string, b []byte) error {
	f, e := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0755)
	if e != nil {
		return e
	}
	_, e = f.Write(b)
	if e == nil {
		e = f.Sync()
	}
	if next := f.Close(); e == nil {
		e = next
	}
	return e
}
func run() error {
	if len(os.Args) != 7 {
		return os.ErrInvalid
	}
	goexe, module, target, dest, journalPath, signerPath := os.Args[1], os.Args[2], os.Args[3], os.Args[4], os.Args[5], os.Args[6]
	for _, p := range []string{goexe, module, dest, journalPath, signerPath} {
		if !filepath.IsAbs(p) || filepath.Clean(p) != p {
			return os.ErrInvalid
		}
	}
	if target != "darwin" && target != "windows" {
		return os.ErrInvalid
	}
	j, e := os.ReadFile(journalPath)
	if e != nil || len(j) == 0 || len(j) > 100<<20 {
		return os.ErrInvalid
	}
	s, e := os.ReadFile(signerPath)
	if e != nil || len(s) == 0 || len(s) > 100<<20 {
		return os.ErrInvalid
	}
	if os.Mkdir(dest, 0700) != nil {
		return os.ErrInvalid
	}
	spec, fixture, e := devicevalidation.NewNetworkMaterial(j, time.Now())
	if e != nil {
		return e
	}
	encoded, e := trustinstall.EncodeForTrustedBuild(spec, time.Now())
	if e != nil {
		return e
	}
	h := sha256.Sum256(s)
	digest := hex.EncodeToString(h[:])
	raw, _ := json.Marshal(fixture)
	suffix, cgo, arch := "", "1", runtime.GOARCH
	if target == "windows" {
		suffix = ".exe"
		cgo = "0"
		arch = "amd64"
	}
	if write(filepath.Join(dest, "trust-journal"+suffix), j) != nil || write(filepath.Join(dest, "identity-vault"+suffix), s) != nil {
		return os.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	flags := "-X main.embeddedRelease=" + encoded + " -X main.embeddedSignerDigest=" + digest + " -X main.embeddedFixtureTLS=" + base64.StdEncoding.EncodeToString(raw)
	c := exec.CommandContext(ctx, goexe, "build", "-trimpath", "-ldflags="+flags, "-o", filepath.Join(dest, "enrollment-registration-validation"+suffix), "./cmd/enrollment-registration-validation")
	c.Dir = module
	c.Env = append(os.Environ(), "GOENV=off", "GOTOOLCHAIN=local", "GOPROXY=off", "GOOS="+target, "GOARCH="+arch, "CGO_ENABLED="+cgo)
	if c.Run() != nil {
		return os.ErrInvalid
	}
	if target == "windows" {
		b, e := os.ReadFile(filepath.Join(module, "public-server/run-enrollment-registration-validation.ps1"))
		if e != nil || write(filepath.Join(dest, "run-enrollment-registration-validation.ps1"), b) != nil {
			return os.ErrInvalid
		}
	}
	m := map[string]string{}
	entries, e := os.ReadDir(dest)
	if e != nil {
		return e
	}
	for _, f := range entries {
		b, e := os.ReadFile(filepath.Join(dest, f.Name()))
		if e != nil {
			return e
		}
		h := sha256.Sum256(b)
		m[f.Name()] = hex.EncodeToString(h[:])
	}
	b, _ := json.Marshal(m)
	return write(filepath.Join(dest, "SHA256.json"), b)
}
