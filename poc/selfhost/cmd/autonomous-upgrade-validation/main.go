// Runs a compiled TEST-only native autonomous installation upgrade lifecycle.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"remoteapp.local/selfhost-poc/installrelease"
	"remoteapp.local/selfhost-poc/privatepipe"
)

type reply struct {
	Status          string `json:"status"`
	PublicKey       string `json:"public_key"`
	EnrollmentState string `json:"enrollment_state"`
	Epoch           uint64 `json:"epoch"`
	Stage           string `json:"stage"`
	Diagnostic      int    `json:"diagnostic_version"`
	TrustEpoch      uint64 `json:"trust_epoch"`
	Reason          string `json:"reason"`
	Phase           string `json:"phase"`
}

type releaseLayout struct {
	V1Stage                string `json:"v1_stage"`
	V1Directory            string `json:"v1_directory"`
	V2Stage                string `json:"v2_stage"`
	V2Directory            string `json:"v2_directory"`
	TransactionV1Stage     string `json:"transaction_v1_stage"`
	TransactionV1Directory string `json:"transaction_v1_directory"`
	TransactionV2Stage     string `json:"transaction_v2_stage"`
	TransactionV2Directory string `json:"transaction_v2_directory"`
	TransactionRoot        string `json:"transaction_root"`
}

func command(path string, input []byte, args ...string) (reply, error) {
	cmd := exec.Command(path, args...)
	if input != nil {
		cmd.Stdin = bytes.NewReader(input)
	}
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	if err := cmd.Start(); err != nil {
		return reply{}, err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		var value reply
		if json.Unmarshal(bytes.TrimSpace(output.Bytes()), &value) != nil {
			return reply{}, fmt.Errorf("invalid reply")
		}
		return value, err
	case <-time.After(20 * time.Second):
		_ = cmd.Process.Kill()
		<-done
		return reply{}, fmt.Errorf("timeout")
	}
}

func read(path string, limit int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || !st.Mode().IsRegular() || st.Size() < 1 || st.Size() > limit {
		return nil, fmt.Errorf("invalid fixture")
	}
	raw, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil || int64(len(raw)) > limit {
		return nil, fmt.Errorf("invalid fixture")
	}
	return raw, nil
}

func productReject(path string) error {
	cmd := exec.Command(path)
	cmd.Stdin = bytes.NewBufferString("{\"Op\":\"invalid\"}\n")
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err == nil {
			return fmt.Errorf("invalid product request accepted")
		}
	case <-time.After(20 * time.Second):
		_ = cmd.Process.Kill()
		<-done
		return fmt.Errorf("product timeout")
	}
	reader := privatepipe.NewBoundedReader(bytes.NewReader(output.Bytes()), 4096)
	var failed reply
	var extra any
	if reader.Read(&failed) != nil || failed.Status != "failed" || reader.Read(&extra) != io.EOF {
		return fmt.Errorf("invalid product rejection")
	}
	return nil
}

