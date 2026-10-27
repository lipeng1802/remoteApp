// Fixed, monotonic installer transaction owner. The signed payload directory,
// provisioner, manager and target root are never supplied at runtime.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"remoteapp.local/selfhost-poc/installrelease"
	"remoteapp.local/selfhost-poc/trustinstall"
)

var embeddedRelease, embeddedInstallRoot, embeddedOperation, embeddedStaging, embeddedManifestDigest, embeddedManifestEpoch, embeddedProvisionerDigest, embeddedManagerDigest string

type toolReply struct {
	Status string `json:"status"`
	Epoch  uint64 `json:"epoch,omitempty"`
	Reason string `json:"reason,omitempty"`
}

type transactionReply struct {
	Status string `json:"status"`
	Epoch  uint64 `json:"epoch"`
	Phase  string `json:"phase"`
}

type configuration struct {
	release trustinstall.Release
	root    string
	epoch   uint64
	suffix  string
}

func configured() (configuration, error) {
	release, err := trustinstall.DecodeCompiled(embeddedRelease)
	raw, rootErr := base64.RawURLEncoding.DecodeString(embeddedInstallRoot)
	epoch, epochErr := strconv.ParseUint(embeddedManifestEpoch, 10, 64)
	if err != nil || rootErr != nil || base64.RawURLEncoding.EncodeToString(raw) != embeddedInstallRoot || epochErr != nil || strconv.FormatUint(epoch, 10) != embeddedManifestEpoch || epoch == 0 || epoch > 1<<53-1 || (embeddedOperation != "install" && embeddedOperation != "upgrade") || len(embeddedStaging) != len(".staging-")+32 || len(embeddedManifestDigest) != 64 || len(embeddedProvisionerDigest) != 64 || len(embeddedManagerDigest) != 64 {
		return configuration{}, installrelease.ErrRelease
	}
	for _, value := range []string{embeddedStaging[len(".staging-"):], embeddedManifestDigest, embeddedProvisionerDigest, embeddedManagerDigest} {
		decoded, decodeErr := hex.DecodeString(value)
		if decodeErr != nil || hex.EncodeToString(decoded) != value {
			return configuration{}, installrelease.ErrRelease
		}
	}
	root := string(raw)
	if !filepath.IsAbs(root) || filepath.Clean(root) != root || filepath.Dir(root) == root || epoch < release.Specification().MinimumEpoch {
		return configuration{}, installrelease.ErrRelease
	}
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	return configuration{release: release, root: root, epoch: epoch, suffix: suffix}, nil
}

func pinned(path, digest string) error {
	before, err := os.Lstat(path)
	if err != nil || !before.Mode().IsRegular() || before.Mode()&os.ModeSymlink != 0 || before.Size() < 1 || before.Size() > 100<<20 {
		return installrelease.ErrRelease
	}
	file, err := os.Open(path)
	if err != nil {
		return installrelease.ErrRelease
	}
	defer file.Close()
	after, err := file.Stat()
	if err != nil || !os.SameFile(before, after) {
		return installrelease.ErrRelease
	}
	hash := sha256.New()
	if _, err = io.Copy(hash, file); err != nil || hex.EncodeToString(hash.Sum(nil)) != digest {
		return installrelease.ErrRelease
	}
	return nil
}

func invoke(path string, args ...string) (toolReply, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, path, args...)
	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = io.Discard
	runErr := command.Run()
	if ctx.Err() != nil || output.Len() < 2 || output.Len() > 4096 {
		return toolReply{}, installrelease.ErrRelease
	}
	decoder := json.NewDecoder(bytes.NewReader(output.Bytes()))
	decoder.DisallowUnknownFields()
	var reply toolReply
	var extra struct{}
	if decoder.Decode(&reply) != nil || decoder.Decode(&extra) != io.EOF {
		return toolReply{}, installrelease.ErrRelease
	}
	canonical, _ := json.Marshal(reply)
	if !bytes.Equal(output.Bytes(), append(canonical, '\n')) {
		return toolReply{}, installrelease.ErrRelease
	}
	return reply, runErr
}

