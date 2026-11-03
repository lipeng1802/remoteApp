package streamgate

import (
	"context"
	"net/netip"
	"time"

	"remoteapp.local/selfhost-poc/startuptrace"
	"tailscale.com/client/local"
	"tailscale.com/tailcfg"
)

// Called only after a TEST TCP timeout and a fresh signed-peer route match.
// Discovery never permits another dial or serves as TCP payload evidence.
func discoveryStage(ctx context.Context, client *local.Client, target netip.Addr, active func(context.Context) bool) startuptrace.Stage {
	probe, cancel := context.WithTimeout(ctx, 750*time.Millisecond)
	defer cancel()
	if !active(probe) {
		return startuptrace.ControllerDiscoveryUnavailable
	}
	reply, err := client.Ping(probe, target, tailcfg.PingDisco)
	if err != nil || probe.Err() != nil || reply == nil || reply.Err != "" ||
		reply.IP != target.String() || reply.NodeIP != target.String() || !active(probe) {
		return startuptrace.ControllerDiscoveryUnavailable
	}
	switch {
	case reply.Endpoint != "" && reply.PeerRelay == "" && reply.DERPRegionID == 0:
		return startuptrace.ControllerDiscoveryDirect
	case reply.Endpoint == "" && reply.PeerRelay != "" && reply.DERPRegionID == 0:
		return startuptrace.ControllerDiscoveryUDPRelay
	case reply.Endpoint == "" && reply.PeerRelay == "" && reply.DERPRegionID > 0:
		return startuptrace.ControllerDiscoveryDERP
	default:
		return startuptrace.ControllerDiscoveryUnavailable
	}
}
