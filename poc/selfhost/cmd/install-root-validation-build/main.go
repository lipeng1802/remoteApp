// Builds a native-only isolated install-root acceptance bundle.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"remoteapp.local/selfhost-poc/trustinstall"
)

func digest(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err = io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func run() error {
	if len(os.Args) != 5 {
		return os.ErrInvalid
	}
	goexe, module, target, destination := os.Args[1], os.Args[2], os.Args[3], os.Args[4]
	if target != runtime.GOOS || (target != "darwin" && target != "windows") || !filepath.IsAbs(goexe) || !filepath.IsAbs(module) || !filepath.IsAbs(destination) || filepath.Clean(destination) != destination {
		return os.ErrInvalid
	}
	if err := os.Mkdir(destination, 0700); err != nil {
		return err
	}
	root := filepath.Join(destination, "protected-install-root")
	suffix := ""
	if target == "windows" {
		suffix = ".exe"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	provisioner := filepath.Join(destination, "install-root-provisioner"+suffix)
	if err := trustinstall.CompileInstallRootProvisioner(ctx, goexe, module, target, provisioner, root, true); err != nil {
		return err
	}
	pin, err := digest(provisioner)
	if err != nil {
		return err
	}
	return trustinstall.CompileInstallRootValidation(ctx, goexe, module, target, filepath.Join(destination, "install-root-validation"+suffix), root, pin)
}

func main() {
	if err := run(); err != nil {
		fmt.Println("BUILD_FAILED")
		os.Exit(1)
	}
	fmt.Println("BUILD_READY native install-root validation bundle; no product payload or production path used")
}
