// Privileged server-local lifecycle utility, never an HTTP endpoint.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"remoteapp.local/selfhost-poc/headscale"
	"remoteapp.local/selfhost-poc/verifier"
	"syscall"
	"time"
)

func run() error {
	if len(os.Args) != 3 {
		return verifier.ErrVerifier
	}
	mode, dir := os.Args[1], os.Args[2]
	d := &headscale.Driver{Run: headscale.LocalRunner()}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if mode == "retire" {
		if e := verifier.Retire(ctx, dir, d); e != nil {
			return e
		}
		fmt.Println("RETIRED scoped verifier resources")
		return nil
	}
	if mode != "create" && mode != "resume" {
		return verifier.ErrVerifier
	}
	r, e := verifier.Open(ctx, dir, mode == "create", d, verifier.NewOverlay)
	if e != nil {
		return e
	}
	fmt.Println("READY persistent verifier only; no payload ACL or API activation")
	select {
	case <-ctx.Done():
	case <-r.Done():
	}
	stop, c := context.WithTimeout(context.Background(), 10*time.Second)
	defer c()
	if e = r.Stop(stop); e != nil {
		return e
	}
	fmt.Println("STOPPED verifier; identity retained for explicit resume")
	return nil
}
func main() {
	if run() != nil {
		fmt.Println("FAIL verifier unavailable; journal preserved, no automatic remint")
		os.Exit(1)
	}
}
