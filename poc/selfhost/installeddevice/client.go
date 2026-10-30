// Package installeddevice owns device API requests under an installed-profile
// lease. It never accepts endpoint/root/CA/issuer or an exported device key.
package installeddevice

import (
	"context"
	"encoding/hex"
	"time"

	"remoteapp.local/selfhost-poc/cleanupwitness"
	"remoteapp.local/selfhost-poc/clientowner"
	"remoteapp.local/selfhost-poc/clientruntime"
	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/trustinstall"
	"remoteapp.local/selfhost-poc/trustprofile"
)

type Request struct {
	Op, VaultScope, ExpectedPublic string
	Device                         invite.Body
	Network                        invite.NetworkRequestBody `json:",omitzero"`
}

func canonical(value string, size int) bool {
	b, err := hex.DecodeString(value)
	return err == nil && len(b) == size && hex.EncodeToString(b) == value
}

func ValidOpen(r Request) bool {
	return r.Op == "open" && canonical(r.VaultScope, 16) && canonical(r.ExpectedPublic, 32) && r.Device == (invite.Body{}) && r.Network == (invite.NetworkRequestBody{})
}

// ValidAutonomousOpen deliberately carries no identity selector. The explicit
// installation binding, not the parent or peer, chooses vault and node scope.
func ValidAutonomousOpen(r Request) bool {
	return r == (Request{Op: "open"})
}

func validCommand(r Request) bool {
	if r.VaultScope != "" || r.ExpectedPublic != "" {
		return false
	}
	if r.Op == "stop" {
		return r.Device == (invite.Body{}) && r.Network == (invite.NetworkRequestBody{})
	}
	if r.Op == "network" {
		return r.Device == (invite.Body{}) && (r.Network.Action == "grant_clock" || r.Network.Action == "state") && r.Network.Grant != (invite.Grant{}) && r.Network == (invite.NetworkRequestBody{Action: r.Network.Action, Grant: r.Network.Grant})
	}
	if r.Network != (invite.NetworkRequestBody{}) {
		return false
	}
	if r.Op != "device" || r.Device.Nonce != "" || r.Device.IssuedAt != 0 {
		return false
	}
	switch r.Device.Action {
	case "register", "invite", "claim", "incoming", "approve", "status", "revoke":
		return true
	}
	return false
}

func validProvision(r Request) bool {
	return r.Op == "provision" && r.VaultScope == "" && r.ExpectedPublic == "" && r.Device == (invite.Body{}) && r.Network.Grant != (invite.Grant{}) && r.Network == (invite.NetworkRequestBody{Grant: r.Network.Grant})
}

// ValidSupervisedCommand reuses the product's strict ordinary request policy.
// Lifecycle/open/identity-selection commands belong to the trusted supervisor,
// never its business forwarding path. The product still validates every frame.
func ValidSupervisedCommand(r Request) bool {
	return r.Op != "stop" && (validCommand(r) || validProvision(r))
}

// Separate supervisor-side operation. The native product never receives it.
func ValidSupervisedRevocation(r Request) bool { return r == (Request{Op: "revoke_session"}) }
func ValidSupervisedQuiesce(r Request) bool    { return r == (Request{Op: "quiesce_session"}) }

// Runner is started only by clientowner.Open/Start, which supplies the durable
// current profile and monotonic lifetime. The output is a trusted private pipe,
// not a log or public peer interface; device results can contain invite/grant.
func Runner(release trustinstall.Release, open clientruntime.SignerFactory, commands <-chan Request, output func(any) error) clientowner.Runner {
	return runner(release, open, commands, output, nil, false, false, nil)
}

// EnrollmentRunner adds a trusted internal node owner, not an IPC-selected path
// or credential. The compiled entry intentionally remains Runner until native
// node-storage ownership and recovery provisioning are installed and verified.
func EnrollmentRunner(release trustinstall.Release, open clientruntime.SignerFactory, commands <-chan Request, output func(any) error, factory EnrollmentFactory) clientowner.Runner {
	return runner(release, open, commands, output, factory, false, false, nil)
}

// Trusted opt-in composition only; ordinary installed entry stays disabled.
func BoundEnrollmentRunner(release trustinstall.Release, open clientruntime.SignerFactory, commands <-chan Request, output func(any) error, factory EnrollmentFactory) clientowner.Runner {
	return runner(release, open, commands, output, factory, true, false, nil)
}

// WatchedEnrollmentRunner additionally requires a first issuer-signed ACTIVE
// state after bind, then owns a terminal continuous watch. It is the opt-in
// bridge toward payload sessions; no binding, node or secret leaves the child.
func WatchedEnrollmentRunner(release trustinstall.Release, open clientruntime.SignerFactory, commands <-chan Request, output func(any) error, factory EnrollmentFactory) clientowner.Runner {
	return runner(release, open, commands, output, factory, true, true, nil)
}

// WatchedEnrollmentRunnerWithSession is the first slice-4 lifecycle bridge.
// work is trusted compiled wiring, never supplied by a parent frame. It starts
// only after the same enrollment watch reaches ACTIVE and is joined before the
// watch, proof, node and signer are released. ProductSession may opt into the
// fixed TLS/HMAC bridge. The ordinary installed-device-client remains disabled;
// a separately compiled installed-autonomous-product entry opts in explicitly.
func WatchedEnrollmentRunnerWithSession(release trustinstall.Release, open clientruntime.SignerFactory, commands <-chan Request, output func(any) error, factory EnrollmentFactory, work SessionWork) clientowner.Runner {
	return runner(release, open, commands, output, factory, true, true, work)
}

