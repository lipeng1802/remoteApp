// Builds isolated desktop acceptance tools. No release/device private material
// is read: native test identities and release signatures are generated at run.
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
	if e := run(); e != nil {
		fmt.Println("BUILD_FAILED")
		os.Exit(1)
	}
	fmt.Println("BUILD_READY: isolated live owner desktop validation, not product installation")
}
func run() error {
	if len(os.Args) != 6 {
		return fmt.Errorf("arguments")
	}
	goexe, module, target, dest, helper := os.Args[1], os.Args[2], os.Args[3], os.Args[4], os.Args[5]
	for _, p := range []string{goexe, module, dest, helper} {
		if !filepath.IsAbs(p) || filepath.Clean(p) != p {
			return fmt.Errorf("path")
		}
	}
	if target != "darwin" && target != "windows" {
		return fmt.Errorf("target")
	}
	h, e := os.ReadFile(helper)
	if e != nil || len(h) == 0 {
		return fmt.Errorf("helper")
	}
	if e = os.Mkdir(dest, 0700); e != nil {
		return e
	} // never replaces an old kit
	suffix := ""
	arch := runtime.GOARCH
	if target == "windows" {
		suffix = ".exe"
		arch = "amd64"
	}
	for _, item := range []struct{ name, pkg string }{{"clientowner-tests", "./clientowner"}, {"streamgate-tests", "./streamgate"}, {"session-tests", "./session"}} {
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
		cmd := exec.CommandContext(ctx, goexe, "test", "-c", "-trimpath", "-o", filepath.Join(dest, item.name+suffix), item.pkg)
		cmd.Dir = module
		cmd.Env = append(os.Environ(), "GOENV=off", "GOTOOLCHAIN=local", "GOPROXY=off", "GOOS="+target, "GOARCH="+arch, "CGO_ENABLED=0")
		e = cmd.Run()
		cancel()
		if e != nil {
			return e
		}
	}
	if e = os.WriteFile(filepath.Join(dest, "trust-journal"+suffix), h, 0755); e != nil {
		return e
	}
	script, e := os.ReadFile(filepath.Join(module, "public-server/run-live-owner-validation.ps1"))
	if e != nil {
		return e
	}
	if e = os.WriteFile(filepath.Join(dest, "run-live-owner-validation.ps1"), script, 0600); e != nil {
		return e
	}
	manifest := map[string]string{}
	entries, e := os.ReadDir(dest)
	if e != nil {
		return e
	}
	for _, entry := range entries {
		b, e := os.ReadFile(filepath.Join(dest, entry.Name()))
		if e != nil {
			return e
		}
		sum := sha256.Sum256(b)
		manifest[entry.Name()] = hex.EncodeToString(sum[:])
	}
	b, e := json.MarshalIndent(manifest, "", "  ")
	if e != nil {
		return e
	}
	return os.WriteFile(filepath.Join(dest, "SHA256.json"), b, 0600)
}
