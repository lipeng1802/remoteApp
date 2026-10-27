// Stable system entry. It accepts no runtime path and revalidates the signed
// current bootstrap set before delegating to its fixed launcher.
package main

import (
	"encoding/base64"
	"encoding/hex"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"remoteapp.local/selfhost-poc/installrelease"
	"remoteapp.local/selfhost-poc/trustinstall"
)

var embeddedRelease, embeddedInstallRoot string

func run() error {
	if len(os.Args) != 1 {
		return installrelease.ErrRelease
	}
	release, err := trustinstall.DecodeCompiled(embeddedRelease)
	if err != nil {
		return err
	}
	raw, err := base64.RawURLEncoding.DecodeString(embeddedInstallRoot)
	root := string(raw)
	if err != nil || base64.RawURLEncoding.EncodeToString(raw) != embeddedInstallRoot || !filepath.IsAbs(root) || filepath.Clean(root) != root || filepath.Dir(root) == root {
		return installrelease.ErrRelease
	}
	public, err := hex.DecodeString(release.Specification().Root)
	if err != nil || len(public) != 32 {
		return installrelease.ErrRelease
	}
	platform, suffix := "darwin", ""
	if runtime.GOOS == "windows" {
		platform, suffix = "windows", ".exe"
	}
	active, err := installrelease.OpenBootstrap(root, public, release.Specification().Channel, platform, release.Specification().MinimumEpoch, release.Specification().Test)
	if err != nil {
		return err
	}
	launcher := filepath.Join(active.Directory, "installed-release-launcher"+suffix)
	command := exec.Command(launcher)
	command.Stdin, command.Stdout, command.Stderr = os.Stdin, os.Stdout, io.Discard
	if err = command.Start(); err != nil {
		return err
	}
	return command.Wait()
}

func main() {
	if run() != nil {
		os.Exit(1)
	}
}
