package main

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"remoteapp.local/selfhost-poc/testretirement"
	"remoteapp.local/selfhost-poc/trustinstall"
	"runtime"
)

var embeddedRelease, embeddedInstallRoot, embeddedSignerDigest string

func run() error {
	finish := len(os.Args) == 2 && os.Args[1] == "finish-files"
	if len(os.Args) != 1 && !finish {
		return os.ErrInvalid
	}
	r, e := trustinstall.DecodeCompiled(embeddedRelease)
	if e != nil {
		return e
	}
	raw, e := base64.RawURLEncoding.DecodeString(embeddedInstallRoot)
	if e != nil || base64.RawURLEncoding.EncodeToString(raw) != embeddedInstallRoot {
		return os.ErrInvalid
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
	if finish {
		return testretirement.FinishFiles(r, string(raw), filepath.Join(dir, "trust-journal"+suffix))
	}
	return testretirement.Idle(r, string(raw), filepath.Join(dir, "trust-journal"+suffix), filepath.Join(dir, "identity-vault"+suffix), embeddedSignerDigest)
}
func main() {
	if e := run(); e != nil {
		json.NewEncoder(os.Stdout).Encode(map[string]string{"status": "test_retirement_failed_state_preserved", "reason": e.Error()})
		os.Exit(1)
	}
	json.NewEncoder(os.Stdout).Encode(map[string]string{"status": "idle_test_retired"})
}
