package nodeproof

import (
	"context"
	"net/netip"
	"time"

	"tailscale.com/client/local"
	"tailscale.com/ipn/ipnstate"
	"tailscale.com/types/key"
)

// RouteStatus is diagnostic readiness, NEVER authorization or payload proof.
// It exposes no peer keys, addresses, endpoints or raw LocalAPI errors.
type RouteStatus struct {
	Status         string `json:"status"`
	QueryOK        bool   `json:"query_ok"`
	Running        bool   `json:"running"`
	PeerKnown      bool   `json:"peer_known"`
	IPMatches      bool   `json:"ip_matches"`
	InNetworkMap   bool   `json:"in_network_map"`
	AllowedIP      bool   `json:"allowed_ip"`
	InEngine       bool   `json:"in_engine"`
	Ready          bool   `json:"ready"`
	SelfRelayKnown bool   `json:"self_relay_known"`
	PeerRelayKnown bool   `json:"peer_relay_known"`
	HandshakeSeen  bool   `json:"handshake_seen"`
	peerTxBytes    int64
	peerNode       key.NodePublic
	peerIP         netip.Addr
	peerDirectUDP  bool
	peerUDPRelay   bool
}

func ClientRouteStatus(ctx context.Context, lc *local.Client, node string, ip netip.Addr) RouteStatus {
	ctx, cancel := context.WithTimeout(ctx, 750*time.Millisecond)
	defer cancel()
	st, err := lc.Status(ctx)
	return routeStatus(st, err != nil || ctx.Err() != nil, node, ip)
}

func routeStatus(st *ipnstate.Status, failed bool, node string, ip netip.Addr) RouteStatus {
	r := RouteStatus{Status: "route_status"}
	var public key.NodePublic
	if failed || st == nil || public.UnmarshalText([]byte(node)) != nil || public.String() != node || !ip.Is4() {
		return r
	}
	r.QueryOK = true
	r.Running = st.BackendState == "Running"
	r.SelfRelayKnown = st.Self != nil && st.Self.Relay != ""
	p := st.Peer[public]
	if p == nil || p.PublicKey != public {
		return r
	}
	r.PeerKnown = true
	r.PeerRelayKnown = p.Relay != ""
	r.HandshakeSeen = !p.LastHandshake.IsZero()
	for _, address := range p.TailscaleIPs {
		r.IPMatches = r.IPMatches || address == ip
	}
	r.InNetworkMap, r.InEngine = p.InNetworkMap, p.InEngine
	if p.AllowedIPs != nil {
		for _, prefix := range p.AllowedIPs.All() {
			r.AllowedIP = r.AllowedIP || prefix == netip.PrefixFrom(ip, 32)
		}
	}
	if r.IPMatches && r.InEngine {
		r.peerTxBytes, r.peerNode, r.peerIP = p.TxBytes, public, ip
		r.peerDirectUDP, r.peerUDPRelay = p.CurAddr != "", p.PeerRelay != ""
	}
	r.Ready = r.Running && r.PeerKnown && r.IPMatches && r.InNetworkMap && r.AllowedIP
	return r
}

// PeerTransmitGrowth compares only observations of the same bound peer.
// The counter covers all WireGuard traffic to that peer, not a TCP flow.
func (r RouteStatus) PeerTransmitGrowth(before RouteStatus) (grew, known bool) {
	if !r.peerIP.IsValid() || r.peerIP != before.peerIP || r.peerNode != before.peerNode ||
		before.peerTxBytes < 0 || r.peerTxBytes < before.peerTxBytes {
		return false, false
	}
	return r.peerTxBytes > before.peerTxBytes, true
}

// PeerUDPSelection describes only the current SDK route selection snapshot.
func (r RouteStatus) PeerUDPSelection() (direct, relay bool) {
	return r.peerDirectUDP, r.peerUDPRelay
}