func transactionDiagnostic(stage, promoted, transaction, trust string, transactionValue reply, transactionErr error) string {
	candidate := "absent"
	trustStatus := "not_checked"
	trustReason := "none"
	trustEpoch := uint64(0)
	if info, err := os.Lstat(stage); err == nil && info.IsDir() && info.Mode()&os.ModeSymlink == 0 {
		candidate = "staged"
		if value, inspectErr := command(filepath.Join(stage, trust), nil, "inspect"); inspectErr == nil {
			trustStatus, trustEpoch = value.Status, value.Epoch
		} else {
			trustStatus, trustReason = value.Status, value.Reason
			if trustStatus == "" {
				trustStatus = "failed"
			}
			if trustReason == "" {
				trustReason = "invalid_reply"
			}
		}
	} else if info, err := os.Lstat(promoted); err == nil && info.IsDir() && info.Mode()&os.ModeSymlink == 0 {
		candidate = "promoted"
		if value, inspectErr := command(filepath.Join(promoted, trust), nil, "inspect"); inspectErr == nil {
			trustStatus, trustEpoch = value.Status, value.Epoch
		} else {
			trustStatus, trustReason = value.Status, value.Reason
			if trustStatus == "" {
				trustStatus = "failed"
			}
			if trustReason == "" {
				trustReason = "invalid_reply"
			}
		}
	}
	managerStatus, managerReason, managerEpoch := "failed", "none", uint64(0)
	if value, statusErr := command(transaction, nil, "status"); statusErr == nil {
		managerStatus, managerEpoch = value.Status, value.Epoch
	} else if value.Status != "" {
		managerStatus, managerReason, managerEpoch = value.Status, value.Reason, value.Epoch
		if managerReason == "" {
			managerReason = "invalid_reply"
		}
	}
	childStatus := transactionValue.Status
	if childStatus == "" {
		childStatus = "invalid"
	}
	childExit := "zero"
	if transactionErr != nil {
		childExit = "nonzero"
	}
	return fmt.Sprintf("candidate=%s trust=%s/%d trust_reason=%s manager=%s/%d manager_reason=%s child=%s/%s", candidate, trustStatus, trustEpoch, trustReason, managerStatus, managerEpoch, managerReason, childStatus, childExit)
}

