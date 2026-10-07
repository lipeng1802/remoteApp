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
	if len(os.Args) != 9 {
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
	if broker.RunDaemon(ctx, c, func() { fmt.Println("READY sole authority; network disabled") }) != nil {
		fmt.Println("FAIL authority unavailable")
		os.Exit(1)
	}
	fmt.Println("STOPPED authority")
}
