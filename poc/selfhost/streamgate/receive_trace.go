package streamgate

import (
	"context"
	"strconv"
	"strings"
	"time"

	"remoteapp.local/selfhost-poc/startuptrace"
	"tailscale.com/client/local"
)

type overlayCounterSnapshot struct {
	packets, filterDrops uint64
	valid                bool
}

func readOverlayCounters(ctx context.Context, client *local.Client, packetMetric, dropMetric string) overlayCounterSnapshot {
	check, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	raw, err := client.DaemonMetrics(check)
	if err != nil {
		return overlayCounterSnapshot{}
	}
	var snapshot overlayCounterSnapshot
	var packetsFound, dropsFound bool
	for _, line := range strings.Split(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		switch fields[0] {
		case packetMetric:
			snapshot.packets, err = strconv.ParseUint(fields[1], 10, 64)
			packetsFound = err == nil
		case dropMetric:
			snapshot.filterDrops, err = strconv.ParseUint(fields[1], 10, 64)
			dropsFound = err == nil
		}
	}
	snapshot.valid = packetsFound && dropsFound
	return snapshot
}

// Daemon counters are aggregate. An increase cannot identify a particular flow.
func observeTargetReceiveEnd(ctx context.Context, client *local.Client, baseline overlayCounterSnapshot) {
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

func readTargetReceive(ctx context.Context, client *local.Client) overlayCounterSnapshot {
	return readOverlayCounters(ctx, client, "tstun_in_from_wg", "tstun_in_from_wg_drop_filter")
}

func readControllerSend(ctx context.Context, client *local.Client) overlayCounterSnapshot {
	return readOverlayCounters(ctx, client, "tstun_out_to_wg", "tstun_out_to_wg_drop_filter")
}

func controllerSendStage(baseline, final overlayCounterSnapshot) startuptrace.Stage {
	switch {
	case !baseline.valid || !final.valid || final.packets < baseline.packets || final.filterDrops < baseline.filterDrops:
		return startuptrace.ControllerSendUnavailable
	case final.filterDrops > baseline.filterDrops:
		return startuptrace.ControllerSendFilterDrop
	case final.packets > baseline.packets:
		return startuptrace.ControllerSendObserved
	default:
		return startuptrace.ControllerSendUnchanged
	}
}
