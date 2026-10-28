// TEST-only fixed-root removal coordinator. Not a production uninstaller.
package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"

	"remoteapp.local/selfhost-poc/installrelease"
	"remoteapp.local/selfhost-poc/programremoval"
	"remoteapp.local/selfhost-poc/trustinstall"
	"remoteapp.local/selfhost-poc/trustprofile"
)

var embeddedRelease, embeddedInstallRoot string

func run() (trustprofile.MaintenanceState, error) {
	if len(os.Args) != 1 {
		return trustprofile.MaintenanceState{}, trustprofile.ErrTrust
	}
	r, err := trustinstall.DecodeCompiled(embeddedRelease)
	if err != nil || !r.Specification().Test {
		return trustprofile.MaintenanceState{}, trustprofile.ErrTrust
	}
	raw, err := base64.RawURLEncoding.DecodeString(embeddedInstallRoot)
	if err != nil || base64.RawURLEncoding.EncodeToString(raw) != embeddedInstallRoot {
		return trustprofile.MaintenanceState{}, trustprofile.ErrTrust
	}
	exe, err := os.Executable()
	if err != nil {
		return trustprofile.MaintenanceState{}, err
	}
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	factory, err := r.NativeFactory(filepath.Join(filepath.Dir(exe), "trust-journal"+suffix))
	if err != nil {
		return trustprofile.MaintenanceState{}, err
	}
	return programremoval.RunTest(context.Background(), r, factory, string(raw))
}

func main() {
	s, err := run()
	if err != nil {
		reason := "program_removal_rejected"
		if err == installrelease.ErrRemovalReceipt {
			reason = "program_removal_receipt_uncertain"
		}
		json.NewEncoder(os.Stdout).Encode(map[string]string{"status": "failed", "reason": reason})
		os.Exit(1)
	}
	json.NewEncoder(os.Stdout).Encode(struct {
		Status string `json:"status"`
		Epoch  uint64 `json:"epoch"`
	}{s.State, s.Epoch})
}
