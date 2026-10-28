// Fixed entry for trusted DMG/Inno coordinators. No runtime arguments.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"remoteapp.local/selfhost-poc/installeradapter"
	"remoteapp.local/selfhost-poc/trustinstall"
)

var embeddedRelease, embeddedOperation, embeddedEpoch, embeddedTransactionDigest, embeddedManagerDigest, embeddedBootstrapDigest string

func run() (uint64, error) {
	if len(os.Args) != 1 {
		return 0, installeradapter.ErrInstall
	}
	release, err := trustinstall.DecodeCompiled(embeddedRelease)
	epoch, epochErr := strconv.ParseUint(embeddedEpoch, 10, 64)
	if err != nil || epochErr != nil || strconv.FormatUint(epoch, 10) != embeddedEpoch || epoch < release.Specification().MinimumEpoch {
		return 0, installeradapter.ErrInstall
	}
	exe, err := os.Executable()
	if err != nil {
		return 0, err
	}
	dir := filepath.Dir(exe)
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	pins := map[string]string{"install-transaction": embeddedTransactionDigest, "installed-release-manager": embeddedManagerDigest, "installed-bootstrap-manager": embeddedBootstrapDigest}
	verify := func(name string) error {
		pin, ok := pins[name]
		decoded, err := hex.DecodeString(pin)
		if !ok || err != nil || len(decoded) != 32 || hex.EncodeToString(decoded) != pin {
			return installeradapter.ErrInstall
		}
		path := filepath.Join(dir, name+suffix)
		st, err := os.Lstat(path)
		if err != nil || !st.Mode().IsRegular() || st.Size() < 1 || st.Size() > 100<<20 {
			return installeradapter.ErrInstall
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		opened, err := f.Stat()
		if err != nil || !os.SameFile(st, opened) {
			return installeradapter.ErrInstall
		}
		h := sha256.New()
		if _, err = io.Copy(h, f); err != nil || hex.EncodeToString(h.Sum(nil)) != pin {
			return installeradapter.ErrInstall
		}
		return nil
	}
	// Verify the entire child chain before the first mutating invocation.
	for name := range pins {
		if verify(name) != nil {
			return 0, installeradapter.ErrInstall
		}
	}
	invoke := func(name string, args ...string) (installeradapter.Reply, error) {
		if verify(name) != nil {
			return installeradapter.Reply{}, installeradapter.ErrInstall
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, filepath.Join(dir, name+suffix), args...)
		cmd.Stderr = io.Discard
		var out bytes.Buffer
		cmd.Stdout = &out
		runErr := cmd.Run()
		if ctx.Err() != nil || out.Len() > 4096 {
			return installeradapter.Reply{}, installeradapter.ErrInstall
		}
		var r installeradapter.Reply
		decoder := json.NewDecoder(&out)
		decoder.DisallowUnknownFields()
		if decoder.Decode(&r) != nil || decoder.Decode(new(any)) != io.EOF {
			return r, installeradapter.ErrInstall
		}
		return r, runErr
	}
	if installeradapter.Run(embeddedOperation, epoch, invoke) != nil {
		return 0, installeradapter.ErrInstall
	}
	return epoch, nil
}

func main() {
	epoch, err := run()
	if err != nil {
		_ = json.NewEncoder(os.Stdout).Encode(installeradapter.Reply{Status: "failed", Reason: "installer_adapter_rejected"})
		os.Exit(1)
	}
	_ = json.NewEncoder(os.Stdout).Encode(installeradapter.Reply{Status: "active", Epoch: epoch})
}
