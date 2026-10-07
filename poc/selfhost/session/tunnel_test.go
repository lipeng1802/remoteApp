package session

import (
	"bytes"
	"context"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/invite"
)

func TestTunnelOpaqueBytesAndAuthorityClosure(t *testing.T) {
	f := setup(t)
	server, client := sockets(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	upstream, echo := net.Pipe()
	local, app := net.Pipe()
	defer app.Close()
	defer echo.Close()
	var opens atomic.Int64
	go func() { io.Copy(echo, echo) }()
	done := make(chan error, 2)
	go func() {
		done <- Tunnel(ctx, server, f.server, f.check, true, func(context.Context) (net.Conn, error) { opens.Add(1); return upstream, nil })
	}()
	go func() {
		done <- Tunnel(ctx, client, f.probe, f.check, false, func(context.Context) (net.Conn, error) { return local, nil })
	}()
	payload := bytes.Repeat([]byte{0x16, 0x03, 0x03, 0x7f}, 17000)
	app.SetDeadline(time.Now().Add(4 * time.Second))
	written := make(chan error, 1)
	go func() { _, e := app.Write(payload); written <- e }()
	received := make([]byte, len(payload))
	if _, e := io.ReadFull(app, received); e != nil || !bytes.Equal(received, payload) {
		t.Fatal("opaque payload changed")
	}
	if e := <-written; e != nil || opens.Load() != 1 {
		t.Fatal("upstream admission")
	}
	f.now.Add(301)
	for i := 0; i < 2; i++ {
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("tunnel workers did not close")
		}
	}
}

func TestTunnelDenialNeverOpensProductUpstream(t *testing.T) {
	f := setup(t)
	server, _ := sockets(t)
	var opened atomic.Int64
	check := func(context.Context, invite.Grant, string, string) bool { return false }
	err := Tunnel(context.Background(), server, f.server, check, true, func(context.Context) (net.Conn, error) { opened.Add(1); return nil, nil })
	if err != ErrDenied || opened.Load() != 0 {
		t.Fatal("denied peer reached upstream")
	}
}

func TestFixedPayloadProofCannotEnterProductTunnel(t *testing.T) {
	f := setup(t)
	server, client := sockets(t)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	var opened atomic.Int64
	done := make(chan error, 1)
	go func() {
		done <- Tunnel(ctx, server, f.server, f.check, true, func(context.Context) (net.Conn, error) { opened.Add(1); return nil, nil })
	}()
	if Probe(ctx, client, f.probe, nil) == nil {
		t.Fatal("fixed-payload proof entered TLS domain")
	}
	if e := <-done; e == nil || opened.Load() != 0 {
		t.Fatal("cross-domain peer opened upstream")
	}
}
