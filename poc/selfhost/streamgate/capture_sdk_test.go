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
	"tailscale.com/tsnet"
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
