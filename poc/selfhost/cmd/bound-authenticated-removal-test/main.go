// Fixed TEST-only independent client. All selectors are compiled; the native
// binding selects identity. No argument, environment or peer supplies a key.
package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"remoteapp.local/selfhost-poc/autonomousbinding"
	"remoteapp.local/selfhost-poc/clientruntime"
	"remoteapp.local/selfhost-poc/privatepipe"
	"remoteapp.local/selfhost-poc/programremoval"
	"remoteapp.local/selfhost-poc/trustinstall"
	"remoteapp.local/selfhost-poc/trustprofile"
)

var embeddedRelease, embeddedInstallRoot, embeddedAddress, embeddedSignerDigest string

func decoded(value string) (string, error) {
	raw, e := base64.RawURLEncoding.DecodeString(value)
	if e != nil || base64.RawURLEncoding.EncodeToString(raw) != value {
		return "", trustprofile.ErrTrust
	}
	return string(raw), nil
}
func run() (trustprofile.MaintenanceState, error) {
	var none trustprofile.MaintenanceState
	if len(os.Args) != 1 {
		return none, trustprofile.ErrTrust
	}
	r, e := trustinstall.DecodeCompiled(embeddedRelease)
	if e != nil || !r.Specification().Test {
		return none, trustprofile.ErrTrust
	}
	life, cancel := context.WithDeadline(context.Background(), time.Unix(r.Specification().Bundled.Claims.ExpiresAt, 0))
	defer cancel()
	closeDone := make(chan struct{})
	stopClose := context.AfterFunc(life, func() { os.Stdin.Close(); close(closeDone) })
	defer func() {
		if !stopClose() {
			<-closeDone
		}
	}()
	reader := privatepipe.NewBoundedReader(os.Stdin, 4096)
	var initial struct{ Op string }
	if reader.Read(&initial) != nil || initial.Op != "arm" {
		return none, trustprofile.ErrTrust
	}
	root, e := decoded(embeddedInstallRoot)
	if e != nil {
		return none, e
	}
	address, e := decoded(embeddedAddress)
	if e != nil {
		return none, e
	}
	exe, e := os.Executable()
	if e != nil {
		return none, e
	}
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	dir := filepath.Dir(exe)
	helper := filepath.Join(dir, "trust-journal"+suffix)
	factory, e := r.NativeFactory(helper)
	if e != nil {
		return none, e
	}
	records, e := r.NativeRecordFactory(helper)
	if e != nil {
		return none, e
	}
	return programremoval.ArmedAuthenticatedBoundTest(life, r, factory, func(scope string, create, test bool) (autonomousbinding.Journal, error) {
		return records(scope, create, test)
	}, root, address, func(record autonomousbinding.Record) (programremoval.BoundSigner, error) {
		s, e := clientruntime.NativeSigner(filepath.Join(dir, "identity-vault"+suffix), embeddedSignerDigest, record.VaultScope, record.PublicKey)()
		if e != nil {
			return nil, e
		}
		return s.(*privatepipe.Signer), nil
	}, func(ctx context.Context) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if e := json.NewEncoder(os.Stdout).Encode(map[string]string{"status": "removal_armed"}); e != nil {
			return e
		}
		var request struct{ Op string }
		if reader.Read(&request) != nil || request.Op != "remove" || ctx.Err() != nil {
			return trustprofile.ErrTrust
		}
		return nil
	})
}
func main() {
	s, e := run()
	if e != nil {
		json.NewEncoder(os.Stdout).Encode(map[string]string{"status": "failed", "reason": "bound_authenticated_removal_rejected"})
		os.Exit(1)
	}
	json.NewEncoder(os.Stdout).Encode(struct {
		Status      string `json:"status"`
		Epoch       uint64 `json:"epoch"`
		Transaction string `json:"transaction"`
	}{s.State, s.Epoch, s.Transaction})
}
