// Explicit fresh TEST bundle builder; no production trust or private keys.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

func main() {
	if run() != nil {
		fmt.Println("BUILD_FAILED native enrollment TEST bundle preserved")
		os.Exit(1)
	}
	fmt.Println("BUILD_READY fresh native enrollment storage TEST bundle")
}
func run() error {
	if len(os.Args) != 5 {
		return os.ErrInvalid
	}
	goexe, module, target, dest := os.Args[1], os.Args[2], os.Args[3], os.Args[4]
	for _, p := range []string{goexe, module, dest} {
		if !filepath.IsAbs(p) || filepath.Clean(p) != p {
			return os.ErrInvalid
		}
	}
	if target != "darwin" && target != "windows" {
		return os.ErrInvalid
	}
	if os.Mkdir(dest, 0700) != nil {
		return os.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	suffix, cgo := "", "1"
	arch := runtime.GOARCH
	if target == "windows" {
		suffix = ".exe"
		cgo = "0"
		arch = "amd64"
	}
	build := func(args ...string) error {
		c := exec.CommandContext(ctx, goexe, args...)
		c.Dir = module
		c.Env = append(os.Environ(), "GOENV=off", "GOTOOLCHAIN=local", "GOPROXY=off", "GOOS="+target, "GOARCH="+arch, "CGO_ENABLED="+cgo)
		return c.Run() // no compiler command/body or environment diagnostics
	}
	if build("build", "-trimpath", "-o", filepath.Join(dest, "enrollment-storage-validation"+suffix), "./cmd/enrollment-storage-validation") != nil {
		return os.ErrInvalid
	}
	if build("test", "-c", "-o", filepath.Join(dest, "enrollment-storage-tests"+suffix), "./enrollmentstore") != nil {
		return os.ErrInvalid
	}
	if target == "windows" {
		b, e := os.ReadFile(filepath.Join(module, "public-server/run-enrollment-storage-validation.ps1"))
		if e != nil {
			return e
		}
		if os.WriteFile(filepath.Join(dest, "run-enrollment-storage-validation.ps1"), b, 0600) != nil {
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
	b, e := json.Marshal(m)
	if e != nil {
		return e
	}
	return os.WriteFile(filepath.Join(dest, "SHA256.json"), b, 0600)
}
