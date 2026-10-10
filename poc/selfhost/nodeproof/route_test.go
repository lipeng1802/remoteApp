package nodeproof

import (
	"net/netip"
	"strings"
	"testing"

	"tailscale.com/ipn/ipnstate"
	"tailscale.com/types/key"
	"tailscale.com/types/views"
)

func TestRouteStatusRequiresExactPeerMapAndHostRoute(t *testing.T) {
	node := "nodekey:" + strings.Repeat("a", 64)
	var public key.NodePublic
	if public.UnmarshalText([]byte(node)) != nil {
		t.Fatal("key fixture")
	}
	ip := netip.MustParseAddr("100.120.0.2")
	for _, bad := range []string{"", "stopped", "missing", "wrong_key", "wrong_ip", "old_map", "no_allowed", "broad_route", "error", "bad_node"} {
		allowed := views.SliceOf([]netip.Prefix{netip.PrefixFrom(ip, 32)})
		p := &ipnstate.PeerStatus{PublicKey: public, TailscaleIPs: []netip.Addr{ip}, AllowedIPs: &allowed, InNetworkMap: true}
		s := &ipnstate.Status{BackendState: "Running", Peer: map[key.NodePublic]*ipnstate.PeerStatus{public: p}}
		input := node
		switch bad {
		case "stopped":
			s.BackendState = "Stopped"
		case "missing":
			delete(s.Peer, public)
		case "wrong_key":
			p.PublicKey = key.NodePublic{}
		case "wrong_ip":
			p.TailscaleIPs = nil
		case "old_map":
			p.InNetworkMap = false
		case "no_allowed":
			p.AllowedIPs = nil
		case "broad_route":
			allowed = views.SliceOf([]netip.Prefix{netip.MustParsePrefix("100.120.0.0/24")})
		case "bad_node":
			input = "private detail"
		}
		r := routeStatus(s, bad == "error", input, ip)
		if r.QueryOK != (bad != "error" && bad != "bad_node") {
			t.Fatal("query diagnostic mismatch")
		}
		if r.Ready != (bad == "") {
			t.Fatalf("readiness mismatch: %s", bad)
		}
		// InEngine=false is not an ACL denial: idle peers may be trimmed.
		if bad == "" && r.InEngine {
			t.Fatal("fixture unexpectedly active")
		}
	}
}
