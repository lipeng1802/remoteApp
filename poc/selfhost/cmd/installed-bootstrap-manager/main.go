package main

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"remoteapp.local/selfhost-poc/installrelease"
	"remoteapp.local/selfhost-poc/trustinstall"
)

var embeddedRelease, embeddedInstallRoot, embeddedOperation, embeddedStaging, embeddedManifestDigest, embeddedManifestEpoch, embeddedPointerNonce string

type reply struct {
	Status string `json:"status"`
	Epoch  uint64 `json:"epoch,omitempty"`
	Reason string `json:"reason,omitempty"`
}

type configuration struct {
	release  trustinstall.Release
	root     string
	epoch    uint64
	public   []byte
	platform string
}

func configured() (configuration, error) {
	release, releaseErr := trustinstall.DecodeCompiled(embeddedRelease)
	raw, rootErr := base64.RawURLEncoding.DecodeString(embeddedInstallRoot)
	epoch, epochErr := strconv.ParseUint(embeddedManifestEpoch, 10, 64)
	public, publicErr := hex.DecodeString(release.Specification().Root)
	root := string(raw)
	if releaseErr != nil || rootErr != nil || base64.RawURLEncoding.EncodeToString(raw) != embeddedInstallRoot || !filepath.IsAbs(root) || filepath.Clean(root) != root || filepath.Dir(root) == root || epochErr != nil || strconv.FormatUint(epoch, 10) != embeddedManifestEpoch || epoch == 0 || epoch > 1<<53-1 || epoch < release.Specification().MinimumEpoch || (embeddedOperation != "install" && embeddedOperation != "upgrade") || len(embeddedStaging) != len(".staging-")+32 || !strings.HasPrefix(embeddedStaging, ".staging-") || !canonicalHex(strings.TrimPrefix(embeddedStaging, ".staging-"), 16) || !canonicalHex(embeddedManifestDigest, 32) || !canonicalHex(embeddedPointerNonce, 16) || publicErr != nil || len(public) != 32 {
		return configuration{}, installrelease.ErrRelease
	}
	platform := "darwin"
	if runtime.GOOS == "windows" {
		platform = "windows"
	}
	return configuration{release: release, root: root, epoch: epoch, public: public, platform: platform}, nil
}

func canonicalHex(value string, size int) bool {
	raw, err := hex.DecodeString(value)
	return err == nil && len(raw) == size && hex.EncodeToString(raw) == value
}

func candidate(configuration configuration) (installrelease.VerifiedBootstrap, string, error) {
	executable, err := os.Executable()
	if err != nil {
		return installrelease.VerifiedBootstrap{}, "", err
	}
	return installrelease.ReconcileBootstrapCandidate(filepath.Join(filepath.Dir(executable), "bootstrap-payload"), configuration.root, embeddedStaging, embeddedManifestDigest, configuration.epoch, configuration.public, configuration.release.Specification().Channel, configuration.platform, configuration.release.Specification().MinimumEpoch, configuration.release.Specification().Test)
}

func run() (reply, error) {
	configuration, err := configured()
	if err != nil {
		return reply{}, err
	}
	operation := "activate"
	if len(os.Args) == 2 {
		operation = os.Args[1]
	} else if len(os.Args) != 1 {
		return reply{}, installrelease.ErrRelease
	}
	if operation == "status" {
		active, err := installrelease.OpenBootstrap(configuration.root, configuration.public, configuration.release.Specification().Channel, configuration.platform, configuration.release.Specification().MinimumEpoch, configuration.release.Specification().Test)
		if err != nil {
			return reply{}, err
		}
		return reply{Status: "active", Epoch: active.Manifest.Epoch}, nil
	}
	verified, state, err := candidate(configuration)
	if err != nil {
		return reply{}, err
	}
	if operation == "cleanup-pointer-test" {
		if !configuration.release.Specification().Test || state != installrelease.CandidatePromoted || installrelease.CleanupBootstrapPointerTest(configuration.root, verified, configuration.public, configuration.release.Specification().Channel, configuration.platform) != nil {
			return reply{}, installrelease.ErrRelease
		}
		return reply{Status: "cleaned"}, nil
	}
	if operation == "cleanup-set-test" {
		if !configuration.release.Specification().Test || state != installrelease.CandidatePromoted || installrelease.CleanupVerifiedBootstrapSet(configuration.root, verified, configuration.public, configuration.release.Specification().Channel, configuration.platform) != nil {
			return reply{}, installrelease.ErrRelease
		}
		return reply{Status: "cleaned"}, nil
	}
	if operation != "activate" {
		return reply{}, installrelease.ErrRelease
	}
	if state == installrelease.CandidateStaged {
		verified, err = installrelease.PromoteBootstrapStaged(verified.Directory, configuration.root, configuration.public, configuration.release.Specification().Channel, configuration.platform, configuration.release.Specification().MinimumEpoch, configuration.release.Specification().Test)
		if err != nil {
			return reply{}, err
		}
	}
	if installrelease.ActivateBootstrap(configuration.root, embeddedPointerNonce, verified, embeddedOperation, configuration.public, configuration.release.Specification().Test) != nil {
		return reply{}, installrelease.ErrRelease
	}
	active, err := installrelease.OpenBootstrap(configuration.root, configuration.public, configuration.release.Specification().Channel, configuration.platform, configuration.release.Specification().MinimumEpoch, configuration.release.Specification().Test)
	if err != nil || active.ManifestDigest != embeddedManifestDigest || active.Manifest.Epoch != configuration.epoch {
		return reply{}, installrelease.ErrRelease
	}
	return reply{Status: "active", Epoch: configuration.epoch}, nil
}

func main() {
	value, err := run()
	if err != nil {
		_ = json.NewEncoder(os.Stdout).Encode(reply{Status: "failed", Reason: "bootstrap_rejected"})
		os.Exit(1)
	}
	_ = json.NewEncoder(os.Stdout).Encode(value)
}
