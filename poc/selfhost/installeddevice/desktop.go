package installeddevice

import (
	"context"
	"strings"

	"remoteapp.local/selfhost-poc/clientowner"
	"remoteapp.local/selfhost-poc/clientruntime"
	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/trustinstall"
	"remoteapp.local/selfhost-poc/trustprofile"
)

// CoordinatedDesktopSession is explicit trusted TEST composition. The local
// scope is selected before opening the signer; the parent cannot choose it in
// an open/business frame. GUI consent/pre-arm must precede product startup.
func CoordinatedDesktopSession(release trustinstall.Release, open clientruntime.SignerFactory, commands <-chan Request, output func(any) error, factory EnrollmentFactory, work SessionWork, expected invite.DesktopPermission) clientowner.Runner {
	return desktopRunner(release, open, commands, output, factory, work, expected, 0)
}

// CoordinatedDesktopSessionViaTestTunnel keeps the same compiled tunnel and
// controller-start coordination as the fixed TEST path, with an exact scope.
func CoordinatedDesktopSessionViaTestTunnel(release trustinstall.Release, open clientruntime.SignerFactory, commands <-chan Request, output func(any) error, factory EnrollmentFactory, work SessionWork, expected invite.DesktopPermission, port uint16) clientowner.Runner {
	if port < 1024 {
		return func(context.Context, trustprofile.Profile) error { return trustprofile.ErrTrust }
	}
	return desktopRunner(release, open, commands, output, factory, work, expected, port)
}

func desktopRunner(release trustinstall.Release, open clientruntime.SignerFactory, commands <-chan Request, output func(any) error, factory EnrollmentFactory, work SessionWork, expected invite.DesktopPermission, port uint16) clientowner.Runner {
	policy, err := invite.DesktopGrantPolicy(expected)
	if err != nil || !release.Specification().Test || factory == nil || work == nil {
		return func(context.Context, trustprofile.Profile) error { return trustprofile.ErrTrust }
	}
	return runner(release, open, commands, output, factory, true, true, work, runnerOptions{testTunnel: port, coordinated: true, policy: policy})
}

// ValidDesktopSupervisedCommand validates structure only. Signatures, exact
// observed grant, role, time, current state and consent remain separate gates.
func ValidDesktopSupervisedCommand(r Request, expected invite.DesktopPermission) bool {
	p, err := invite.DesktopGrantPolicy(expected)
	return err == nil && r.Op != "stop" && (validCommandForPolicy(r, p) || validProvision(r) && r.Network.Grant.Claims.Permission == p.Permission())
}

func validCommandForPolicy(r Request, p invite.GrantPolicy) bool {
	if !p.IsDesktop() {
		return validCommand(r)
	}
	if r.VaultScope != "" || r.ExpectedPublic != "" {
		return false
	}
	if r.Op == "stop" {
		return r == (Request{Op: "stop"})
	}
	if r.Op == "network" {
		return validCommand(r) && r.Network.Grant.Claims.Permission == p.Permission()
	}
	if r.Op != "device" || r.Network != (invite.NetworkRequestBody{}) {
		return false
	}
	b := r.Device
	allowed := invite.Body{Action: b.Action}
	scope := invite.DesktopPermission(p.Permission())
	switch b.Action {
	case "register", "incoming":
	case "invite":
		allowed.Permission = scope
	case "claim":
		allowed.Permission, allowed.TargetCode, allowed.InviteID, allowed.Code = scope, b.TargetCode, b.InviteID, b.Code
		if b.TargetCode == "" || b.Code == "" {
			return false
		}
	case "approve":
		allowed.RequestID, allowed.Allow = b.RequestID, b.Allow
		if b.RequestID == "" {
			return false
		}
		if b.Allow || b.Permission != "" {
			allowed.Permission = scope
		}
	case "status":
		allowed.RequestID = b.RequestID
		if b.RequestID == "" {
			return false
		}
	case "revoke":
		allowed.GrantID = b.GrantID
		if b.GrantID == "" {
			return false
		}
	default:
		return false
	}
	return b == allowed
}

// ValidDesktopDeviceResult checks a successful reply's exact shape/scope and
// request correlation. It is not a signature, revocation or process-join proof.
func ValidDesktopDeviceResult(expected invite.DesktopPermission, b invite.Body, r invite.Result) bool {
	p, err := invite.DesktopGrantPolicy(expected)
	if err != nil || !validCommandForPolicy(Request{Op: "device", Device: b}, p) {
		return false
	}
	allowed := invite.Result{Status: r.Status}
	switch b.Action {
	case "register":
		if r.Status != "registered" || !canonical(r.DeviceID, 32) || !canonical(r.DeviceCode, 6) {
			return false
		}
		allowed.DeviceID, allowed.DeviceCode = r.DeviceID, r.DeviceCode
	case "invite":
		if r.Status != "inviting" || !canonical(r.InviteID, 16) || len(r.Code) != 8 || strings.Trim(r.Code, "0123456789") != "" {
			return false
		}
		allowed.InviteID, allowed.Code, allowed.Permission = r.InviteID, r.Code, expected
	case "claim", "incoming":
		if b.Action == "incoming" && r == (invite.Result{Status: "idle"}) {
			return true
		}
		if r.Status != "pending" || !canonical(r.RequestID, 16) {
			return false
		}
		allowed.RequestID, allowed.Permission = r.RequestID, expected
		if b.Action == "incoming" {
			if !canonical(r.ControllerID, 32) {
				return false
			}
			allowed.ControllerID = r.ControllerID
		}
	case "approve", "status":
		if r.RequestID != b.RequestID {
			return false
		}
		allowed.RequestID, allowed.Permission = b.RequestID, expected
		if b.Action == "approve" && ((b.Allow && r.Status != "approved") || (!b.Allow && r.Status != "denied")) {
			return false
		}
		switch r.Status {
		case "approved":
			if r.Grant == nil || r.Grant.Claims.Permission != p.Permission() {
				return false
			}
			allowed.Grant = r.Grant
		case "pending", "denied", "revoked", "expired":
		default:
			return false
		}
	case "revoke":
		if r.Status != "revoked" {
			return false
		}
	default:
		return false
	}
	return r == allowed
}
