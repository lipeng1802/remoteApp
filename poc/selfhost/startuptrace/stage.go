// Package startuptrace exposes only fixed TEST startup stages, never raw errors.
package startuptrace

import (
	"context"
	"fmt"
	"os"
)

type Stage string

const (
	TargetTLS        Stage = "target_tls"
	TargetListen     Stage = "target_listen"
	TargetSecret     Stage = "target_secret"
	TargetServe      Stage = "target_serve"
	ControllerListen Stage = "controller_listen"
	ControllerDial   Stage = "controller_dial"
	ControllerSecret Stage = "controller_secret"
	ControllerTLS    Stage = "controller_tls"
	ControllerAccept Stage = "controller_accept"
	ControllerBridge Stage = "controller_bridge"
)

const (
	TargetGateDenied       Stage = "target_gate_denied"
	TargetLocalFailed      Stage = "target_local_failed"
	TargetSelfDenied       Stage = "target_self_denied"
	TargetListenFailed     Stage = "target_listen_failed"
	TargetListening        Stage = "target_listening"
	TargetAcceptDenied     Stage = "target_accept_denied"
	TargetPeerDenied       Stage = "target_peer_denied"
	ControllerGateDenied   Stage = "controller_gate_denied"
	ControllerLocalFailed  Stage = "controller_local_failed"
	ControllerUpgrade      Stage = "controller_upgrade"
	ControllerPeer         Stage = "controller_peer"
	ControllerPeerDenied   Stage = "controller_peer_denied"
	OverlayRequestFailed   Stage = "overlay_request_failed"
	OverlayUpgradeRejected Stage = "overlay_upgrade_rejected"
	OverlayRouteMissing    Stage = "overlay_route_missing"
	OverlayResponseInvalid Stage = "overlay_response_invalid"
	OverlayUpgraded        Stage = "overlay_upgraded"
)

type Report struct {
	Status string `json:"status"`
	Stage  string `json:"stage"`
}

func Valid(s string) bool {
	switch Stage(s) {
	case TargetGateDenied, TargetLocalFailed, TargetSelfDenied, TargetListenFailed, TargetListening, TargetAcceptDenied, TargetPeerDenied, ControllerGateDenied, ControllerLocalFailed, ControllerUpgrade, ControllerPeer, ControllerPeerDenied, OverlayRequestFailed, OverlayUpgradeRejected, OverlayRouteMissing, OverlayResponseInvalid, OverlayUpgraded, TargetTLS, TargetListen, TargetSecret, TargetServe, ControllerListen, ControllerDial, ControllerSecret, ControllerTLS, ControllerAccept, ControllerBridge:
		return true
	}
	return false
}

type key struct{}

// Context is trusted compiled wiring; no peer or environment can enable it.
func Context(ctx context.Context, test bool) context.Context {
	return context.WithValue(ctx, key{}, test)
}
func Emit(ctx context.Context, s Stage) {
	if ctx == nil {
		return
	}
	if enabled, _ := ctx.Value(key{}).(bool); enabled && Valid(string(s)) {
		fmt.Fprintln(os.Stderr, "PRODUCT_STAGE "+string(s))
	}
}
