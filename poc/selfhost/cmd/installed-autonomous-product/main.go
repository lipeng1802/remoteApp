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
	"remoteapp.local/selfhost-poc/startuptrace"
	"runtime"
	"sync"
	"syscall"
	"time"

	"remoteapp.local/selfhost-poc/autonomousbinding"
	"remoteapp.local/selfhost-poc/cleanupwitness"
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
var embeddedTestTunnel string

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
	encoder := json.NewEncoder(os.Stdout)
	var outputMu sync.Mutex
	output := func(value any) error { outputMu.Lock(); defer outputMu.Unlock(); return encoder.Encode(value) }
	w := &cleanupwitness.Witness{}
	err := runObserved(output, w)
	w.Seal()
	if err != nil {
		status := "failed"
		if w.Verified() {
			status = "fault_joined"
		}
		_ = output(map[string]string{"status": status})
		os.Exit(1)
	}
	_ = output(map[string]string{"status": "closed"})
}

func run(output func(any) error) (result error) {
	return runObserved(output, nil)
}
func runObserved(output func(any) error, w *cleanupwitness.Witness) (result error) {
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
	w.Begin(cleanupwitness.OwnerJournal)
	owner, err := clientowner.OpenExactRelease(release, factory, time.Now)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := owner.Close(context.Background()); closeErr != nil {
			result = closeErr
		}
		var cleanErr error
		if !owner.JournalClosedCleanly(context.Background()) {
			cleanErr = trustprofile.ErrTrust
		}
		w.Complete(cleanupwitness.OwnerJournal, cleanErr)
	}()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	ctx = cleanupwitness.Context(ctx, w)
	ctx = startuptrace.Context(ctx, release.Specification().Test)
	defer cancel()
	requests := make(chan installeddevice.Request)
	readerDone := make(chan struct{})
	w.Begin(cleanupwitness.CommandReader)
	go func() {
		defer close(readerDone)
		defer close(requests)
		reader := privatepipe.NewReader(os.Stdin)
		for {
			var request installeddevice.Request
			readErr := reader.Read(&request)
			if readErr != nil {
				request = installeddevice.Request{Op: "invalid"}
			}
			select {
			case requests <- request:
			case <-ctx.Done():
				return
			}
			if readErr != nil {
				return
			}
		}
	}()
	defer func() { cancel(); _ = os.Stdin.Close(); <-readerDone; w.Complete(cleanupwitness.CommandReader, nil) }()

	var first installeddevice.Request
	select {
	case request, ok := <-requests:
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
	w.Begin(cleanupwitness.BindingJournal)
	binding, err := autonomousbinding.OpenWithJournal(release.Scope(), release.Specification().Test, bindingRecords(recordFactory))
	if err != nil {
		return trustprofile.ErrTrust
	}
	if binding.Check() != nil {
		_ = binding.Close()
		return trustprofile.ErrTrust
	}
	defer func() {
		closeErr := binding.Close()
		w.Complete(cleanupwitness.BindingJournal, closeErr)
		if closeErr != nil {
			result = closeErr
		}
	}()
	installed := binding.Record()
	open := clientruntime.NativeSigner(filepath.Join(dir, "identity-vault"+suffix), embeddedSignerDigest, installed.VaultScope, installed.PublicKey)
	var observation *clientruntime.ProductObservation
	if release.Specification().Test {
		observation = &clientruntime.ProductObservation{}
	}
	product := clientruntime.Product{Observation: observation, Spec: processowner.Spec{
		Executable: filepath.Join(dir, "tls-product"+suffix),
		SHA256:     embeddedProductDigest,
	}}
	commands := make(chan installeddevice.Request)
	makeRunner := installeddevice.WatchedEnrollmentRunnerWithSession
	if embeddedTestTunnel != "" {
		if embeddedTestTunnel != "loopback18444" || !release.Specification().Test {
			return trustprofile.ErrTrust
		}
		makeRunner = installeddevice.WatchedSessionViaTestTunnel
	}
	runner := makeRunner(
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
	// Routing failures can return before the runner has observed cancellation.
	// This defer is registered after binding.Close, so the runner and signer
	// must actually join before the installed binding lease is released.
	defer func() {
		if stopErr := owner.Stop(context.Background()); stopErr != nil {
			result = stopErr
		}
	}()
	// Revocation, watch failure, product failure and local stop all converge on
	// the same joined owner path. No force-kill or automatic re-enrollment here.
	return installeddevice.ControlProductObserved(ctx, release, owner, handle, requests, commands, output, observation)
}
