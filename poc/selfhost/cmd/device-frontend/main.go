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

func networkMode(args []string) (private, public bool, err error) {
	if len(args) == 8 {
		return false, false, nil
	}
	if len(args) != 9 {
		return false, false, deviceservice.ErrService
	}
	switch args[8] {
	case "--private-network":
		return true, false, nil
	case "--public-network":
		return false, true, nil
	}
	return false, false, deviceservice.ErrService
}

func main() {
	private, public, err := networkMode(os.Args)
	if err != nil {
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
	c.PrivateNetwork, c.PublicNetwork = private, public
	if deviceservice.Run(ctx, c, func(net.Addr) {
		if c.PublicNetwork {
			fmt.Println("READY unprivileged frontend; explicit direct signed network mode")
		} else if c.PrivateNetwork {
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