func provision(path, root string, test bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, path).Output()
	if err != nil || ctx.Err() != nil {
		return installrelease.ErrRelease
	}
	receipt, err := installrelease.DecodeLayoutReceipt(output)
	if err != nil || installrelease.ValidateLayoutReceipt(receipt, root, test) != nil {
		return installrelease.ErrRelease
	}
	return nil
}

func ensureTrust(path, operation string, epoch uint64) error {
	verb := "initialize"
	if operation == "upgrade" {
		verb = "upgrade"
	}
	reply, err := invoke(path, verb)
	if err == nil && reply.Status == "usable" && reply.Epoch == epoch {
		return nil
	}
	reply, err = invoke(path, "inspect")
	if err != nil || reply.Status != "usable" || reply.Epoch != epoch {
		return installrelease.ErrRelease
	}
	return nil
}

func run() (transactionReply, error) {
	if len(os.Args) != 1 {
		return transactionReply{}, installrelease.ErrRelease
	}
	configuration, err := configured()
	if err != nil {
		return transactionReply{}, err
	}
	executable, err := os.Executable()
	if err != nil {
		return transactionReply{}, err
	}
	bundle := filepath.Dir(executable)
	provisioner := filepath.Join(bundle, "install-root-provisioner"+configuration.suffix)
	manager := filepath.Join(bundle, "installed-release-manager"+configuration.suffix)
	if pinned(provisioner, embeddedProvisionerDigest) != nil || pinned(manager, embeddedManagerDigest) != nil {
		return transactionReply{}, installrelease.ErrRelease
	}
	public, err := hex.DecodeString(configuration.release.Specification().Root)
	if err != nil || len(public) != 32 {
		return transactionReply{}, installrelease.ErrRelease
	}
	platform := "darwin"
	if runtime.GOOS == "windows" {
		platform = "windows"
	}
	payload := filepath.Join(bundle, "release-payload")
	verifiedPayload, err := installrelease.VerifyDirectory(payload, public, configuration.release.Specification().Channel, platform, configuration.release.Specification().MinimumEpoch)
	if err != nil || verifiedPayload.ManifestDigest != embeddedManifestDigest || verifiedPayload.Manifest.Epoch != configuration.epoch {
		return transactionReply{}, installrelease.ErrRelease
	}
	// Initialize or advance native trust from the already verified, stable
	// installer payload. On Windows a just-created protected staging directory
	// can be visible to validation before a nested executable can reliably open
	// Credential Manager. Durable trust is therefore committed before layout
	// provisioning; a later failure is safely resumed against the same exact
	// signed payload and native epoch.
	if ensureTrust(filepath.Join(payload, "trust-install"+configuration.suffix), embeddedOperation, configuration.epoch) != nil {
		return transactionReply{}, installrelease.ErrRelease
	}
	if provision(provisioner, configuration.root, configuration.release.Specification().Test) != nil {
		return transactionReply{}, installrelease.ErrRelease
	}
	_, _, err = installrelease.ReconcileCandidate(payload, configuration.root, embeddedStaging, embeddedManifestDigest, configuration.epoch, public, configuration.release.Specification().Channel, platform, configuration.release.Specification().MinimumEpoch, configuration.release.Specification().Test)
	if err != nil {
		return transactionReply{}, err
	}
	reply, err := invoke(manager, embeddedOperation)
	if err != nil || reply.Status != "active" || reply.Epoch != configuration.epoch {
		return transactionReply{}, installrelease.ErrRelease
	}
	final, state, err := installrelease.ReconcileCandidate(filepath.Join(bundle, "release-payload"), configuration.root, embeddedStaging, embeddedManifestDigest, configuration.epoch, public, configuration.release.Specification().Channel, platform, configuration.release.Specification().MinimumEpoch, configuration.release.Specification().Test)
	if err != nil || state != installrelease.CandidatePromoted || final.ManifestDigest != embeddedManifestDigest {
		return transactionReply{}, installrelease.ErrRelease
	}
	return transactionReply{Status: "active", Epoch: configuration.epoch, Phase: "committed"}, nil
}

func main() {
	reply, err := run()
	if err != nil {
		_ = json.NewEncoder(os.Stdout).Encode(toolReply{Status: "failed", Reason: "transaction_rejected"})
		os.Exit(1)
	}
	_ = json.NewEncoder(os.Stdout).Encode(reply)
}
