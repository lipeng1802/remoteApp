package streamgate

import (
	"context"
	"strconv"
	"strings"
	"time"

	"remoteapp.local/selfhost-poc/startuptrace"
	"tailscale.com/client/local"
)

type targetReceiveSnapshot struct {
	packets, filterDrops uint64
	valid                bool
}

func readTargetReceive(ctx context.Context, client *local.Client) targetReceiveSnapshot {
	check, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	raw, err := client.DaemonMetrics(check)
	if err != nil {
		return targetReceiveSnapshot{}
	}
	var snapshot targetReceiveSnapshot
	var packetsFound, dropsFound bool
	for _, line := range strings.Split(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		switch fields[0] {
		case "tstun_in_from_wg":
			snapshot.packets, err = strconv.ParseUint(fields[1], 10, 64)
			packetsFound = err == nil
		case "tstun_in_from_wg_drop_filter":
			snapshot.filterDrops, err = strconv.ParseUint(fields[1], 10, 64)
			dropsFound = err == nil
		}
	}
	snapshot.valid = packetsFound && dropsFound
	return snapshot
}

// Daemon counters are aggregate. An increase cannot identify a particular flow.
func observeTargetReceiveEnd(ctx context.Context, client *local.Client, baseline targetReceiveSnapshot) {
	if !startuptrace.Enabled(ctx) || client == nil {
		startuptrace.Emit(ctx, startuptrace.TargetAcceptEnded)
		return
	}
	final := readTargetReceive(context.WithoutCancel(ctx), client)
	stage := startuptrace.TargetReceiveUnchanged
	switch {
	case !baseline.valid || !final.valid || final.packets < baseline.packets || final.filterDrops < baseline.filterDrops:
		stage = startuptrace.TargetReceiveUnavailable
	case final.filterDrops > baseline.filterDrops:
		stage = startuptrace.TargetReceiveFilterDrop
	case final.packets > baseline.packets:
		stage = startuptrace.TargetReceiveObserved
	}
	startuptrace.Emit(ctx, stage)
}
