// Explicit autonomous product entry. stdin/stdout MUST be dedicated trusted
// local IPC, never a terminal logger. This binary is built separately from the
// default device-only client and cannot be enabled by a runtime request.
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

	"remoteapp.local/selfhost-poc/autonomousbinding"
	"remoteapp.local/selfhost-poc/clientowner"
	"remoteapp.local/selfhost-poc/clientruntime"
	"remoteapp.local/selfhost-poc/enrollmentstore"
	"remoteapp.local/selfhost-poc/installeddevice"
	"remoteapp.local/selfhost-poc/privatepipe"
	"remoteapp.local/selfhost-poc/processowner"
	"remoteapp.local/selfhost-poc/trustinstall"
	"remoteapp.local/selfhost-poc/trustprofile"
)

var embeddedRelease, embeddedSignerDigest, embeddedProductDigest string

func bindingRecords(records trustinstall.RecordFactory) autonomousbinding.JournalFactory {
	return func(scope string, create, test bool) (autonomousbinding.Journal, error) {
		return records(scope, create, test)
	}
}

func enrollmentRecords(records trustinstall.RecordFactory) enrollmentstore.JournalFactory {
	return func(scope string, create, test bool) (enrollmentstore.Journal, error) {
		return records(scope, create, test)
	}
}

func main() {
	output := json.NewEncoder(os.Stdout).Encode
	if run(output) != nil {
		_ = output(map[string]string{"status": "failed"})
		os.Exit(1)
	}
	_ = output(map[string]string{"status": "closed"})
}

func run(output func(any) error) (result error) {
	if len(os.Args) != 1 || embeddedProductDigest == "" {
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
		if closeErr := owner.Close(context.Background()); closeErr != nil {
			result = closeErr
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
			readErr := reader.Read(&request)
			if readErr != nil {
				request = installeddevice.Request{Op: "invalid"}
			}
			select {
			case commands <- request:
			case <-ctx.Done():
				return
			}
			if readErr != nil {
				return
			}
		}
	}()
	defer func() { cancel(); _ = os.Stdin.Close(); <-readerDone }()

	var first installeddevice.Request
	select {
	case request, ok := <-commands:
		if !ok {
			return trustprofile.ErrTrust
		}
		first = request
	case <-ctx.Done():
		return ctx.Err()
	}
	if !installeddevice.ValidAutonomousOpen(first) {
		return trustprofile.ErrTrust
	}
	recordFactory, err := release.NativeRecordFactory(filepath.Join(dir, "trust-journal"+suffix))
	if err != nil {
		return trustprofile.ErrTrust
	}
	binding, err := autonomousbinding.OpenWithJournal(release.Scope(), release.Specification().Test, bindingRecords(recordFactory))
	if err != nil {
		return trustprofile.ErrTrust
	}
	if binding.Check() != nil {
		_ = binding.Close()
		return trustprofile.ErrTrust
	}
	defer func() {
		if closeErr := binding.Close(); closeErr != nil {
			result = closeErr
		}
	}()
	installed := binding.Record()
	open := clientruntime.NativeSigner(filepath.Join(dir, "identity-vault"+suffix), embeddedSignerDigest, installed.VaultScope, installed.PublicKey)
	product := clientruntime.Product{Spec: processowner.Spec{
		Executable: filepath.Join(dir, "tls-product"+suffix),
		SHA256:     embeddedProductDigest,
	}}
	runner := installeddevice.WatchedEnrollmentRunnerWithSession(
		release,
		open,
		commands,
		output,
		installeddevice.NativeEnrollmentFactoryWithJournal(installed.EnrollmentScope, release.Specification().Test, enrollmentRecords(recordFactory)),
		installeddevice.ProductSession(product),
	)
	handle, err := owner.Start(ctx, runner)
	if err != nil {
		return err
	}
	// Revocation, watch failure, product failure and local stop all converge on
	// the same joined owner path. No force-kill or automatic re-enrollment here.
	return handle.Wait(context.Background())
}
