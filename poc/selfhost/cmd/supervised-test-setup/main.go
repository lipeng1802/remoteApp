// Fresh TEST installation or explicit pre-trust file reconciliation only.
// No runtime root/identity selectors, trust reset or automatic state cleanup.
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"remoteapp.local/selfhost-poc/installrelease"
	"remoteapp.local/selfhost-poc/privatepipe"
	"remoteapp.local/selfhost-poc/trustinstall"
	"remoteapp.local/selfhost-poc/trustprofile"
	"runtime"
	"strings"
	"time"
)

var embeddedRelease, embeddedInstallRoot string
var failureStage = "compiled_configuration"
var failureReason string

func run() error {
	resume := len(os.Args) == 2 && os.Args[1] == "resume-files"
	if len(os.Args) != 1 && !resume {
		return os.ErrInvalid
	}
	r, e := trustinstall.DecodeCompiled(embeddedRelease)
	if e != nil || !r.Specification().Test {
		return os.ErrInvalid
	}
	raw, e := base64.RawURLEncoding.DecodeString(embeddedInstallRoot)
	if e != nil || base64.RawURLEncoding.EncodeToString(raw) != embeddedInstallRoot {
		return os.ErrInvalid
	}
	root := string(raw)
	if !filepath.IsAbs(root) || filepath.Clean(root) != root {
		return os.ErrInvalid
	}
	if !resume {
		if _, e = os.Lstat(root); !os.IsNotExist(e) {
			return os.ErrExist
		}
	}
	exe, e := os.Executable()
	if e != nil {
		return e
	}
	dir := filepath.Dir(exe)
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	pub, _ := hex.DecodeString(r.Specification().Root)
	s := r.Specification()
	if resume {
		probe, e := r.NativeRecordState(filepath.Join(dir, "trust-journal"+suffix))
		if e != nil {
			return e
		}
		state, e := probe(r.Scope(), true)
		if e != nil || state != trustprofile.JournalAbsent {
			return os.ErrExist
		}
		if _, e = installrelease.ValidateProtectedLayout(root, true); e != nil {
			return e
		}
	}
	payload := filepath.Join(dir, "payload")
	bootstrap := filepath.Join(dir, "bootstrap-payload")
	failureStage = "signed_payload"
	v, e := installrelease.VerifyDirectory(payload, pub, s.Channel, runtime.GOOS, s.MinimumEpoch)
	if e != nil {
		return e
	}
	b, e := installrelease.VerifyBootstrapDirectory(bootstrap, pub, s.Channel, runtime.GOOS, s.MinimumEpoch)
	if e != nil {
		return e
	}
	// All signed inputs are checked before creating the isolated root.
	failureStage = "protected_layout"
	if _, e = installrelease.PrepareProtectedLayout(root, true); e != nil {
		return e
	}
	stage := ".staging-11111111111111111111111111111111"
	failureStage = "signed_import"
	v, _, e = installrelease.ReconcileCandidate(payload, root, stage, v.ManifestDigest, v.Manifest.Epoch, pub, s.Channel, runtime.GOOS, s.MinimumEpoch, true)
	if e != nil {
		return e
	}
	failureStage = "payload_promote"
	if strings.HasPrefix(filepath.Base(v.Directory), ".staging-") {
		v, e = installrelease.PromoteStaged(v.Directory, filepath.Join(root, "releases"), pub, s.Channel, runtime.GOOS, s.MinimumEpoch)
	}
	if e != nil {
		return e
	}
	failureStage = "bootstrap_import"
	bootstrapDigest, bootstrapEpoch := b.ManifestDigest, b.Manifest.Epoch
	b, _, e = installrelease.ReconcileBootstrapCandidate(bootstrap, root, stage, bootstrapDigest, bootstrapEpoch, pub, s.Channel, runtime.GOOS, s.MinimumEpoch, true)
	if e != nil && resume {
		b, _, e = installrelease.ReconcileBootstrapCandidate(bootstrap, root, stage, bootstrapDigest, bootstrapEpoch, pub, s.Channel, runtime.GOOS, s.MinimumEpoch, true)
	}
	if e != nil {
		return e
	}
	if strings.HasPrefix(filepath.Base(b.Directory), ".staging-") {
		b, e = installrelease.PromoteBootstrapStaged(b.Directory, root, pub, s.Channel, runtime.GOOS, s.MinimumEpoch, true)
	}
	if e != nil {
		return e
	}
	if e = installrelease.ActivateBootstrap(root, "22222222222222222222222222222222", b, "install", pub, true); e != nil {
		opened, check := installrelease.OpenBootstrap(root, pub, s.Channel, runtime.GOOS, s.MinimumEpoch, true)
		if !resume || check != nil || opened.ManifestDigest != b.ManifestDigest {
			return e
		}
	}
	helper := filepath.Join(v.Directory, "trust-journal"+suffix)
	failureStage = "native_trust_factory"
	factory, e := r.NativeFactory(helper)
	if e != nil {
		return e
	}
	failureStage = "native_trust_owner"
	offline, e := trustinstall.NewOfflineOwner(r, factory, time.Now)
	if e != nil {
		return e
	}
	failureStage = "native_trust_initialize"
	if _, e = offline.Initialize(); e != nil {
		return e
	}
	records, e := r.NativeRecordFactory(helper)
	failureStage = "native_activation"
	if e != nil {
		return e
	}
	scope, e := installrelease.Scope(r.Scope())
	if e != nil {
		return e
	}
	journal, e := records(scope, true, true)
	if e != nil {
		return e
	}
	_, e = installrelease.ActivateFirst(journal, v)
	closeErr := journal.Close()
	if e != nil {
		return e
	}
	if closeErr != nil {
		return closeErr
	}
	// Executable belongs to the just verified immutable signed release.
	cmd := exec.Command(filepath.Join(v.Directory, "autonomous-enrollment-install"+suffix), "initialize")
	failureStage = "native_enrollment"
	output, e := cmd.Output()
	if e != nil {
		failureStage = enrollmentFailureStage(output)
		failureReason = enrollmentFailureReason(output)
		return e
	}
	var result struct {
		Status string `json:"status"`
	}
	if json.Unmarshal(output, &result) != nil || result.Status != "usable" {
		return os.ErrInvalid
	}
	return nil
}
func main() {
	if err := run(); err != nil {
		value := map[string]string{"status": "setup_failed_state_preserved", "stage": failureStage}
		reason := trustprofile.NativeFailureReason(err)
		if reason == "" {
			reason = failureReason
		}
		if reason != "" {
			value["reason"] = reason
		}
		json.NewEncoder(os.Stdout).Encode(value)
		os.Exit(1)
	}
	json.NewEncoder(os.Stdout).Encode(map[string]string{"status": "test_installed"})
}