// WatchedSessionViaTestTunnel requires explicit compiled TEST opt-in. Requests
// cannot choose a tunnel, root or endpoint. Production wiring stays unchanged.
func WatchedSessionViaTestTunnel(release trustinstall.Release, open clientruntime.SignerFactory, commands <-chan Request, output func(any) error, factory EnrollmentFactory, work SessionWork) clientowner.Runner {
	if !release.Specification().Test {
		return func(context.Context, trustprofile.Profile) error { return trustprofile.ErrTrust }
	}
	return runner(release, open, commands, output, factory, true, true, work, true)
}

func runner(release trustinstall.Release, open clientruntime.SignerFactory, commands <-chan Request, output func(any) error, factory EnrollmentFactory, binding, watched bool, work SessionWork, testTunnel ...bool) clientowner.Runner {
	return func(ctx context.Context, profile trustprofile.Profile) (result error) {
		w := cleanupwitness.From(ctx)
		w.Begin(cleanupwitness.Runner)
		defer w.Complete(cleanupwitness.Runner, nil)
		if ctx.Err() != nil || open == nil || output == nil || commands == nil {
			return trustprofile.ErrTrust
		}
		w.Begin(cleanupwitness.Signer)
		signer, err := open()
		if err != nil || signer == nil {
			return trustprofile.ErrTrust
		}
		joined, ok := signer.(interface{ CloseJoined() error })
		if !ok {
			signer.Close()
			return trustprofile.ErrTrust
		}
		defer func() {
			err := joined.CloseJoined()
			w.Complete(cleanupwitness.Signer, err)
			if err != nil {
				result = trustprofile.ErrTrust
			}
		}()
		anchor := release.Anchor()
		createClient := invite.NewProfileNetworkClient
		if len(testTunnel) == 1 && testTunnel[0] {
			if !release.Specification().Test {
				return trustprofile.ErrTrust
			}
			createClient = invite.NewTestTunnelNetworkClient
		}
		client, err := createClient(profile, anchor.Root[:], anchor.Service, anchor.MinimumEpoch, signer)
		if err != nil {
			return trustprofile.ErrTrust
		}
		defer client.Close()
		gate := newAuthorizationGate(profile, signer)
		var enrolled enrollment
		defer func() {
			if err := enrolled.closeJoined(); err != nil {
				result = trustprofile.ErrTrust
			}
		}()
		var expires <-chan time.Time
		var watchDone <-chan struct{}
		var sessionDone <-chan struct{}
		var expiry *time.Timer
		defer func() {
			if expiry != nil {
				expiry.Stop()
			}
		}()
		if err := output(map[string]string{"status": "ready"}); err != nil {
			return err
		}
		w.Arm(release.Specification().Test)
		for {
			select {
			case <-sessionDone:
				// Product/session exit while the owner is still authorized is a
				// terminal fault. There is no automatic restart or new scope.
				return trustprofile.ErrTrust
			case <-watchDone:
				if ctx.Err() == context.Canceled {
					return nil
				}
				return trustprofile.ErrTrust
			case <-expires:
				return trustprofile.ErrTrust
			case <-ctx.Done():
				if ctx.Err() == context.Canceled {
					return nil
				}
				return trustprofile.ErrTrust
			case request, exists := <-commands:
				if !exists {
					return nil
				}
				provision := validProvision(request)
				if (!validCommand(request) && !(provision && factory != nil)) || (enrolled.attempted && request.Op != "stop") {
					return trustprofile.ErrTrust
				}
				if request.Op == "stop" {
					return nil
				}
				if provision {
					reply, err := enrolled.start(ctx, gate, client, factory, request.Network.Grant)
					if err == nil && binding {
						reply, err = enrolled.bind(ctx, gate, client)
					}
					if err == nil && watched {
						reply, err = enrolled.startWatch(ctx, gate, client)
						if err == nil {
							watchDone = enrolled.watch.Done()
						}
					}
					if outErr := output(reply); outErr != nil {
						return outErr
					}
					if err != nil {
						return trustprofile.ErrTrust
					}
					if watched && work != nil {
						if err = enrolled.prepareSessionPeers(ctx, gate, client); err != nil {
							return trustprofile.ErrTrust
						}
						if err = enrolled.startSession(ctx, work); err != nil {
							return trustprofile.ErrTrust
						}
						sessionDone = enrolled.session.Done()
					}
					expiry = time.NewTimer(time.Unix(gate.grant.Claims.ExpiresAt, 0).Sub(gate.clock()))
					expires = expiry.C
					continue
				}
				if request.Op == "network" {
					reply, err := gate.request(ctx, client, request.Network)
					if outErr := output(reply); outErr != nil {
						return outErr
					}
					// Authorization/transport failure retires this owner. No cached
					// permission or automatic retry/recovery for a later session.
					if err != nil {
						return trustprofile.ErrTrust
					}
					continue
				}
				call, cancel := context.WithTimeout(ctx, 15*time.Second)
				value, err := client.DeviceRequest(call, request.Device)
				cancel()
				reply := map[string]any{"status": "device_result", "passed": err == nil}
				if err == nil {
					reply["device"] = value
				}
				if err := output(reply); err != nil {
					return err
				}
			}
		}
	}
}
