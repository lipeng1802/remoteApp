// Fresh offline Windows TEST bundle. No server, SSH, production trust or keys.
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
	"time"
)

func manifest(root string) ([]byte, error) {
	hashes := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return os.ErrInvalid
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			return os.ErrInvalid
		}
		name, err := filepath.Rel(root, path)
		if err != nil || name == "SHA256.json" {
			return os.ErrInvalid
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		h := sha256.Sum256(b)
		hashes[filepath.ToSlash(name)] = hex.EncodeToString(h[:])
		return nil
	})
	if err != nil {
		return nil, err
	}
	return json.Marshal(hashes)
}

func run() error {
	if len(os.Args) != 4 {
		return os.ErrInvalid
	}
	goexe, module, dest := os.Args[1], os.Args[2], os.Args[3]
	for _, p := range []string{goexe, module, dest} {
		if !filepath.IsAbs(p) || filepath.Clean(p) != p {
			return os.ErrInvalid
		}
	}
	if os.Mkdir(dest, 0700) != nil {
		return os.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	command := func(windows bool, args ...string) error {
		c := exec.CommandContext(ctx, goexe, args...)
		c.Dir = module
		c.Env = append(os.Environ(), "GOENV=off", "GOTOOLCHAIN=local", "GOPROXY=off")
		if windows {
			c.Env = append(c.Env, "GOOS=windows", "GOARCH=amd64", "CGO_ENABLED=0")
		}
		return c.Run() // Keep compiler/profile material out of diagnostics.
	}
	// Builder-only helpers outside the distributable directory, retained on error.
	work, err := os.MkdirTemp(filepath.Dir(dest), "windows-preflight-build-")
	if err != nil {
		return err
	}
	journal, signer := filepath.Join(work, "trust-journal.exe"), filepath.Join(work, "identity-vault.exe")
	for path, pkg := range map[string]string{journal: "./cmd/trust-journal", signer: "./cmd/identity-vault"} {
		if command(true, "build", "-trimpath", "-o", path, pkg) != nil {
			return os.ErrInvalid
		}
	}
	if command(false, "run", "./cmd/enrollment-storage-validation-build", goexe, module, "windows", filepath.Join(dest, "storage")) != nil {
		return os.ErrInvalid
	}
	if command(false, "run", "./cmd/enrollment-registration-validation-build", goexe, module, "windows", filepath.Join(dest, "registration"), journal, signer) != nil {
		return os.ErrInvalid
	}
	if command(true, "build", "-trimpath", "-o", filepath.Join(dest, "enrollment-sdk-validation.exe"), "./cmd/enrollment-sdk-validation") != nil {
		return os.ErrInvalid
	}
	b, err := os.ReadFile(filepath.Join(module, "public-server/run-windows-enrollment-preflight.ps1"))
	if err != nil {
		return err
	}
	if err = write(filepath.Join(dest, "run-windows-enrollment-preflight.ps1"), b); err != nil {
		return err
	}
	b, err = manifest(dest)
	if err != nil {
		return err
	}
	return write(filepath.Join(dest, "SHA256.json"), b)
}

func write(path string, b []byte) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, err = f.Write(b)
	synced, closed := f.Sync(), f.Close()
	if err != nil {
		return err
	}
	if synced != nil {
		return synced
	}
	return closed
}
func main() {
	if run() != nil {
		fmt.Println("BUILD FAILED windows_native_preflight; preserve TEST files")
		os.Exit(1)
	}
	fmt.Println("BUILD READY windows_native_preflight; NOT Windows execution or server acceptance")
}
