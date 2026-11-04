//go:build remoteapp_test_capture

package streamgate

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	_ "tailscale.com/feature/capture"
	"tailscale.com/net/netns"

	"tailscale.com/tsnet"
	"tailscale.com/tstest/integration"
	"tailscale.com/tstest/integration/testcontrol"
	"tailscale.com/types/logger"
)

// Offline SDK check: capture must close and join even without packet traffic.
func TestSDKCaptureWithoutTrafficClosesReader(t *testing.T) {
	control := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "offline", http.StatusServiceUnavailable)
	}))
	defer control.Close()
	s := &tsnet.Server{Dir: t.TempDir(), ControlURL: control.URL, Ephemeral: true, Logf: func(string, ...any) {}}
	defer s.Close()
	client, err := s.LocalClient()
	if err != nil {
		t.Fatal("local client unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	stream, err := client.StreamDebugCapture(ctx)
	if err != nil {
		t.Fatal("capture endpoint unavailable")
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		io.Copy(io.Discard, io.LimitReader(stream, 24))
	}()
	stream.Close()
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("capture reader did not join")
	}
}

// Exercises the actual SDK first-packet flush with isolated local nodes.
func TestSDKCaptureFirstPacketHeader(t *testing.T) {
	netns.SetEnabled(false)
	defer netns.SetEnabled(true)
	control := &testcontrol.Server{DERPMap: integration.RunDERPAndSTUN(t, logger.Discard, "127.0.0.1"), Logf: logger.Discard}
	httpServer := httptest.NewServer(control)
	defer httpServer.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	first := &tsnet.Server{Dir: t.TempDir(), Hostname: "capture-first", Ephemeral: true, ControlURL: httpServer.URL, Logf: logger.Discard}
	defer first.Close()
	second := &tsnet.Server{Dir: t.TempDir(), Hostname: "capture-second", Ephemeral: true, ControlURL: httpServer.URL, Logf: logger.Discard}
	defer second.Close()
	status, err := first.Up(ctx)
	if err != nil {
		t.Fatal("first local node unavailable")
	}
	secondStatus, err := second.Up(ctx)
	if err != nil {
		t.Fatal("second local node unavailable")
	}
	client, err := first.LocalClient()
	if err != nil {
		t.Fatal("capture client unavailable")
	}
	stream, err := client.StreamDebugCapture(ctx)
	if err != nil {
		t.Fatal("capture stream unavailable")
	}
	defer stream.Close()
	listener, err := first.Listen("tcp4", net.JoinHostPort(status.TailscaleIPs[0].String(), "47476"))
	if err != nil {
		t.Fatal("local SDK listener unavailable")
	}
	defer listener.Close()
	done := make(chan synEvidence, 1)
	go func() {
		done <- readSYNEvidence(stream, secondStatus.TailscaleIPs[0], status.TailscaleIPs[0], true, nil)
	}()
	defer func() { stream.Close(); <-done }()
	// The first accepted connection is sufficient; the test sends no payload.
	accepted := make(chan struct{})
	go func() {
		defer close(accepted)
		conn, err := listener.Accept()
		if err == nil {
			conn.Close()
		}
	}()
	defer func() { listener.Close(); <-accepted }()
	conn, err := second.Dial(ctx, "tcp4", net.JoinHostPort(status.TailscaleIPs[0].String(), "47476"))
	if err != nil {
		t.Fatal("local SDK TCP unavailable")
	}
	conn.Close()
	select {
	case evidence := <-done:
		done <- evidence
		if !evidence.syn || !evidence.reply {
			t.Fatal("SDK TCP handshake not observed")
		}
	case <-ctx.Done():
		t.Fatal("TCP handshake did not flush SDK capture")
	}
}
