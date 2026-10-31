package main

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"remoteapp.local/selfhost-poc/installrelease"
)

func TestOfflinePreflightRejectsBeforeBundleOrCompiler(t *testing.T) {
	previous := os.Args
	t.Cleanup(func() { os.Args = previous })
	parent := t.TempDir()
	root := filepath.Join(parent, "existing")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(parent, "bundle")
	missing := filepath.Join(parent, "missing-input")
	os.Args = []string{"builder", missing, parent, runtime.GOOS, dest, missing, missing, missing, "--offline-install-root", root}
	err := run()
	if !errors.Is(err, installrelease.ErrRelease) || installrelease.InstallPreflightReason(err) != "test_install_root_not_fresh" {
		t.Fatal("did not reject at preflight", err)
	}
	if _, err = os.Lstat(dest); !os.IsNotExist(err) {
		t.Fatal("rejected preflight created bundle")
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatal("existing root changed")
	}
}

func TestCheckOnlyNeedsNoCompilerOrInputs(t *testing.T) {
	previous := os.Args
	t.Cleanup(func() { os.Args = previous })
	parent := t.TempDir()
	root := filepath.Join(parent, "install")
	os.Args = []string{"builder", "--check-offline-install-root", root}
	if err := run(); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(parent)
	if err != nil || len(entries) != 0 {
		t.Fatal("check-only mutated directory")
	}
}
