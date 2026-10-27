// Offline maintenance IPC. Trust/helper paths come only from compiled release
// material. The caller supplies the public identity already created by the
// trusted OS identity provisioning step, never a private key or replacement root.
package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"remoteapp.local/selfhost-poc/autonomousinstall"
	"remoteapp.local/selfhost-poc/clientruntime"
	"remoteapp.local/selfhost-poc/privatepipe"
	"remoteapp.local/selfhost-poc/trustinstall"
	"remoteapp.local/selfhost-poc/trustprofile"
)

var embeddedRelease, embeddedSignerDigest string

type request struct {
	Op             string `json:"op"`
	VaultScope     string `json:"vault_scope"`
	ExpectedPublic string `json:"expected_public"`
}

func run() error {
	if len(os.Args) != 1 {
		return trustprofile.ErrTrust
	}
	r, err := trustinstall.DecodeCompiled(embeddedRelease)
	if err != nil {
		return err
	}
	var req request
	if privatepipe.NewReader(os.Stdin).Read(&req) != nil {
		return trustprofile.ErrTrust
	}
	if req.Op != "initialize" && req.Op != "inspect" && req.Op != "upgrade" {
		return trustprofile.ErrTrust
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	dir := filepath.Dir(exe)
	f, err := r.NativeFactory(filepath.Join(dir, "trust-journal"+suffix))
	if err != nil {
		return err
	}
	open := clientruntime.NativeSigner(filepath.Join(dir, "identity-vault"+suffix), embeddedSignerDigest, req.VaultScope, req.ExpectedPublic)
	identity := func() (autonomousinstall.Identity, error) {
		s, err := open()
		if err != nil {
			return nil, err
		}
		joined, ok := s.(autonomousinstall.Identity)
		if !ok {
			s.Close()
			return nil, trustprofile.ErrTrust
		}
		return joined, nil
	}
	o, err := autonomousinstall.New(r, f, identity, req.VaultScope, req.ExpectedPublic, time.Now)
	if err != nil {
		return err
	}
	switch req.Op {
	case "initialize":
		return o.Initialize(context.Background())
	case "upgrade":
		return o.Upgrade(context.Background())
	default:
		return o.Inspect(context.Background())
	}
}
func main() {
	if run() != nil {
		_ = json.NewEncoder(os.Stdout).Encode(map[string]string{"status": "failed"})
		os.Exit(1)
	}
	_ = json.NewEncoder(os.Stdout).Encode(map[string]string{"status": "ready"})
}
