package main

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

func TestOfflineBundleCannotCreateInstallRoot(t *testing.T) {
	previous := os.Args
	t.Cleanup(func() { os.Args = previous })
	parent := t.TempDir()
	root := filepath.Join(parent, "install")
	input := filepath.Join(parent, "input")
	if err := os.WriteFile(input, []byte("unused test input"), 0600); err != nil {
		t.Fatal(err)
	}
	os.Args = []string{"builder", filepath.Join(parent, "missing-compiler"), parent, runtime.GOOS, root, input, input, input, "--offline-install-root", root}
	if err := run(); !errors.Is(err, errBuildInput) {
		t.Fatal("overlapping output was not rejected before build", err)
	}
	if _, err := os.Lstat(root); !os.IsNotExist(err) {
		t.Fatal("builder created the reserved install root", err)
	}
}

func TestOfflineOutputPathSeparation(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "install")
	for _, dest := range []string{root, parent, filepath.Join(root, "bundle")} {
		if err := checkOfflineOutputPath(dest, root); err != errBuildInput {
			t.Fatal("overlap accepted", err)
		}
	}
	if runtime.GOOS == "windows" {
		for _, alias := range []string{root + ".", root + " ", root + ". ", filepath.Join(parent+".", "bundle")} {
			if err := checkOfflineOutputPath(alias, root); err != errBuildInput {
				t.Fatal("normalized output alias accepted", err)
			}
			if err := checkOfflineOutputPath(filepath.Join(parent, "bundle"), alias); err != errBuildInput {
				t.Fatal("normalized root alias accepted", err)
			}
		}
		if err := checkOfflineOutputPath(strings.ToUpper(root), root); err != errBuildInput {
			t.Fatal("case alias accepted", err)
		}
	}
	for _, dest := range []string{filepath.Join(parent, "bundle"), root + "-bundle"} {
		if err := checkOfflineOutputPath(dest, root); err != nil {
			t.Fatal("separate sibling rejected", err)
		}
	}
}

func TestWindowsOutputTrailingDotCannotCreateInstallRoot(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows filename normalization")
	}
	previous := os.Args
	t.Cleanup(func() { os.Args = previous })
	parent := t.TempDir()
	root := filepath.Join(parent, "install")
	input := filepath.Join(parent, "input")
	if err := os.WriteFile(input, []byte("unused test input"), 0600); err != nil {
		t.Fatal(err)
	}
	os.Args = []string{"builder", filepath.Join(parent, "missing-compiler"), parent, runtime.GOOS, root + ".", input, input, input, "--offline-install-root", root}
	err := run()
	_, rootErr := os.Lstat(root)
	if !errors.Is(err, errBuildInput) || !os.IsNotExist(rootErr) {
		t.Fatalf("alias reached build: err=%v rootExists=%v", err, rootErr == nil)
	}
}

func TestWindowsExtendedOutputCannotCreateInstallRoot(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows extended path")
	}
	previous := os.Args
	t.Cleanup(func() { os.Args = previous })
	parent := t.TempDir()
	root := filepath.Join(parent, "install")
	input := filepath.Join(parent, "input")
	if err := os.WriteFile(input, []byte("unused test input"), 0600); err != nil {
		t.Fatal(err)
	}
	alias := `\\?\` + root
	os.Args = []string{"builder", filepath.Join(parent, "missing-compiler"), parent, runtime.GOOS, alias, input, input, input, "--offline-install-root", root}
	err := run()
	_, rootErr := os.Lstat(root)
	if !errors.Is(err, errBuildInput) || !os.IsNotExist(rootErr) {
		t.Fatalf("extended alias reached build: err=%v rootExists=%v", err, rootErr == nil)
	}
}
