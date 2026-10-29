package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"remoteapp.local/selfhost-poc/desktopcontrol"
	"remoteapp.local/selfhost-poc/productmaintenance"
	"remoteapp.local/selfhost-poc/supervisedcontrol"
)

func startDesktopControl(ctx context.Context, b *productmaintenance.Controller) (string, func() error, error) {
	var nonce [16]byte
	if _, e := rand.Read(nonce[:]); e != nil {
		return "", nil, e
	}
	name := `\\.\pipe\remoteapp-session-` + hex.EncodeToString(nonce[:])
	ln, e := desktopcontrol.Listen(name)
	if e != nil {
		return "", nil, e
	}
	life, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() { defer close(done); supervisedcontrol.Serve(life, ln, b) }()
	return name, func() error { cancel(); ln.Close(); <-done; return nil }, nil
}
