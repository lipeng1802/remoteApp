// TEST launcher with trusted private business IPC and independent maintenance.
// It selects identity from native binding; no argv/root/key/endpoint selectors.
package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/netip"
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
var embeddedTestTunnel, embeddedMaintenanceAddress, embeddedRemovalDigest string

func maintenanceAddress(encoded string) (string, error) {
	if encoded == "" {
		return "127.0.0.1:0", nil
	}
	raw, e := base64.RawURLEncoding.DecodeString(encoded)
	address, e2 := netip.ParseAddrPort(string(raw))
	if e != nil || e2 != nil || base64.RawURLEncoding.EncodeToString(raw) != encoded || address.String() != string(raw) || address.Addr().String() != "127.0.0.1" || address.Port() < 1024 {
		return "", trustprofile.ErrTrust
	}
	return string(raw), nil
}

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
	var remover *productmaintenance.ArmedRemoval
	var controller *productmaintenance.Controller
	defer func() {
		if remover != nil {
			if err := remover.Close(context.Background()); err != nil {
				result = err
			}
		}
		if controller != nil {
			if err := controller.Release(context.Background()); err != nil {
				result = err
			}
		}
	}()
	if embeddedRemovalDigest != "" {
		if embeddedMaintenanceAddress == "" {
			return trustprofile.ErrTrust
		}
		arm, stop := context.WithTimeout(ctx, 20*time.Second)
		remover, e = productmaintenance.StartArmedRemoval(arm, processowner.Spec{Executable: filepath.Join(dir, "bound-authenticated-removal-test"+suffix), SHA256: embeddedRemovalDigest})
		stop()
		if e != nil {
			return e
		}
	}
	controller, e = productmaintenance.StartTest(ctx, r, processowner.Spec{Executable: string(product), SHA256: embeddedProductDigest, CaptureProductStage: true}, factory, bf, func(record autonomousbinding.Record) (productmaintenance.Signer, error) {
		s, e := clientruntime.NativeSigner(filepath.Join(dir, "identity-vault"+suffix), embeddedSignerDigest, record.VaultScope, record.PublicKey)()
		if e != nil {
			return nil, e
		}
		return s.(*privatepipe.Signer), nil
	})
	if e != nil {
		return e
	}
	port, portErr := trustinstall.ParseTestTunnelPort(embeddedTestTunnel)
	if portErr != nil {
		return trustprofile.ErrTrust
	}
	if e = controller.ConfigureRevocation(ctx, port); e != nil {
		return e
	}
	address, e := maintenanceAddress(embeddedMaintenanceAddress)
	if e != nil {
		return e
	}
	l, e := net.Listen("tcp4", address)
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
	if remover != nil {
		metadata["removal_protocol"] = 1
		metadata["uninstaller_process_id"] = remover.PID()
	}
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
				// A provisioned maintenance endpoint may have joined/deleted
				// the payload, but the original independent revoke signer is
				// still needed. Keep the same supervisor/business endpoint.
				// Do not inject an unsolicited reply into its parent pipe.
				if controller.RetainAfterMaintenance(ctx) {
					continue
				}
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
			if request == (installeddevice.Request{Op: "remove_program"}) {
				if remover == nil || controller.AllowProgramRemoval(ctx) != nil || remover.Begin() != nil {
					if e = output(map[string]string{"status": "request_rejected"}); e != nil {
						return e
					}
				} else if e = output(map[string]string{"status": "removal_pending"}); e != nil {
					return e
				}
				continue
			}
			if request == (installeddevice.Request{Op: "program_removal_status"}) {
				if remover == nil {
					e = output(map[string]string{"status": "request_rejected"})
				} else {
					e = output(remover.Status())
				}
				if e != nil {
					return e
				}
				continue
			}
			if request == (installeddevice.Request{Op: "stop"}) {
				life, stop := context.WithTimeout(ctx, 10*time.Second)
				err := controller.AllowRetainedStop(life)
				stop()
				if err != nil {
					if err = output(map[string]string{"status": "request_rejected"}); err != nil {
						return err
					}
					continue
				}
				return nil
			}
			if request == (installeddevice.Request{Op: "stop_local_fault"}) {
				life, stop := context.WithTimeout(ctx, 20*time.Second)
				e := controller.CancelLocalFault(life)
				stop()
				if e != nil {
					if e = output(map[string]string{"status": "local_cancel_rejected"}); e != nil {
						return e
					}
					continue
				}
				if e = output(map[string]any{"status": "local_cancel_ready", "joined": true, "fault": true, "remote_revocation_verified": false}); e != nil {
					return e
				}
				return productmaintenance.ErrLocalCancelJoined
			}
			if request == (installeddevice.Request{Op: "stop_fault"}) {
				life, stop := context.WithTimeout(ctx, 20*time.Second)
				e := controller.ReleaseFault(life)
				stop()
				if e != nil {
					fmt.Fprintln(os.Stderr, "FAULT_RELEASE_REASON "+productmaintenance.FaultReleaseReason(e))
					rejection := map[string]string{"status": "fault_stop_rejected"}
					if runtime.GOOS == "windows" {
						rejection["reason"] = productmaintenance.FaultReleaseReason(e)
					}
					if e = output(rejection); e != nil {
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
			if request.Op == "product_stage" {
				if request != (installeddevice.Request{Op: "product_stage"}) {
					return trustprofile.ErrTrust
				}
				report, err := controller.ProductStage(ctx)
				if err != nil {
					err = output(map[string]string{"status": "product_unavailable"})
				} else {
					err = output(report)
				}
				if err != nil {
					return err
				}
				continue
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
				if e = output(map[string]string{"status": "request_rejected", "reason": productmaintenance.ForwardRejectionReason(e)}); e != nil {
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
		if errors.Is(e, productmaintenance.ErrLocalCancelJoined) {
			output(map[string]any{"status": "local_cancel_closed", "joined": true, "fault": true, "remote_revocation_verified": false})
		} else if errors.Is(e, productmaintenance.ErrFaultJoined) {
			output(map[string]any{"status": "fault_closed", "joined": true, "fault": true})
		} else {
			output(map[string]string{"status": "failed"})
		}
		os.Exit(1)
	}
	output(map[string]string{"status": "closed"})
}