func enrollmentFailureStage(output []byte) string {
	if len(output) > 4096 {
		return "native_enrollment"
	}
	var v struct {
		Status string `json:"status"`
		Stage  string `json:"stage"`
		Reason string `json:"reason"`
	}
	reader := privatepipe.NewReader(bytes.NewReader(output))
	if reader.Read(&v) != nil || reader.Read(new(any)) != io.EOF || v.Status != "failed" {
		return "native_enrollment"
	}
	switch v.Stage {
	case "identity_intent", "identity_reservation", "identity_scope", "identity_create", "enrollment_scope", "enrollment_initialize", "binding_commit", "identity_join":
		return "native_" + v.Stage
	}
	return "native_enrollment"
}

func enrollmentFailureReason(output []byte) string {
	if enrollmentFailureStage(output) == "native_enrollment" {
		return ""
	}
	var v struct {
		Status string `json:"status"`
		Stage  string `json:"stage"`
		Reason string `json:"reason"`
	}
	r := privatepipe.NewReader(bytes.NewReader(output))
	if r.Read(&v) != nil || r.Read(new(any)) != io.EOF {
		return ""
	}
	switch v.Reason {
	case "native_helper_timeout", "native_authentication_required", "interactive_logon_required":
		return v.Reason
	}
	return ""
}
