// Fixed installed-release launcher. It accepts no arguments or runtime trust
// material and revalidates the active signed directory before every launch.
package main

import (
	"encoding/hex"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"remoteapp.local/selfhost-poc/installrelease"
	"remoteapp.local/selfhost-poc/trustinstall"
)

var embeddedRelease string

func main() {
	if run() != nil {
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) != 1 {
		return installrelease.ErrRelease
	}
	release, err := trustinstall.DecodeCompiled(embeddedRelease)
	if err != nil {
		return err
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	root := filepath.Dir(executable)
	releases, err := installrelease.ValidateProtectedLayout(root, release.Specification().Test)
	if err != nil {
		return err
	}
	suffix, platform := "", "darwin"
	if runtime.GOOS == "windows" {
		suffix, platform = ".exe", "windows"
	}
	helper := filepath.Join(root, "trust-journal"+suffix)
	factory, err := release.NativeFactory(helper)
	if err != nil {
		return err
	}
	owner, err := trustinstall.NewOfflineOwner(release, factory, time.Now)
	if err != nil {
		return err
	}
	if _, err = owner.Inspect(); err != nil {
		return err
	}
	records, err := release.NativeRecordFactory(helper)
	if err != nil {
		return err
	}
	scope, err := installrelease.Scope(release.Scope())
	if err != nil {
		return err
	}
	journal, err := records(scope, false, release.Specification().Test)
	if err != nil {
		return err
	}
	public, err := hex.DecodeString(release.Specification().Root)
	if err != nil || len(public) != 32 {
		_ = journal.Close()
		return installrelease.ErrRelease
	}
	active, openErr := installrelease.OpenActive(journal, releases, public, release.Specification().Channel, platform, release.Specification().MinimumEpoch)
	closeErr := journal.Close()
	if openErr != nil || closeErr != nil || installrelease.ValidateProtectedRelease(active.Directory, root, release.Specification().Test) != nil {
		return installrelease.ErrRelease
	}
	product := filepath.Join(active.Directory, "installed-autonomous-product"+suffix)
	cmd := exec.Command(product)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, io.Discard
	if err = cmd.Start(); err != nil {
		return err
	}
	return cmd.Wait()
}
