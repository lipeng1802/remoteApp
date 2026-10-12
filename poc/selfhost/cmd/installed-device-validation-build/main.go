// Explicit TEST-only builder. Release/CA private keys remain in build memory;
// only a throwaway loopback fixture leaf key is compiled into the validator.
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
		fmt.Println("BUILD_FAILED isolated test bundle preserved")
		os.Exit(1)
	}
	fmt.Println("BUILD_READY isolated actual installed device native validation bundle")
}

func write(path string, b []byte, mode os.FileMode) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	_, err = f.Write(b)
	if err == nil {
		err = f.Sync()
	}
	if closed := f.Close(); err == nil {
		err = closed
	}
	return err
}

func run() error {
	if len(os.Args) != 7 {
		return os.ErrInvalid
	}
	goexe, module, target, dest, journalPath, signerPath := os.Args[1], os.Args[2], os.Args[3], os.Args[4], os.Args[5], os.Args[6]
	for _, path := range []string{goexe, module, dest, journalPath, signerPath} {
		if !filepath.IsAbs(path) || filepath.Clean(path) != path {
			return os.ErrInvalid
		}
	}
	if target != "darwin" && target != "windows" {
		return os.ErrInvalid
	}
	journal, err := os.ReadFile(journalPath)
	if err != nil || len(journal) == 0 || len(journal) > 100<<20 {
		return os.ErrInvalid
	}
	signer, err := os.ReadFile(signerPath)
	if err != nil || len(signer) == 0 || len(signer) > 100<<20 {
		return os.ErrInvalid
	}
	if os.Mkdir(dest, 0700) != nil {
		return os.ErrInvalid
	}
	spec, fixture, err := devicevalidation.NewMaterial(journal, time.Now())
	if err != nil {
		return err
	}
	encoded, err := trustinstall.EncodeForTrustedBuild(spec, time.Now())
	if err != nil {
		return err
	}
	sum := sha256.Sum256(signer)
	signerDigest := hex.EncodeToString(sum[:])
	suffix := ""
	if target == "windows" {
		suffix = ".exe"
	}
	if write(filepath.Join(dest, "trust-journal"+suffix), journal, 0755) != nil || write(filepath.Join(dest, "identity-vault"+suffix), signer, 0755) != nil {
		return os.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	client := filepath.Join(dest, "installed-device-client"+suffix)
	if trustinstall.CompileDeviceClient(ctx, goexe, module, target, client, spec, signerDigest, time.Now()) != nil {
		return os.ErrInvalid
	}
	b, err := os.ReadFile(client)
	if err != nil {
		return err
	}
	sum = sha256.Sum256(b)
	clientDigest := hex.EncodeToString(sum[:])
	raw, err := json.Marshal(fixture)
	if err != nil {
		return err
	}
	flags := "-X main.embeddedRelease=" + encoded + " -X main.embeddedSignerDigest=" + signerDigest + " -X main.embeddedClientDigest=" + clientDigest + " -X main.embeddedFixtureTLS=" + base64.StdEncoding.EncodeToString(raw)
	command := exec.CommandContext(ctx, goexe, "build", "-trimpath", "-ldflags="+flags, "-o", filepath.Join(dest, "installed-device-validation"+suffix), "./cmd/installed-device-validation")
	command.Dir = module
	arch := runtime.GOARCH
	if target == "windows" {
		arch = "amd64"
	}
	command.Env = append(os.Environ(), "GOENV=off", "GOTOOLCHAIN=local", "GOPROXY=off", "CGO_ENABLED=0", "GOOS="+target, "GOARCH="+arch)
	if command.Run() != nil {
		return os.ErrInvalid
	}
	if target == "windows" {
		b, err = os.ReadFile(filepath.Join(module, "public-server/run-installed-device-validation.ps1"))
		if err != nil {
			return err
		}
		if write(filepath.Join(dest, "run-installed-device-validation.ps1"), b, 0600) != nil {
			return os.ErrInvalid
		}
	}
	manifest := make(map[string]string)
	entries, err := os.ReadDir(dest)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		b, err = os.ReadFile(filepath.Join(dest, entry.Name()))
		if err != nil {
			return err
		}
		sum := sha256.Sum256(b)
		manifest[entry.Name()] = hex.EncodeToString(sum[:])
	}
	b, err = json.Marshal(manifest)
	if err != nil {
		return err
	}
	return write(filepath.Join(dest, "SHA256.json"), b, 0600)
}