func run() (result error) {
	if len(os.Args) != 1 {
		return fmt.Errorf("no arguments permitted")
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	dir := filepath.Dir(exe)
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	manifest, err := read(filepath.Join(dir, "SHA256.json"), 8192)
	if err != nil {
		return err
	}
	var hashes map[string]string
	if len(manifest) > 8192 || json.Unmarshal(manifest, &hashes) != nil {
		return fmt.Errorf("invalid manifest")
	}
	layoutRaw, err := read(filepath.Join(dir, "release-layout.json"), 4096)
	var layout releaseLayout
	if err != nil || json.Unmarshal(layoutRaw, &layout) != nil {
		return fmt.Errorf("invalid release layout")
	}
	validStage := func(value string) bool {
		if len(value) != len(".staging-")+32 || value[:len(".staging-")] != ".staging-" {
			return false
		}
		raw, err := hex.DecodeString(value[len(".staging-"):])
		return err == nil && len(raw) == 16 && hex.EncodeToString(raw) == value[len(".staging-"):]
	}
	validDirectory := func(value string) bool {
		if len(value) != 16+1+16 || value[16] != '-' {
			return false
		}
		raw, err := hex.DecodeString(value[:16] + value[17:])
		return err == nil && len(raw) == 16 && hex.EncodeToString(raw) == value[:16]+value[17:]
	}
	if !validStage(layout.V1Stage) || !validStage(layout.V2Stage) || layout.V1Stage == layout.V2Stage ||
		!validStage(layout.TransactionV1Stage) || !validStage(layout.TransactionV2Stage) || layout.TransactionV1Stage == layout.TransactionV2Stage ||
		!validDirectory(layout.V1Directory) || !validDirectory(layout.V2Directory) || layout.V1Directory == layout.V2Directory ||
		!validDirectory(layout.TransactionV1Directory) || !validDirectory(layout.TransactionV2Directory) || layout.TransactionV1Directory == layout.TransactionV2Directory ||
		!filepath.IsAbs(layout.TransactionRoot) || filepath.Clean(layout.TransactionRoot) != layout.TransactionRoot || filepath.Dir(layout.TransactionRoot) == layout.TransactionRoot {
		return fmt.Errorf("invalid release layout values")
	}
	expected := []string{
		"trust-journal" + suffix, "identity-vault" + suffix, "tls-product" + suffix,
		"trust-install-v1" + suffix, "trust-install-v2" + suffix,
		"autonomous-enrollment-install-v1" + suffix, "autonomous-enrollment-install-v2" + suffix,
		"installed-autonomous-product-v1" + suffix, "installed-autonomous-product-v2" + suffix,
		"release-manager-v1" + suffix, "release-manager-v2" + suffix,
		"release-launcher-v1" + suffix, "release-launcher-v2" + suffix,
		"bootstrap-launcher-v1" + suffix, "bootstrap-launcher-v2" + suffix,
		"autonomous-upgrade-validation" + suffix, "old-profile.json", "release-layout.json",
	}
	stageFiles := []string{"autonomous-enrollment-install" + suffix, "identity-vault" + suffix, "installed-autonomous-product" + suffix, "tls-product" + suffix, "trust-install" + suffix, "trust-journal" + suffix, "release-manifest.json"}
	for _, stage := range []string{layout.V1Stage, layout.V2Stage} {
		for _, name := range stageFiles {
			expected = append(expected, filepath.ToSlash(filepath.Join("releases", stage, name)))
		}
	}
	transactionFiles := []string{"install-root-provisioner" + suffix, "install-transaction" + suffix, "installed-bootstrap-manager" + suffix, "installed-release-manager" + suffix, "trust-journal" + suffix}
	bootstrapFiles := []string{"bootstrap-manifest.json", "installed-release-launcher" + suffix, "trust-journal" + suffix}
	for _, version := range []string{"v1", "v2"} {
		for _, name := range transactionFiles {
			expected = append(expected, filepath.ToSlash(filepath.Join("transaction-"+version, name)))
		}
		for _, name := range stageFiles {
			expected = append(expected, filepath.ToSlash(filepath.Join("transaction-"+version, "release-payload", name)))
		}
		for _, name := range bootstrapFiles {
			expected = append(expected, filepath.ToSlash(filepath.Join("transaction-"+version, "bootstrap-payload", name)))
		}
	}
	if len(hashes) != len(expected) {
		return fmt.Errorf("incomplete manifest")
	}
	for _, name := range expected {
		want, ok := hashes[name]
		if !ok {
			return fmt.Errorf("missing fixture hash")
		}
		raw, err := read(filepath.Join(dir, filepath.FromSlash(name)), 100<<20)
		h := sha256.Sum256(raw)
		if err != nil || hex.EncodeToString(h[:]) != want {
			return fmt.Errorf("fixture hash mismatch")
		}
	}
	allowedDirectories := map[string]bool{
		".": true, "releases": true, "bootstrap": true, filepath.ToSlash(filepath.Join("bootstrap", "sets")): true,
		filepath.ToSlash(filepath.Join("releases", layout.V1Stage)): true,
		filepath.ToSlash(filepath.Join("releases", layout.V2Stage)): true,
		"transaction-v1": true, "transaction-v2": true,
		filepath.ToSlash(filepath.Join("transaction-v1", "release-payload")):   true,
		filepath.ToSlash(filepath.Join("transaction-v2", "release-payload")):   true,
		filepath.ToSlash(filepath.Join("transaction-v1", "bootstrap-payload")): true,
		filepath.ToSlash(filepath.Join("transaction-v2", "bootstrap-payload")): true,
	}
	actual := 0
	if err = filepath.WalkDir(dir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("invalid bundle entry")
		}
		name, relErr := filepath.Rel(dir, path)
		if relErr != nil {
			return relErr
		}
		name = filepath.ToSlash(name)
		if entry.IsDir() {
			if !allowedDirectories[name] {
				return fmt.Errorf("unexpected bundle directory")
			}
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("invalid bundle file")
		}
		if name == "SHA256.json" {
			return nil
		}
		if _, ok := hashes[name]; !ok {
			return fmt.Errorf("unexpected bundle file")
		}
		actual++
		return nil
	}); err != nil || actual != len(hashes) {
		return fmt.Errorf("bundle inventory mismatch")
	}
	tool := func(name string) string { return filepath.Join(dir, name+suffix) }
	trustCreated, autonomousCreated, reservationCreated, activationCreated := false, false, false, false
	defer func() {
		if reservationCreated {
			if value, err := command(tool("autonomous-enrollment-install-v2"), nil, "cleanup-reservation-test"); err != nil || value.Status != "cleaned" {
				if result == nil {
					result = fmt.Errorf("reservation cleanup")
				}
			}
		}
		if autonomousCreated {
			if value, err := command(tool("autonomous-enrollment-install-v2"), nil, "cleanup-test"); err != nil || value.Status != "cleaned" {
				if result == nil {
					result = fmt.Errorf("autonomous cleanup")
				}
			}
		}
		if activationCreated {
			if value, err := command(tool("release-manager-v2"), nil, "cleanup-test"); err != nil || value.Status != "cleaned" {
				if result == nil {
					result = fmt.Errorf("activation cleanup")
				}
			}
		}
		if trustCreated {
			if value, err := command(tool("trust-install-v1"), nil, "cleanup-test"); err != nil || value.Status != "usable" {
				if result == nil {
					result = fmt.Errorf("trust cleanup")
				}
			}
		}
	}()
	positive := func(name string, epoch uint64, args ...string) (reply, error) {
		value, err := command(tool(name), nil, args...)
		if err != nil || value.Status != "usable" || (epoch != 0 && value.Epoch != epoch) {
			return value, fmt.Errorf("positive command rejected name=%s op=%v status=%s epoch=%d reason=%s exit=%v", name, args, value.Status, value.Epoch, value.Reason, err)
		}
		return value, nil
	}
	negative := func(name string, args ...string) error {
		value, err := command(tool(name), nil, args...)
		if err == nil || value.Status != "failed" {
			return fmt.Errorf("negative command accepted name=%s op=%v status=%s epoch=%d", name, args, value.Status, value.Epoch)
		}
		return nil
	}
	status := func(name, wantStatus, wantStage string, epoch uint64) (reply, error) {
		value, err := command(tool(name), nil, "status")
		if err != nil || value.Status != wantStatus || value.Stage != wantStage || value.Diagnostic != 1 || value.TrustEpoch != epoch || value.PublicKey != "" {
			return value, fmt.Errorf("invalid minimal diagnostic status=%s stage=%s diagnostic=%d trust_epoch=%d reason=%s exit=%v", value.Status, value.Stage, value.Diagnostic, value.TrustEpoch, value.Reason, err)
		}
		return value, nil
	}
	if _, err = positive("trust-install-v1", 1, "initialize"); err != nil {
		return err
	}
	trustCreated = true
	if value, managerErr := command(tool("release-manager-v1"), nil, "install"); managerErr != nil || value.Status != "active" || value.Epoch != 1 {
		return fmt.Errorf("first release activation")
	}
	activationCreated = true
	if value, managerErr := command(tool("release-manager-v1"), nil, "status"); managerErr != nil || value.Status != "active" || value.Epoch != 1 {
		return fmt.Errorf("first release status")
	}
	if _, statusErr := status("autonomous-enrollment-install-v1", "not_installed", "", 1); statusErr != nil {
		return fmt.Errorf("fresh status: %w", statusErr)
	}
	first, err := positive("autonomous-enrollment-install-v1", 0, "initialize")
	if err != nil || len(first.PublicKey) != 64 || first.EnrollmentState != "idle" {
		return fmt.Errorf("initial autonomous identity")
	}
	autonomousCreated = true
	if value, statusErr := status("autonomous-enrollment-install-v1", "usable", "", 1); statusErr != nil || value.EnrollmentState != "idle" {
		return fmt.Errorf("installed status")
	}
	if err = negative("autonomous-enrollment-install-v1", "initialize"); err != nil {
		return err
	}
	if err = negative("trust-install-v2", "inspect"); err != nil {
		return err
	}
	if err = negative("autonomous-enrollment-install-v2", "inspect"); err != nil {
		return err
	}
	if _, err = positive("trust-install-v2", 2, "upgrade"); err != nil {
		return err
	}
	if value, managerErr := command(tool("release-manager-v2"), nil, "upgrade"); managerErr != nil || value.Status != "active" || value.Epoch != 2 {
		return fmt.Errorf("release upgrade activation")
	}
	for _, manager := range []string{"release-manager-v1", "release-manager-v2"} {
		if value, managerErr := command(tool(manager), nil, "status"); managerErr != nil || value.Status != "active" || value.Epoch != 2 {
			return fmt.Errorf("release status after upgrade")
		}
	}
	if value, managerErr := command(tool("release-manager-v1"), nil, "upgrade"); managerErr == nil || value.Status != "failed" {
		return fmt.Errorf("release rollback accepted")
	}
	for _, name := range []string{"autonomous-enrollment-install-v2", "autonomous-enrollment-install-v1"} {
		value, err := positive(name, 0, "inspect")
		if err != nil || value.PublicKey != first.PublicKey || value.EnrollmentState != "idle" {
			return fmt.Errorf("identity changed across upgrade")
		}
		value, err = status(name, "usable", "", 2)
		if err != nil || value.EnrollmentState != "idle" {
			return fmt.Errorf("status changed across upgrade")
		}
	}
	if err = negative("trust-install-v1", "upgrade"); err != nil {
		return err
	}
	if err = negative("trust-install-v2", "upgrade", filepath.Join(dir, "old-profile.json")); err != nil {
		return err
	}
	if _, err = positive("trust-install-v2", 2, "inspect"); err != nil {
		return err
	}
	if value, cleanupErr := command(tool("autonomous-enrollment-install-v2"), nil, "cleanup-test"); cleanupErr != nil || value.Status != "cleaned" {
		return fmt.Errorf("autonomous cleanup status=%s reason=%s exit=%v", value.Status, value.Reason, cleanupErr)
	}
	autonomousCreated = false
	if err = productReject(tool("release-launcher-v2")); err != nil {
		return err
	}
	if err = productReject(tool("release-launcher-v1")); err != nil {
		return err
	}
	if value, reserveErr := command(tool("autonomous-enrollment-install-v2"), nil, "reserve-test"); reserveErr != nil || value.Status != "maintenance_required" || value.Stage != "binding_pending" {
		return fmt.Errorf("pending reservation")
	}
	reservationCreated = true
	if _, statusErr := status("autonomous-enrollment-install-v2", "maintenance_required", "binding_pending", 2); statusErr != nil {
		return fmt.Errorf("pending status")
	}
	if err = negative("autonomous-enrollment-install-v2", "initialize"); err != nil {
		return err
	}
	if value, cleanupErr := command(tool("autonomous-enrollment-install-v2"), nil, "cleanup-reservation-test"); cleanupErr != nil || value.Status != "cleaned" {
		return fmt.Errorf("reservation cleanup")
	}
	reservationCreated = false
	if _, statusErr := status("autonomous-enrollment-install-v2", "not_installed", "", 2); statusErr != nil {
		return fmt.Errorf("clean status")
	}
	if value, cleanupErr := command(tool("release-manager-v2"), nil, "cleanup-test"); cleanupErr != nil || value.Status != "cleaned" {
		return fmt.Errorf("pre-transaction activation cleanup")
	}
	activationCreated = false
	if value, cleanupErr := command(tool("trust-install-v2"), nil, "cleanup-test"); cleanupErr != nil || value.Status != "usable" {
		return fmt.Errorf("pre-transaction trust cleanup")
	}
	trustCreated = false

	transactionTool := func(version, name string) string {
		return filepath.Join(dir, "transaction-"+version, name+suffix)
	}
	transactionRoot := layout.TransactionRoot
	bootstrapStarted := false
	cleanupTransaction := func(strict bool) error {
		failed := false
		if bootstrapStarted {
			if value, cleanupErr := command(transactionTool("v2", "installed-bootstrap-manager"), nil, "cleanup-pointer-test"); cleanupErr != nil || value.Status != "cleaned" {
				failed = true
			}
			for _, version := range []string{"v2", "v1"} {
				if value, cleanupErr := command(transactionTool(version, "installed-bootstrap-manager"), nil, "cleanup-set-test"); cleanupErr != nil || value.Status != "cleaned" {
					failed = true
				}
			}
		}
		if value, cleanupErr := command(transactionTool("v2", "installed-release-manager"), nil, "cleanup-test"); cleanupErr != nil || value.Status != "cleaned" {
			failed = true
		}
		trustCleanup := filepath.Join(dir, "transaction-v2", "release-payload", "trust-install"+suffix)
		if value, cleanupErr := command(trustCleanup, nil, "cleanup-test"); cleanupErr != nil || value.Status != "usable" {
			failed = true
		}
		for _, version := range []string{"v2", "v1"} {
			if value, cleanupErr := command(transactionTool(version, "installed-release-manager"), nil, "cleanup-release-test"); cleanupErr != nil || value.Status != "cleaned" {
				failed = true
			}
		}
		provisioner := transactionTool("v2", "install-root-provisioner")
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		output, receiptErr := exec.CommandContext(ctx, provisioner).Output()
		cancel()
		receipt, decodeErr := installrelease.DecodeLayoutReceipt(output)
		if receiptErr != nil || decodeErr != nil || installrelease.ValidateLayoutReceipt(receipt, transactionRoot, true) != nil || installrelease.CleanupProtectedTestLayout(receipt, transactionRoot) != nil {
			failed = true
		}
		if strict && failed {
			return fmt.Errorf("transaction cleanup")
		}
		return nil
	}
	transactionStarted := true
	defer func() {
		if transactionStarted {
			_ = cleanupTransaction(false)
		}
	}()
	for _, fixture := range []struct {
		version string
		epoch   uint64
	}{
		{"v1", 1}, {"v2", 2},
	} {
		bootstrapNeedsReopen := false
		for attempt := 0; attempt < 2; attempt++ {
			value, transactionErr := command(transactionTool(fixture.version, "install-transaction"), nil)
			prepared := func(reply reply, commandErr error) bool {
				return commandErr == nil && reply.Status == "prepared" && reply.Epoch == fixture.epoch && (reply.Phase == installrelease.CandidateStaged || reply.Phase == installrelease.CandidatePromoted)
			}
			if !prepared(value, transactionErr) {
				// ImportStaged can durably create and verify the exact protected
				// candidate before Windows reports an uncertain directory receipt.
				// A fresh owner is allowed once; ReconcileCandidate performs no new
				// write when the same signed staging/final candidate already exists.
				reopened, reopenErr := command(transactionTool(fixture.version, "install-transaction"), nil)
				if prepared(reopened, reopenErr) {
					value, transactionErr = reopened, nil
				}
			}
			if !prepared(value, transactionErr) {
				releases := filepath.Join(transactionRoot, "releases")
				stage := filepath.Join(releases, map[string]string{"v1": layout.TransactionV1Stage, "v2": layout.TransactionV2Stage}[fixture.version])
				promoted := filepath.Join(releases, map[string]string{"v1": layout.TransactionV1Directory, "v2": layout.TransactionV2Directory}[fixture.version])
				diagnostic := transactionDiagnostic(stage, promoted, transactionTool(fixture.version, "installed-release-manager"), "trust-install"+suffix, value, transactionErr)
				return fmt.Errorf("transaction %s epoch %d attempt %d: %s", fixture.version, fixture.epoch, attempt+1, diagnostic)
			}
			operation := "install"
			if fixture.version == "v2" {
				operation = "upgrade"
			}
			managed, managerErr := command(transactionTool(fixture.version, "installed-release-manager"), nil, operation)
			if managerErr != nil || managed.Status != "active" || managed.Epoch != fixture.epoch {
				releases := filepath.Join(transactionRoot, "releases")
				stage := filepath.Join(releases, map[string]string{"v1": layout.TransactionV1Stage, "v2": layout.TransactionV2Stage}[fixture.version])
				promoted := filepath.Join(releases, map[string]string{"v1": layout.TransactionV1Directory, "v2": layout.TransactionV2Directory}[fixture.version])
				diagnostic := transactionDiagnostic(stage, promoted, transactionTool(fixture.version, "installed-release-manager"), "trust-install"+suffix, managed, managerErr)
				return fmt.Errorf("manager %s epoch %d attempt %d: %s", fixture.version, fixture.epoch, attempt+1, diagnostic)
			}
			confirmed, confirmErr := command(transactionTool(fixture.version, "install-transaction"), nil)
			if confirmErr != nil || confirmed.Status != "prepared" || confirmed.Epoch != fixture.epoch || confirmed.Phase != installrelease.CandidatePromoted {
				return fmt.Errorf("transaction %s epoch %d promotion confirmation", fixture.version, fixture.epoch)
			}
			value, bootstrapErr := command(transactionTool(fixture.version, "installed-bootstrap-manager"), nil)
			if bootstrapErr != nil || value.Status != "active" || value.Epoch != fixture.epoch {
				// ImportBootstrapStaged and ActivateBootstrap have the same
				// Windows directory-receipt boundary as the release preparation.
				// Give one fresh, fixed manager owner the opportunity to reopen
				// an exact staged/promoted set or an exact active pointer.
				reopened, reopenErr := command(transactionTool(fixture.version, "installed-bootstrap-manager"), nil)
				if reopenErr == nil && reopened.Status == "active" && reopened.Epoch == fixture.epoch {
					value, bootstrapErr = reopened, nil
				} else {
					// Windows may durably replace current.json and still fail the
					// directory durability receipt. Accept that uncertainty only when
					// a separate read proves the exact epoch active, and require the
					// next manager process to reopen it successfully. No write retry or
					// arbitrary failure is treated as success.
					status, statusErr := command(transactionTool(fixture.version, "installed-bootstrap-manager"), nil, "status")
					if attempt != 0 || statusErr != nil || status.Status != "active" || status.Epoch != fixture.epoch {
						return fmt.Errorf("bootstrap %s epoch %d reopen", fixture.version, fixture.epoch)
					}
					bootstrapNeedsReopen = true
					bootstrapStarted = true
					continue
				}
			}
			if bootstrapNeedsReopen {
				bootstrapNeedsReopen = false
			}
			bootstrapStarted = true
		}
		if bootstrapNeedsReopen {
			return fmt.Errorf("bootstrap %s epoch %d restart", fixture.version, fixture.epoch)
		}
		if value, statusErr := command(transactionTool(fixture.version, "installed-bootstrap-manager"), nil, "status"); statusErr != nil || value.Status != "active" || value.Epoch != fixture.epoch {
			return fmt.Errorf("bootstrap %s status", fixture.version)
		}
		if err = productReject(tool("bootstrap-launcher-v1")); err != nil {
			return fmt.Errorf("old stable bootstrap at epoch %d: %w", fixture.epoch, err)
		}
	}
	if err = productReject(tool("bootstrap-launcher-v2")); err != nil {
		return fmt.Errorf("new stable bootstrap: %w", err)
	}
	if value, managerErr := command(transactionTool("v2", "installed-release-manager"), nil, "status"); managerErr != nil || value.Status != "active" || value.Epoch != 2 {
		return fmt.Errorf("transaction active status")
	}
	if err = cleanupTransaction(true); err != nil {
		return err
	}
	transactionStarted = false
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Println("VALIDATION FAILED: autonomous native upgrade lifecycle:", err)
		os.Exit(1)
	}
	fmt.Println("VALIDATION PASSED: signed bootstrap launcher/helper set switches atomically, old and new stable launchers follow the active release, restartable install/upgrade remain idempotent, protected native identity survives upgrade, and TEST state is cleaned. NOT full product session, elevated production root, GUI or signed installer acceptance.")
}
