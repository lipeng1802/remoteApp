//go:build linux || darwin

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"remoteapp.local/selfhost-poc/broker"
	"strconv"
	"syscall"
)

func main() {
	check := len(os.Args) == 11 && os.Args[9] == "--check-network-config"
	if len(os.Args) != 9 && (len(os.Args) != 11 || (os.Args[9] != "--network-config" && !check)) {
		fmt.Println("FAIL authority_arguments")
		os.Exit(1)
	}
	floor, e1 := strconv.ParseUint(os.Args[4], 10, 64)
	peer, e2 := strconv.ParseUint(os.Args[7], 10, 32)
	group, e3 := strconv.ParseUint(os.Args[8], 10, 32)
	if e1 != nil || e2 != nil || e3 != nil {
		fmt.Println("FAIL authority_arguments")
		os.Exit(1)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	c := broker.DaemonConfig{Profile: os.Args[1], ReleaseRoot: os.Args[2], Service: os.Args[3], MinimumEpoch: floor, StateDirectory: os.Args[5], Socket: os.Args[6], PeerUID: uint32(peer), GroupID: uint32(group)}
	c.Report = func(stage string) { fmt.Println("DIAGNOSTIC network_" + stage) }
	if len(os.Args) == 11 {
		c.NetworkConfig = os.Args[10]
	}
	if check {
		if broker.CheckNetworkPreparation(c) != nil {
			fmt.Println("FAIL network_preparation")
			os.Exit(1)
		}
		fmt.Println("CHECK PASSED: idle files and signed profile only; NOT live provider, policy mode, issuer match or network acceptance")
		return
	}
	if broker.RunDaemon(ctx, c, func() {
		if c.NetworkConfig == "" {
			fmt.Println("READY sole authority; network disabled")
		} else {
			fmt.Println("READY sole authority; supervised network enabled on private broker")
		}
	}) != nil {
		fmt.Println("FAIL authority unavailable")
		os.Exit(1)
	}
	fmt.Println("STOPPED authority")
}
