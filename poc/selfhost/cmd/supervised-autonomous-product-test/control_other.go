//go:build !windows

package main

import (
	"context"
	"remoteapp.local/selfhost-poc/productmaintenance"
	"remoteapp.local/selfhost-poc/supervisedcontrol"
)

func startDesktopControl(ctx context.Context, b *productmaintenance.Controller) (string, func() error, error) {
	ln, err := supervisedcontrol.ListenPrivateUnix()
	if err != nil {
		return "", nil, err
	}
	life, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() { defer close(done); supervisedcontrol.Serve(life, ln, b) }()
	return ln.Address(), func() error { cancel(); ln.Close(); <-done; return ln.Cleanup() }, nil
}
