// Offline immutable-release activation manager. Runtime input selects only an
// operation; root, channel, floor, helper and candidate location are compiled.
package main

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"remoteapp.local/selfhost-poc/installrelease"
	"remoteapp.local/selfhost-poc/trustinstall"
	"remoteapp.local/selfhost-poc/trustprofile"
)

var embeddedRelease, embeddedStaging, embeddedManifestDigest, embeddedManifestEpoch string

type reply struct {
	Status string `json:"status"`
	Epoch  uint64 `json:"epoch,omitempty"`
	Reason string `json:"reason,omitempty"`
}

func main() {
	value, err := run()
	if err != nil {
		reason := "release_rejected"
		if errors.Is(err, trustprofile.ErrDesktopLogon) {
			reason = "interactive_logon_required"
		}
		_ = json.NewEncoder(os.Stdout).Encode(reply{Status: "failed", Reason: reason})
		os.Exit(1)
	}
	_ = json.NewEncoder(os.Stdout).Encode(value)
}

func installed(release trustinstall.Release, helper string) error {
	factory, err := release.NativeFactory(helper)
	if err != nil {
		return err
	}
	owner, err := trustinstall.NewOfflineOwner(release, factory, time.Now)
	if err != nil {
		return err
	}
	_, err = owner.Inspect()
	return err
}

func anchor(release trustinstall.Release) ([]byte, error) {
	raw, err := hex.DecodeString(release.Specification().Root)
	if err != nil || len(raw) != 32 {
		return nil, installrelease.ErrRelease
	}
	return raw, nil
}

func expectedCandidate(root string, release trustinstall.Release, public []byte, platform string, epoch uint64) (installrelease.Verified, error) {
	releases := filepath.Join(root, "releases")
	staging := filepath.Join(releases, embeddedStaging)
	_, statErr := os.Lstat(staging)
	if statErr == nil {
		verified, err := installrelease.PromoteStaged(staging, releases, public, release.Specification().Channel, platform, release.Specification().MinimumEpoch)
		if err != nil {
			return installrelease.Verified{}, err
		}
		if verified.ManifestDigest != embeddedManifestDigest || verified.Manifest.Epoch != epoch {
			return installrelease.Verified{}, installrelease.ErrRelease
		}
		return verified, nil
	}
	if !os.IsNotExist(statErr) {
		return installrelease.Verified{}, installrelease.ErrRelease
	}
	name, nameErr := installrelease.ReleaseDirectoryName(installrelease.Verified{Manifest: installrelease.Manifest{Epoch: epoch}, ManifestDigest: embeddedManifestDigest})
	if nameErr != nil {
		return installrelease.Verified{}, installrelease.ErrRelease
	}
	verified, reopenErr := installrelease.VerifyDirectory(filepath.Join(releases, name), public, release.Specification().Channel, platform, release.Specification().MinimumEpoch)
	if reopenErr != nil || verified.ManifestDigest != embeddedManifestDigest || verified.Manifest.Epoch != epoch {
		return installrelease.Verified{}, installrelease.ErrRelease
	}
	return verified, nil
}

func run() (reply, error) {
	if len(os.Args) != 2 || (os.Args[1] != "install" && os.Args[1] != "upgrade" && os.Args[1] != "status" && os.Args[1] != "cleanup-test") {
		return reply{}, installrelease.ErrRelease
	}
	release, err := trustinstall.DecodeCompiled(embeddedRelease)
	if err != nil || len(embeddedStaging) != len(".staging-")+32 || embeddedStaging[:len(".staging-")] != ".staging-" {
		return reply{}, installrelease.ErrRelease
	}
	stagingRaw, stagingErr := hex.DecodeString(embeddedStaging[len(".staging-"):])
	digestRaw, digestErr := hex.DecodeString(embeddedManifestDigest)
	if stagingErr != nil || len(stagingRaw) != 16 || hex.EncodeToString(stagingRaw) != embeddedStaging[len(".staging-"):] || digestErr != nil || len(digestRaw) != 32 || hex.EncodeToString(digestRaw) != embeddedManifestDigest {
		return reply{}, installrelease.ErrRelease
	}
	epoch, err := parseEpoch(embeddedManifestEpoch)
	if err != nil || epoch < release.Specification().MinimumEpoch {
		return reply{}, installrelease.ErrRelease
	}
	executable, err := os.Executable()
	if err != nil {
		return reply{}, err
	}
	root := filepath.Dir(executable)
	suffix, platform := "", "darwin"
	if runtime.GOOS == "windows" {
		suffix, platform = ".exe", "windows"
	}
	helper := filepath.Join(root, "trust-journal"+suffix)
	if err = installed(release, helper); err != nil {
		return reply{}, err
	}
	records, err := release.NativeRecordFactory(helper)
	if err != nil {
		return reply{}, err
	}
	scope, err := installrelease.Scope(release.Scope())
	if err != nil {
		return reply{}, err
	}
	if os.Args[1] == "cleanup-test" {
		if !release.Specification().Test {
			return reply{}, installrelease.ErrRelease
		}
		if err = trustprofile.DeleteTestDesktopJournal(helper, scope); err != nil {
			return reply{}, err
		}
		return reply{Status: "cleaned"}, nil
	}
	public, err := anchor(release)
	if err != nil {
		return reply{}, err
	}
	if os.Args[1] == "status" {
		journal, err := records(scope, false, release.Specification().Test)
		if err != nil {
			return reply{}, err
		}
		active, openErr := installrelease.OpenActive(journal, filepath.Join(root, "releases"), public, release.Specification().Channel, platform, release.Specification().MinimumEpoch)
		closeErr := journal.Close()
		if openErr != nil || closeErr != nil {
			return reply{}, installrelease.ErrRelease
		}
		return reply{Status: "active", Epoch: active.Manifest.Epoch}, nil
	}
	candidate, err := expectedCandidate(root, release, public, platform, epoch)
	if err != nil {
		return reply{}, err
	}
	journal, err := records(scope, os.Args[1] == "install", release.Specification().Test)
	if err != nil {
		return reply{}, err
	}
	var active installrelease.Activation
	if os.Args[1] == "install" {
		active, err = installrelease.ActivateFirst(journal, candidate)
	} else {
		active, err = installrelease.ActivateUpgrade(journal, candidate)
	}
	closeErr := journal.Close()
	if err != nil || closeErr != nil {
		return reply{}, installrelease.ErrRelease
	}
	return reply{Status: "active", Epoch: active.Epoch}, nil
}

func parseEpoch(value string) (uint64, error) {
	if value == "" || len(value) > 16 || (len(value) > 1 && value[0] == '0') {
		return 0, installrelease.ErrRelease
	}
	var epoch uint64
	for _, c := range value {
		if c < '0' || c > '9' || epoch > (1<<53-1-uint64(c-'0'))/10 {
			return 0, installrelease.ErrRelease
		}
		epoch = epoch*10 + uint64(c-'0')
	}
	if epoch == 0 {
		return 0, installrelease.ErrRelease
	}
	return epoch, nil
}
