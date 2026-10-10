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
	Status       string `json:"status"`
	QueryOK      bool   `json:"query_ok"`
	Running      bool   `json:"running"`
	PeerKnown    bool   `json:"peer_known"`
	IPMatches    bool   `json:"ip_matches"`
	InNetworkMap bool   `json:"in_network_map"`
	AllowedIP    bool   `json:"allowed_ip"`
	InEngine     bool   `json:"in_engine"`
	Ready        bool   `json:"ready"`
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
	p := st.Peer[public]
	if p == nil || p.PublicKey != public {
		return r
	}
	r.PeerKnown = true
	for _, address := range p.TailscaleIPs {
		r.IPMatches = r.IPMatches || address == ip
	}
	r.InNetworkMap, r.InEngine = p.InNetworkMap, p.InEngine
	if p.AllowedIPs != nil {
		for _, prefix := range p.AllowedIPs.All() {
			r.AllowedIP = r.AllowedIP || prefix == netip.PrefixFrom(ip, 32)
		}
	}
	r.Ready = r.Running && r.PeerKnown && r.IPMatches && r.InNetworkMap && r.AllowedIP
	return r
}
