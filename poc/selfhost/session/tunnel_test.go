package session

import (
	"bytes"
	"context"
	"io"
	"net"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/startuptrace"
)

func TestTunnelOpaqueBytesAndAuthorityClosure(t *testing.T) {
	f := portableSetup(t)
	testTunnelWithCheck(t, f, f.portableCheck)
}
func testTunnel(t *testing.T, f *fixture) {
	testTunnelWithCheck(t, f, f.check)
}
func testTunnelWithCheck(t *testing.T, f *fixture, check Check) {
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
		done <- Tunnel(ctx, server, f.server, check, true, func(context.Context) (net.Conn, error) { opens.Add(1); return upstream, nil })
	}()
	go func() {
		done <- Tunnel(ctx, client, f.probe, check, false, func(context.Context) (net.Conn, error) { return local, nil })
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
	f := portableSetup(t)
	server, _ := sockets(t)
	var opened atomic.Int64
	check := func(context.Context, invite.Grant, string, string) bool { return false }
	err := Tunnel(context.Background(), server, f.server, check, true, func(context.Context) (net.Conn, error) { opened.Add(1); return nil, nil })
	if err != ErrDenied || opened.Load() != 0 {
		t.Fatal("denied peer reached upstream")
	}
}

func TestFixedPayloadProofCannotEnterProductTunnel(t *testing.T) {
	f := portableSetup(t)
	server, client := sockets(t)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	var opened atomic.Int64
	done := make(chan error, 1)
	go func() {
		done <- Tunnel(ctx, server, f.server, f.portableCheck, true, func(context.Context) (net.Conn, error) { opened.Add(1); return nil, nil })
	}()
	if Probe(ctx, client, f.probe, nil) == nil {
		t.Fatal("fixed-payload proof entered TLS domain")
	}
	if e := <-done; e == nil || opened.Load() != 0 {
		t.Fatal("cross-domain peer opened upstream")
	}
}

func TestTunnelTerminalTrace(t *testing.T) {
	for _, mode := range []string{"authorization", "local", "overlay", "cancel", "disabled"} {
		t.Run(mode, func(t *testing.T) {
			f := portableSetup(t)
			server, client := sockets(t)
			ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
			defer cancel()
			local, targetApp := net.Pipe()
			upstream, controllerApp := net.Pipe()
			defer targetApp.Close()
			defer controllerApp.Close()

			reader, writer, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			original := os.Stderr
			os.Stderr = writer
			defer func() { os.Stderr = original; writer.Close(); reader.Close() }()

			done := make(chan error, 2)
			var denied atomic.Bool
			check := func(ctx context.Context, g invite.Grant, target, controller string) bool {
				return !denied.Load() && f.portableCheck(ctx, g, target, controller)
			}
			go func() {
				traceCtx := startuptrace.Context(ctx, mode != "disabled")
				done <- Tunnel(traceCtx, server, f.server, check, true, func(context.Context) (net.Conn, error) { return local, nil })
			}()
			go func() {
				done <- Tunnel(ctx, client, f.probe, f.portableCheck, false, func(context.Context) (net.Conn, error) { return upstream, nil })
			}()
			controllerApp.SetWriteDeadline(time.Now().Add(2 * time.Second))
			targetApp.SetReadDeadline(time.Now().Add(2 * time.Second))
			sent := make(chan error, 1)
			go func() { _, err := controllerApp.Write([]byte{42}); sent <- err }()
			var got [1]byte
			_, readErr := io.ReadFull(targetApp, got[:])
			writeErr := <-sent
			if readErr != nil || writeErr != nil || got[0] != 42 {
				cancel()
				<-done
				<-done
				t.Fatal("tunnel did not forward data", readErr, writeErr)
			}

			var expected startuptrace.Stage
			switch mode {
			case "authorization":
				expected = startuptrace.LifecycleTunnelAuthorizationEnded
				denied.Store(true)
			case "local":
				expected = startuptrace.LifecycleTunnelLocalToOverlayEnded
				targetApp.Close()
			case "overlay":
				expected = startuptrace.LifecycleTunnelOverlayToLocalEnded
				controllerApp.Close()
			case "cancel":
				cancel()
			case "disabled":
				targetApp.Close()
			}
			<-done
			<-done
			writer.Close()
			raw, err := io.ReadAll(reader)
			want := ""
			if expected != "" {
				want = "PRODUCT_STAGE " + string(expected) + "\n"
			}
			if err != nil || string(raw) != want {
				t.Fatalf("trace = %q, %v, want %q", raw, err, want)
			}
		})
	}
}
