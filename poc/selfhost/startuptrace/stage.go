// Package startuptrace exposes only fixed TEST startup stages, never raw errors.
package startuptrace

import (
	"context"
	"fmt"
	"os"
)

type Stage string

const (
	TargetSYNOpenFailed                    Stage = "target_syn_open_failed"
	TargetSYNHeaderUnavailable             Stage = "target_syn_header_unavailable"
	TargetSYNNotObserved                   Stage = "target_syn_not_observed"
	TargetSYNStreamUnavailable             Stage = "target_syn_stream_unavailable"
	TargetSYNObserved                      Stage = "target_syn_observed"
	TargetSYNReplyObserved                 Stage = "target_syn_reply_observed"
	ControllerSYNObserved                  Stage = "deadline_controller_syn_observed"
	ControllerSYNReplyObserved             Stage = "deadline_controller_syn_reply_observed"
	ControllerDiscoveryDERPDataObserved    Stage = "deadline_route_derp_discovery_tsmp_observed"
	ControllerDiscoveryDERPDataUnconfirmed Stage = "deadline_route_derp_discovery_tsmp_unconfirmed"
	ControllerDiscoveryUnavailable         Stage = "deadline_route_discovery_unavailable"
	ControllerDiscoveryDirect              Stage = "deadline_route_discovery_direct"
	ControllerDiscoveryUDPRelay            Stage = "deadline_route_discovery_udp_relay"
	ControllerDiscoveryDERP                Stage = "deadline_route_discovery_derp"

	ControllerPeerSendDirectSelected   Stage = "deadline_route_peer_send_direct_selected"
	ControllerPeerSendUDPRelaySelected Stage = "deadline_route_peer_send_udp_relay_selected"
	ControllerPeerSendObserved         Stage = "deadline_route_peer_send_observed"
	ControllerPeerSendUnchanged        Stage = "deadline_route_peer_send_unchanged"
	ControllerSendUnavailable          Stage = "deadline_route_engine_present_send_unavailable"
	ControllerSendFilterDrop           Stage = "deadline_route_engine_present_send_filter_drop"
	ControllerSendObserved             Stage = "deadline_route_engine_present_send_observed"
	ControllerSendUnchanged            Stage = "deadline_route_engine_present_send_unchanged"
)

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
	ControllerRouteWait    Stage = "controller_route_wait"
	ControllerRouteDenied  Stage = "controller_route_denied"
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

const (
	OverlayBadUpgrade       Stage = "overlay_bad_upgrade"
	OverlayPermissionDenied Stage = "overlay_permission_denied"
	OverlayResolveFailed    Stage = "overlay_resolve_failed"
	OverlayConnectRefused   Stage = "overlay_connect_refused"
	OverlayConnectTimeout   Stage = "overlay_connect_timeout"
	OverlayConnectFailed    Stage = "overlay_connect_failed"
)

const (
	OverlayRequestDeadline Stage = "overlay_request_deadline"
	OverlayRequestCanceled Stage = "overlay_request_canceled"
	OverlayRequestEOF      Stage = "overlay_request_eof"
	OverlayRequestRefused  Stage = "overlay_request_refused"
	OverlayRequestReset    Stage = "overlay_request_reset"
	OverlayRequestTimeout  Stage = "overlay_request_timeout"
)

const (
	SecretTargetConfig          Stage = "secret_target_config"
	SecretTargetAuthorize       Stage = "secret_target_authorize"
	SecretTargetHello           Stage = "secret_target_hello"
	SecretTargetNonce           Stage = "secret_target_nonce"
	SecretTargetSign            Stage = "secret_target_sign"
	SecretTargetWelcome         Stage = "secret_target_welcome"
	SecretTargetProof           Stage = "secret_target_proof"
	SecretTargetReauthorize     Stage = "secret_target_reauthorize"
	SecretTargetSend            Stage = "secret_target_send"
	SecretControllerConfig      Stage = "secret_controller_config"
	SecretControllerAuthorize   Stage = "secret_controller_authorize"
	SecretControllerHello       Stage = "secret_controller_hello"
	SecretControllerWelcome     Stage = "secret_controller_welcome"
	SecretControllerVerify      Stage = "secret_controller_verify"
	SecretControllerSign        Stage = "secret_controller_sign"
	SecretControllerProof       Stage = "secret_controller_proof"
	SecretControllerReauthorize Stage = "secret_controller_reauthorize"
	SecretControllerReceive     Stage = "secret_controller_receive"
	SecretControllerCanonical   Stage = "secret_controller_canonical"
)

const (
	DeadlineRouteQueryFailed   Stage = "deadline_route_query_failed"
	DeadlineRoutePeerMissing   Stage = "deadline_route_peer_missing"
	DeadlineRouteNotReady      Stage = "deadline_route_not_ready"
	DeadlineRouteEnginePending Stage = "deadline_route_engine_pending"
	DeadlineRouteEnginePresent Stage = "deadline_route_engine_present"
)

