//go:build darwin || linux

// desktop-test-authority owns a one-shot, isolated desktop acceptance service.
package main

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"remoteapp.local/selfhost-poc/broker"
	"remoteapp.local/selfhost-poc/headscale"
	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/verifier"
)

var errStage = errors.New("desktop_test_stage_unavailable")

const (
	preparedFile = "authority-prepared"
	startedFile  = "authority-started"
	joinedFile   = "authority-joined"
	retiredFile  = "authority-retired"
)

func writeOnce(root, name string, b []byte) error {
	f, err := os.CreateTemp(root, ".authority-*")
	if err != nil {
		return errStage
	}
	defer os.Remove(f.Name())
	_, writeErr := f.Write(b)
	syncErr := f.Sync()
	closeErr := f.Close()
	if writeErr != nil || syncErr != nil || closeErr != nil ||
		os.Link(f.Name(), filepath.Join(root, name)) != nil || os.Remove(f.Name()) != nil {
		return errStage
	}
	d, err := os.Open(root)
	if err != nil {
		return errStage
	}
	defer d.Close()
	if d.Sync() != nil {
		return errStage
	}
	return nil
}

func privatePath(path string, mode os.FileMode) bool {
	i, err := os.Lstat(path)
	if err != nil || i.Mode() != mode {
		return false
	}
	st, ok := i.Sys().(*syscall.Stat_t)
	return ok && st.Uid == uint32(os.Geteuid()) && (mode.IsDir() || st.Nlink == 1)
}

func marker(root, name, want string) bool {
	path := filepath.Join(root, name)
	if !privatePath(path, 0600) {
		return false
	}
	i, err := os.Stat(path)
	if err != nil || i.Size() != int64(len(want)) {
		return false
	}
	b, err := os.ReadFile(path)
	return err == nil && string(b) == want
}

func prepare(root string) error {
	if _, err := headscale.PrepareDesktopTest(root); err != nil {
		return errStage
	}
	store, err := invite.OpenDesktopAuthorization(filepath.Join(root, "authority"), time.Now)
	if err != nil {
		return errStage
	}
	issuer := hex.EncodeToString(store.PublicKey())
	if store.Close() != nil {
		return errStage
	}
	return writeOnce(root, preparedFile, []byte(issuer))
}

func openStage(root string) (*invite.Store, *headscale.DesktopTestInstance, error) {
	instance, err := headscale.OpenDesktopTest(root)
	state := filepath.Join(root, "authority")
	if err != nil || !privatePath(state, os.ModeDir|0700) ||
		!privatePath(filepath.Join(state, "state.json"), 0600) {
		return nil, nil, errStage
	}
	store, err := invite.OpenDesktopAuthorization(state, time.Now)
	if err != nil {
		return nil, nil, errStage
	}
	if !marker(root, preparedFile, hex.EncodeToString(store.PublicKey())) {
		store.Close()
		return nil, nil, errStage
	}
	return store, instance, nil
}

func serve(ctx context.Context, root string) (result error) {
	store, instance, err := openStage(root)
	if err != nil {
		return err
	}
	defer func() {
		if store.Close() != nil {
			result = errStage
		}
		// Only publish a joined receipt after HTTPS, broker, supervisor,
		// verifier and Store have all completed their real shutdown.
		if result == nil {
			result = writeOnce(root, joinedFile, []byte("joined"))
		}
	}()
	if ctx.Err() != nil || len(store.ProvisionRecords()) != 0 || writeOnce(root, startedFile, []byte("started")) != nil {
		return errStage
	}
	driver := &headscale.Driver{Run: instance.Run, PolicyDir: instance.PolicyDirectory(), PolicyMode: "database"}
	policy, cancel := context.WithTimeout(ctx, 10*time.Second)
	err = driver.ReplaceRules(policy, nil)
	cancel()
	if err != nil {
		return errStage
	}
	// Keep dependent owners alive until the HTTP and broker barriers return.
	v, err := verifier.OpenDesktopTest(context.WithoutCancel(ctx), filepath.Join(root, "verifier"), true, driver, nil)
	if err != nil {
		return errStage
	}
	defer func() {
		if v.Stop(context.Background()) != nil {
			result = errStage
		}
	}()
	if ctx.Err() != nil {
		return errStage
	}
	super, err := broker.StartDesktopTestHeadscaleNetworkSupervisor(context.WithoutCancel(ctx), store, v, instance, time.Second)
	if err != nil {
		return errStage
	}
	defer func() {
		if super.Stop(context.Background()) != nil || !super.DenyAllConfirmed() {
			result = errStage
		}
	}()
	readyCtx, cancelReady := context.WithTimeout(ctx, 10*time.Second)
	ready := super.WaitReady(readyCtx)
	cancelReady()
	if !ready || ctx.Err() != nil {
		return errStage
	}
	return serveEndpoint(ctx, root, hex.EncodeToString(store.PublicKey()), super.Authority(), super.Done())
}

func retire(ctx context.Context, root string) (result error) {
	store, instance, err := openStage(root)
	if err != nil {
		return err
	}
	defer func() {
		if store.Close() != nil {
			result = errStage
		}
	}()
	if !marker(root, startedFile, "started") || !marker(root, joinedFile, "joined") {
		return errStage
	}
	if marker(root, retiredFile, "retired") {
		return nil
	}
	driver := &headscale.Driver{Run: instance.Run, PolicyDir: instance.PolicyDirectory(), PolicyMode: "database"}
	if err := driver.ReplaceRules(ctx, nil); err != nil {
		return errStage
	}
	for _, record := range store.ProvisionRecords() {
		if record.Status == "cleaned" {
			continue
		}
		if store.ProvisionCleanup(record.ID, false) != nil || driver.Cleanup(ctx, record) != nil ||
			store.ProvisionCleanup(record.ID, true) != nil {
			return errStage
		}
	}
	if verifier.RetireDesktopTest(ctx, filepath.Join(root, "verifier"), driver) != nil {
		return errStage
	}
	return writeOnce(root, retiredFile, []byte("retired"))
}

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, errStage)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	var err error
	switch os.Args[1] {
	case "prepare":
		err = prepare(os.Args[2])
	case "serve":
		err = serve(ctx, os.Args[2])
	case "retire":
		work, cancel := context.WithTimeout(ctx, 30*time.Second)
		err = retire(work, os.Args[2])
		cancel()
	default:
		err = errStage
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, errStage)
		os.Exit(1)
	}
}
