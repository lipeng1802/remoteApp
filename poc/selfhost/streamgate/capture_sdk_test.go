//go:build remoteapp_test_capture

package streamgate

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	_ "tailscale.com/feature/capture"
	"tailscale.com/net/netns"
	"tailscale.com/tailcfg"
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
	if _, err = second.Up(ctx); err != nil {
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
	done := make(chan bool, 1)
	go func() { done <- readSYNHeader(stream) }()
	defer func() { stream.Close(); <-done }()
	peer, err := second.LocalClient()
	if err != nil {
		t.Fatal("peer client unavailable")
	}
	if _, err = peer.Ping(ctx, status.TailscaleIPs[0], tailcfg.PingTSMP); err != nil {
		t.Fatal("local encrypted IP ping unavailable")
	}
	select {
	case valid := <-done:
		done <- valid
		if !valid {
			t.Fatal("invalid SDK header")
		}
	case <-ctx.Done():
		t.Fatal("first packet did not flush SDK header")
	}
}
