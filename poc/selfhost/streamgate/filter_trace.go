package streamgate

import (
	"context"
	"net/netip"
	"slices"
	"time"

	"remoteapp.local/selfhost-poc/startuptrace"
	"tailscale.com/client/local"
	"tailscale.com/types/ipproto"
	"tailscale.com/wgengine/filter"
)

// observeTargetFilter records a read-only snapshot before the first accept.
func observeTargetFilter(ctx context.Context, client *local.Client, source, target netip.Addr) {
	if !startuptrace.Enabled(ctx) {
		return
	}
	check, cancel := context.WithTimeout(ctx, 750*time.Millisecond)
	defer cancel()
	rules, err := client.DebugPacketFilterRules(check)
	if err != nil {
		startuptrace.Emit(ctx, startuptrace.TargetFilterUnavailable)
		return
	}
	matches, err := filter.MatchesFromFilterRules(rules)
	if err != nil {
		startuptrace.Emit(ctx, startuptrace.TargetFilterUnavailable)
		return
	}
	for _, match := range matches {
		if !match.SrcsContains(source) || !slices.Contains(match.IPProto.AsSlice(), ipproto.TCP) {
			continue
		}
		for _, destination := range match.Dsts {
			if destination.Net.Contains(target) && destination.Ports.Contains(47476) {
				startuptrace.Emit(ctx, startuptrace.TargetFilterMatch)
				return
			}
		}
	}
	startuptrace.Emit(ctx, startuptrace.TargetFilterNoIPMatch)
}
