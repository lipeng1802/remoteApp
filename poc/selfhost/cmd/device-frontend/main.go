package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"remoteapp.local/selfhost-poc/broker"
	"remoteapp.local/selfhost-poc/deviceservice"
	"strconv"
	"syscall"
)

func main() {
	if len(os.Args) != 8 && (len(os.Args) != 9 || os.Args[8] != "--private-network") {
		fmt.Println("FAIL frontend_arguments")
		os.Exit(1)
	}
	floor, e1 := strconv.ParseUint(os.Args[4], 10, 64)
	uid, e2 := strconv.ParseUint(os.Args[7], 10, 32)
	if e1 != nil || e2 != nil {
		fmt.Println("FAIL frontend_arguments")
		os.Exit(1)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	c := deviceservice.Config{Directory: os.Args[1], ReleaseRoot: os.Args[2], Service: os.Args[3], MinimumEpoch: floor, Listen: os.Args[5], Broker: &broker.Client{Socket: os.Args[6], ServerUID: uint32(uid)}}
	c.PrivateNetwork = len(os.Args) == 9
	if deviceservice.Run(ctx, c, func(net.Addr) {
		if c.PrivateNetwork {
			fmt.Println("READY unprivileged frontend; private network enabled")
		} else {
			fmt.Println("READY unprivileged frontend; network disabled")
		}
	}) != nil {
		fmt.Println("FAIL frontend unavailable")
		os.Exit(1)
	}
	fmt.Println("STOPPED frontend")
}
