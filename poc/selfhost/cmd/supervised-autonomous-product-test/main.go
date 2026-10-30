// TEST launcher with trusted private business IPC and independent maintenance.
// It selects identity from native binding; no argv/root/key/endpoint selectors.
package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"remoteapp.local/selfhost-poc/autonomousbinding"
	"remoteapp.local/selfhost-poc/clientruntime"
	"remoteapp.local/selfhost-poc/installeddevice"
	"remoteapp.local/selfhost-poc/localmaintenance"
	"remoteapp.local/selfhost-poc/privatepipe"
	"remoteapp.local/selfhost-poc/processowner"
	"remoteapp.local/selfhost-poc/productmaintenance"
	"remoteapp.local/selfhost-poc/trustinstall"
	"remoteapp.local/selfhost-poc/trustprofile"
)

var embeddedRelease, embeddedProductPath, embeddedProductDigest, embeddedSignerDigest string
var embeddedTestTunnel string

func run(output func(any) error) (result error) {
	if len(os.Args) != 1 {
		return trustprofile.ErrTrust
	}
	r, e := trustinstall.DecodeCompiled(embeddedRelease)
	if e != nil || !r.Specification().Test {
		return trustprofile.ErrTrust
	}
	product, e := base64.RawURLEncoding.DecodeString(embeddedProductPath)
	if e != nil || base64.RawURLEncoding.EncodeToString(product) != embeddedProductPath {
		return trustprofile.ErrTrust
	}
	reader := privatepipe.NewReader(os.Stdin)
	var first installeddevice.Request
	if reader.Read(&first) != nil || !installeddevice.ValidAutonomousOpen(first) {
		return trustprofile.ErrTrust
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
	helper := filepath.Join(dir, "trust-journal"+suffix)
	factory, e := r.NativeFactory(helper)
	if e != nil {
		return e
	}
	records, e := r.NativeRecordFactory(helper)
	if e != nil {
		return e
	}
	bf := func(scope string, create, test bool) (autonomousbinding.Journal, error) {
		return records(scope, create, test)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	controller, e := productmaintenance.StartTest(ctx, r, processowner.Spec{Executable: string(product), SHA256: embeddedProductDigest}, factory, bf, func(record autonomousbinding.Record) (productmaintenance.Signer, error) {
		s, e := clientruntime.NativeSigner(filepath.Join(dir, "identity-vault"+suffix), embeddedSignerDigest, record.VaultScope, record.PublicKey)()
		if e != nil {
			return nil, e
		}
		return s.(*privatepipe.Signer), nil
	})
	if controller != nil {
		defer func() {
			if e := controller.Release(context.Background()); e != nil {
				result = e
			}
		}()
	}
	if e != nil {
		return e
	}
	if embeddedTestTunnel != "" && embeddedTestTunnel != "loopback18444" {
		return trustprofile.ErrTrust
	}
	if e = controller.ConfigureRevocation(ctx, embeddedTestTunnel == "loopback18444"); e != nil {
		return e
	}
	l, e := net.Listen("tcp4", "127.0.0.1:0")
	if e != nil {
		return e
	}
	endpoint, e := localmaintenance.NewTest(r, controller, controller.Signer(), l)
	if e != nil {
		l.Close()
		return e
	}
	type doneReply struct {
		state trustprofile.MaintenanceState
		err   error
	}
	done := make(chan doneReply, 1)
	go func() { state, e := endpoint.Serve(ctx); done <- doneReply{state, e}; close(done) }()
	defer func() { endpoint.Close(); <-done }()
	requests := make(chan installeddevice.Request)
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		defer close(requests)
		for {
			var request installeddevice.Request
			e := reader.Read(&request)
			if e != nil {
				request = installeddevice.Request{Op: "invalid"}
			}
			select {
			case requests <- request:
			case <-ctx.Done():
				return
			}
			if e != nil {
				return
			}
		}
	}()
	defer func() { cancel(); os.Stdin.Close(); <-readDone }()
	controlPipe, closeControl, e := startDesktopControl(ctx, controller)
	if e != nil {
		return e
	}
	defer func() {
		if e := closeControl(); e != nil {
			result = e
		}
	}()
	identity := controller.Identity()
	metadata := map[string]any{"status": "ready", "maintenance_address": l.Addr().String(), "vault_scope": identity.VaultScope, "public_key": identity.PublicKey}
	if controlPipe != "" {
		if runtime.GOOS == "windows" {
			metadata["control_pipe"] = controlPipe
		} else {
			metadata["control_socket"] = controlPipe
		}
	}
	if e = output(metadata); e != nil {
		return e
	}
	var completed <-chan doneReply = done
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case value := <-completed:
			completed = nil
			if value.err == nil {
				if e = output(map[string]any{"status": "maintenance_closed", "epoch": value.state.Epoch, "transaction": value.state.Transaction}); e != nil {
					return e
				}
				return nil
			}
			// Failed authentication/confirmation does not silently stop a live
			// product. Trusted parent must explicitly stop and join this launcher.
			if e = output(map[string]string{"status": "maintenance_endpoint_closed"}); e != nil {
				return e
			}
		case request, ok := <-requests:
			if !ok {
				return trustprofile.ErrTrust
			}
			if request == (installeddevice.Request{Op: "stop"}) {
				return nil
			}
			if request == (installeddevice.Request{Op: "stop_fault"}) {
				life, stop := context.WithTimeout(ctx, 20*time.Second)
				e := controller.ReleaseFault(life)
				stop()
				if e != nil {
					if e = output(map[string]string{"status": "fault_stop_rejected"}); e != nil {
						return e
					}
					continue
				}
				// Parent may close lifecycle stdin only after native revocation
				// confirmation + product/signer join. Final proof follows control
				// handler and inherited reader cleanup, not this intermediate ACK.
				if e = output(map[string]any{"status": "fault_stop_ready", "joined": true, "fault": true}); e != nil {
					return e
				}
				return productmaintenance.ErrFaultJoined
			}
			if request.Op == "product_count" {
				if request != (installeddevice.Request{Op: "product_count"}) {
					return trustprofile.ErrTrust
				}
				life, stop := context.WithTimeout(ctx, 4*time.Second)
				count, e := controller.Count(life)
				stop()
				if e != nil {
					e = output(map[string]string{"status": "product_unavailable"})
				} else {
					e = output(count)
				}
				if e != nil {
					return e
				}
				continue
			}
			life, stop := context.WithTimeout(ctx, 20*time.Second)
			reply, e := controller.Forward(life, request)
			stop()
			if e != nil {
				if e = output(map[string]string{"status": "request_rejected"}); e != nil {
					return e
				}
				continue
			}
			if e = output(json.RawMessage(reply)); e != nil {
				return e
			}
		}
	}
}

func main() {
	output := json.NewEncoder(os.Stdout).Encode
	if e := run(output); e != nil {
		if errors.Is(e, productmaintenance.ErrFaultJoined) {
			output(map[string]any{"status": "fault_closed", "joined": true, "fault": true})
		} else {
			output(map[string]string{"status": "failed"})
		}
		os.Exit(1)
	}
	output(map[string]string{"status": "closed"})
}