const (
	DeadlineSelfRelayUnknown Stage = "deadline_ready_self_relay_unknown"
	DeadlinePeerRelayUnknown Stage = "deadline_ready_peer_relay_unknown"
	DeadlineHandshakeUnseen  Stage = "deadline_ready_handshake_unseen"
)

const (
	TargetReceiveUnavailable Stage = "target_receive_unavailable"
	TargetReceiveFilterDrop  Stage = "target_receive_filter_drop"
	TargetReceiveObserved    Stage = "target_receive_observed"
	TargetReceiveUnchanged   Stage = "target_receive_unchanged"
	TargetFilterUnavailable  Stage = "target_filter_unavailable"
	TargetFilterMatch        Stage = "target_filter_match"
	TargetFilterNoIPMatch    Stage = "target_filter_no_ip_match"
)

const (
	TargetAcceptEnded Stage = "target_accept_ended"
	TargetTCPAccepted Stage = "target_tcp_accepted"
)

type Report struct {
	Status string `json:"status"`
	Stage  string `json:"stage"`
}

func Valid(s string) bool {
	switch Stage(s) {
	case TargetSYNOpenFailed, TargetSYNHeaderUnavailable, TargetSYNNotObserved, TargetSYNStreamUnavailable, TargetSYNObserved, TargetSYNReplyObserved, ControllerSYNObserved, ControllerSYNReplyObserved, ControllerDiscoveryDERPDataObserved, ControllerDiscoveryDERPDataUnconfirmed, ControllerDiscoveryUnavailable, ControllerDiscoveryDirect, ControllerDiscoveryUDPRelay, ControllerDiscoveryDERP, ControllerPeerSendDirectSelected, ControllerPeerSendUDPRelaySelected, ControllerPeerSendObserved, ControllerPeerSendUnchanged, ControllerSendUnavailable, ControllerSendFilterDrop, ControllerSendObserved, ControllerSendUnchanged, TargetReceiveUnavailable, TargetReceiveFilterDrop, TargetReceiveObserved, TargetReceiveUnchanged, TargetAcceptEnded, TargetTCPAccepted, TargetFilterUnavailable, TargetFilterMatch, TargetFilterNoIPMatch, DeadlineSelfRelayUnknown, DeadlinePeerRelayUnknown, DeadlineHandshakeUnseen, ControllerRouteWait, ControllerRouteDenied, DeadlineRouteQueryFailed, DeadlineRoutePeerMissing, DeadlineRouteNotReady, DeadlineRouteEnginePending, DeadlineRouteEnginePresent, SecretTargetConfig, SecretTargetAuthorize, SecretTargetHello, SecretTargetNonce, SecretTargetSign, SecretTargetWelcome, SecretTargetProof, SecretTargetReauthorize, SecretTargetSend, SecretControllerConfig, SecretControllerAuthorize, SecretControllerHello, SecretControllerWelcome, SecretControllerVerify, SecretControllerSign, SecretControllerProof, SecretControllerReauthorize, SecretControllerReceive, SecretControllerCanonical, OverlayRequestDeadline, OverlayRequestCanceled, OverlayRequestEOF, OverlayRequestRefused, OverlayRequestReset, OverlayRequestTimeout, OverlayBadUpgrade, OverlayPermissionDenied, OverlayResolveFailed, OverlayConnectRefused, OverlayConnectTimeout, OverlayConnectFailed, TargetGateDenied, TargetLocalFailed, TargetSelfDenied, TargetListenFailed, TargetListening, TargetAcceptDenied, TargetPeerDenied, ControllerGateDenied, ControllerLocalFailed, ControllerUpgrade, ControllerPeer, ControllerPeerDenied, OverlayRequestFailed, OverlayUpgradeRejected, OverlayRouteMissing, OverlayResponseInvalid, OverlayUpgraded, TargetTLS, TargetListen, TargetSecret, TargetServe, ControllerListen, ControllerDial, ControllerSecret, ControllerTLS, ControllerAccept, ControllerBridge:
		return true
	}
	return false
}

type key struct{}

// Context is trusted compiled wiring; no peer or environment can enable it.
func Context(ctx context.Context, test bool) context.Context {
	return context.WithValue(ctx, key{}, test)
}
func Enabled(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	enabled, _ := ctx.Value(key{}).(bool)
	return enabled
}
func Emit(ctx context.Context, s Stage) {
	if ctx == nil {
		return
	}
	if enabled, _ := ctx.Value(key{}).(bool); enabled && Valid(string(s)) {
		fmt.Fprintln(os.Stderr, "PRODUCT_STAGE "+string(s))
	}
}
