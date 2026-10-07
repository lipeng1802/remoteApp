package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"remoteapp.local/selfhost-poc/deviceservice"
	"strconv"
	"syscall"
)

func main() {
	// All arguments are paths or public installation trust; no private keys.
	if len(os.Args) != 6 {
		fmt.Println("FAIL device_service_arguments")
		os.Exit(1)
	}
	floor, e := strconv.ParseUint(os.Args[4], 10, 64)
	if e != nil {
		fmt.Println("FAIL device_service_arguments")
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	e = deviceservice.Run(ctx, deviceservice.Config{Directory: os.Args[1], ReleaseRoot: os.Args[2], Service: os.Args[3], MinimumEpoch: floor, Listen: os.Args[5]}, func(net.Addr) { fmt.Println("READY device_only_service; network provisioning disabled") })
	if e != nil {
		fmt.Println("FAIL device_service_unavailable")
		os.Exit(1)
	}
	fmt.Println("STOPPED device_service")
}
