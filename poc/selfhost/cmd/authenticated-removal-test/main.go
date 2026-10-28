// Fixed TEST-only independent uninstaller client. No runtime selectors or keys.
package main

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"

	"remoteapp.local/selfhost-poc/clientruntime"
	"remoteapp.local/selfhost-poc/installrelease"
	"remoteapp.local/selfhost-poc/localmaintenance"
	"remoteapp.local/selfhost-poc/privatepipe"
	"remoteapp.local/selfhost-poc/programremoval"
	"remoteapp.local/selfhost-poc/trustinstall"
	"remoteapp.local/selfhost-poc/trustprofile"
)

var embeddedRelease, embeddedInstallRoot, embeddedAddress, embeddedVaultScope, embeddedPublic, embeddedSignerDigest string

func decoded(value string) (string, error) {
	raw, e := base64.RawURLEncoding.DecodeString(value)
	if e != nil || base64.RawURLEncoding.EncodeToString(raw) != value {
		return "", trustprofile.ErrTrust
	}
	return string(raw), nil
}

func run() (result trustprofile.MaintenanceState, err error) {
	if len(os.Args) != 1 {
		return result, trustprofile.ErrTrust
	}
	r, e := trustinstall.DecodeCompiled(embeddedRelease)
	if e != nil || !r.Specification().Test {
		return result, trustprofile.ErrTrust
	}
	root, e := decoded(embeddedInstallRoot)
	if e != nil {
		return result, e
	}
	address, e := decoded(embeddedAddress)
	if e != nil {
		return result, e
	}
	if _, e = installrelease.ValidateProtectedBootstrapLayout(root, true); e != nil {
		return result, e
	}
	exe, e := os.Executable()
	if e != nil {
		return result, e
	}
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	dir := filepath.Dir(exe)
	factory, e := r.NativeFactory(filepath.Join(dir, "trust-journal"+suffix))
	if e != nil {
		return result, e
	}
	signer, e := clientruntime.NativeSigner(filepath.Join(dir, "identity-vault"+suffix), embeddedSignerDigest, embeddedVaultScope, embeddedPublic)()
	if e != nil {
		return result, e
	}
	defer func() {
		if closeErr := signer.(*privatepipe.Signer).CloseJoined(); err == nil {
			err = closeErr
		}
	}()
	public, e := hex.DecodeString(embeddedPublic)
	if e != nil {
		return result, trustprofile.ErrTrust
	}
	expected, e := localmaintenance.RequestTest(context.Background(), r, address, public, signer)
	if e != nil {
		return result, e
	}
	if e = signer.(*privatepipe.Signer).CloseJoined(); e != nil {
		return expected, e
	}
	return programremoval.RunExpectedTest(context.Background(), r, factory, root, expected)
}

func main() {
	s, e := run()
	status, reason := s.State, ""
	if e != nil {
		status = "failed"
		reason = "authenticated_removal_rejected"
		if e == installrelease.ErrRemovalReceipt {
			reason = "program_removal_receipt_uncertain"
		}
	}
	json.NewEncoder(os.Stdout).Encode(struct {
		Status      string `json:"status"`
		Epoch       uint64 `json:"epoch,omitempty"`
		Transaction string `json:"transaction,omitempty"`
		Reason      string `json:"reason,omitempty"`
	}{status, s.Epoch, s.Transaction, reason})
	if e != nil {
		os.Exit(1)
	}
}
