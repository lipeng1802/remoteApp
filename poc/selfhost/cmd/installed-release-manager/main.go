// Offline immutable-release activation manager. Runtime input selects only an
// operation; root, channel, floor, helper and candidate location are compiled.
package main

import (
	"encoding/base64"
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

var embeddedRelease, embeddedInstallRoot, embeddedStaging, embeddedManifestDigest, embeddedManifestEpoch string

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

func expectedCandidate(releases string, release trustinstall.Release, public []byte, platform string, epoch uint64) (installrelease.Verified, error) {
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

// Activation commit and helper shutdown are separate durability boundaries.
// If either acknowledgement is uncertain, do not write again in this process:
// release the old owner, reopen read-only, and accept only this exact signed
// candidate. A previous epoch or any invalid state remains a hard failure.
func reopenExactActivation(records trustinstall.RecordFactory, scope string, test bool, releases, root string, public []byte, release trustinstall.Release, platform string, candidate installrelease.Verified) (uint64, error) {
	journal, err := records(scope, false, test)
	if err != nil {
		return 0, err
	}
	active, openErr := installrelease.OpenActive(journal, releases, public, release.Specification().Channel, platform, release.Specification().MinimumEpoch)
	closeErr := journal.Close()
	if openErr != nil || closeErr != nil || active.ManifestDigest != candidate.ManifestDigest || active.Manifest.Epoch != candidate.Manifest.Epoch || active.Directory != candidate.Directory || installrelease.ValidateProtectedRelease(active.Directory, root, test) != nil {
		return 0, installrelease.ErrRelease
	}
	return active.Manifest.Epoch, nil
}

func run() (reply, error) {
	if len(os.Args) != 2 || (os.Args[1] != "install" && os.Args[1] != "upgrade" && os.Args[1] != "status" && os.Args[1] != "cleanup-test" && os.Args[1] != "cleanup-release-test") {
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
	executableRoot := filepath.Dir(executable)
	root, helperRoot := executableRoot, executableRoot
	if embeddedInstallRoot != "" {
		raw, decodeErr := base64.RawURLEncoding.DecodeString(embeddedInstallRoot)
		if decodeErr != nil || base64.RawURLEncoding.EncodeToString(raw) != embeddedInstallRoot {
			return reply{}, installrelease.ErrRelease
		}
		root = string(raw)
		if !filepath.IsAbs(root) || filepath.Clean(root) != root || filepath.Dir(root) == root {
			return reply{}, installrelease.ErrRelease
		}
	}
	releases, err := installrelease.ValidateProtectedLayout(root, release.Specification().Test)
	if err != nil {
		return reply{}, err
	}
	suffix, platform := "", "darwin"
	if runtime.GOOS == "windows" {
		suffix, platform = ".exe", "windows"
	}
	helper := filepath.Join(helperRoot, "trust-journal"+suffix)
	records, err := release.NativeRecordFactory(helper)
	if err != nil {
		return reply{}, err
	}
	scope, err := installrelease.Scope(release.Scope())
	if err != nil {
		return reply{}, err
	}
	if os.Args[1] == "cleanup-release-test" {
		if !release.Specification().Test {
			return reply{}, installrelease.ErrRelease
		}
		state, stateErr := release.NativeRecordState(helper)
		if stateErr != nil {
			return reply{}, stateErr
		}
		current, stateErr := state(scope, true)
		if stateErr != nil || current != trustprofile.JournalAbsent {
			return reply{}, installrelease.ErrRelease
		}
		public, publicErr := anchor(release)
		candidate, candidateErr := expectedCandidate(releases, release, public, platform, epoch)
		if publicErr != nil || candidateErr != nil || installrelease.CleanupVerifiedTestRelease(candidate.Directory, root, embeddedManifestDigest, epoch, public, release.Specification().Channel, platform, release.Specification().MinimumEpoch, true) != nil {
			return reply{}, installrelease.ErrRelease
		}
		return reply{Status: "cleaned"}, nil
	}
	if err = installed(release, helper); err != nil {
		if os.Args[1] != "cleanup-test" || !release.Specification().Test {
			return reply{}, err
		}
		factory, e := release.NativeFactory(helper)
		if e != nil {
			return reply{}, e
		}
		offline, e := trustinstall.NewOfflineOwner(release, factory, time.Now)
		if e != nil {
			return reply{}, e
		}
		if _, e = offline.MaintenanceState(); e != nil {
			return reply{}, e
		}
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
		active, openErr := installrelease.OpenActive(journal, releases, public, release.Specification().Channel, platform, release.Specification().MinimumEpoch)
		closeErr := journal.Close()
		if openErr != nil || closeErr != nil || installrelease.ValidateProtectedRelease(active.Directory, root, release.Specification().Test) != nil {
			return reply{}, installrelease.ErrRelease
		}
		return reply{Status: "active", Epoch: active.Manifest.Epoch}, nil
	}
	candidate, err := expectedCandidate(releases, release, public, platform, epoch)
	if err != nil {
		return reply{}, err
	}
	if installrelease.ValidateProtectedRelease(candidate.Directory, root, release.Specification().Test) != nil {
		return reply{}, installrelease.ErrRelease
	}
	journal, openErr := records(scope, false, release.Specification().Test)
	if openErr == nil {
		// Upgrade must authenticate the previous signed epoch even when it is
		// below this manager's new floor. Installed trust was already advanced;
		// activation still requires a strictly newer exact candidate below.
		active, activeErr := installrelease.OpenActive(journal, releases, public, release.Specification().Channel, platform, 1)
		if activeErr != nil {
			journal.Close()
			return reply{}, installrelease.ErrRelease
		}
		if active.ManifestDigest == candidate.ManifestDigest && active.Manifest.Epoch == candidate.Manifest.Epoch && active.Directory == candidate.Directory {
			if journal.Close() != nil {
				return reply{}, installrelease.ErrRelease
			}
			return reply{Status: "active", Epoch: active.Manifest.Epoch}, nil
		}
		if os.Args[1] != "upgrade" {
			journal.Close()
			return reply{}, installrelease.ErrRelease
		}
		activated, activateErr := installrelease.ActivateUpgrade(journal, candidate)
		closeErr := journal.Close()
		if activateErr != nil || closeErr != nil {
			epoch, reopenErr := reopenExactActivation(records, scope, release.Specification().Test, releases, root, public, release, platform, candidate)
			if reopenErr != nil {
				return reply{}, installrelease.ErrRelease
			}
			return reply{Status: "active", Epoch: epoch}, nil
		}
		return reply{Status: "active", Epoch: activated.Epoch}, nil
	}
	if os.Args[1] != "install" {
		return reply{}, openErr
	}
	journal, err = records(scope, true, release.Specification().Test)
	if err != nil {
		return reply{}, err
	}
	active, activateErr := installrelease.ActivateFirst(journal, candidate)
	closeErr := journal.Close()
	if activateErr != nil || closeErr != nil {
		epoch, reopenErr := reopenExactActivation(records, scope, release.Specification().Test, releases, root, public, release, platform, candidate)
		if reopenErr != nil {
			return reply{}, installrelease.ErrRelease
		}
		return reply{Status: "active", Epoch: epoch}, nil
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
