// Installed device API entry. stdin/stdout MUST be dedicated trusted local IPC,
// never a terminal logger. No endpoint, CA, root, signer path or key arguments.
package main

import (
	"context"
	"encoding/json"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"remoteapp.local/selfhost-poc/clientowner"
	"remoteapp.local/selfhost-poc/clientruntime"
	"remoteapp.local/selfhost-poc/installeddevice"
	"remoteapp.local/selfhost-poc/privatepipe"
	"remoteapp.local/selfhost-poc/trustinstall"
	"remoteapp.local/selfhost-poc/trustprofile"
)

var embeddedRelease, embeddedSignerDigest string

func main() {
	output := json.NewEncoder(os.Stdout).Encode
	if run(output) != nil {
		output(map[string]string{"status": "failed"})
		os.Exit(1)
	}
	output(map[string]string{"status": "closed"})
}

func run(output func(any) error) (result error) {
	if len(os.Args) != 1 {
		return trustprofile.ErrTrust
	}
	release, err := trustinstall.DecodeCompiled(embeddedRelease)
	if err != nil {
		return err
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
	factory, err := release.NativeFactory(filepath.Join(dir, "trust-journal"+suffix))
	if err != nil {
		return err
	}
	owner, err := clientowner.Open(release, factory, time.Now)
	if err != nil {
		return err
	}
	defer func() {
		if err := owner.Close(context.Background()); err != nil {
			result = err
		}
	}()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	commands := make(chan installeddevice.Request)
	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		defer close(commands)
		reader := privatepipe.NewReader(os.Stdin)
		for {
			var request installeddevice.Request
			err := reader.Read(&request)
			if err != nil {
				request = installeddevice.Request{Op: "invalid"}
			}
			select {
			case commands <- request:
			case <-ctx.Done():
				return
			}
			if err != nil {
				return
			}
		}
	}()
	defer func() { cancel(); os.Stdin.Close(); <-readerDone }()
	var first installeddevice.Request
	select {
	case first = <-commands:
	case <-ctx.Done():
		return ctx.Err()
	}
	if !installeddevice.ValidOpen(first) {
		return trustprofile.ErrTrust
	}
	open := clientruntime.NativeSigner(filepath.Join(dir, "identity-vault"+suffix), embeddedSignerDigest, first.VaultScope, first.ExpectedPublic)
	handle, err := owner.Start(ctx, installeddevice.Runner(release, open, commands, output))
	if err != nil {
		return err
	}
	// Expiry/signal/stop terminate Runner, then HTTP/signing workers actually
	// join before journal Close and the final closed frame. No force kill here.
	return handle.Wait(context.Background())
}
