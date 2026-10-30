package supervisedcontrol

import (
	"context"
	"encoding/json"
	"net"
	"remoteapp.local/selfhost-poc/clientruntime"
	"remoteapp.local/selfhost-poc/installeddevice"
	"remoteapp.local/selfhost-poc/privatepipe"
	"sync/atomic"
	"testing"
	"time"
)

type fixture struct{ calls atomic.Int32 }

func (b *fixture) Forward(ctx context.Context, r installeddevice.Request) (json.RawMessage, error) {
	b.calls.Add(1)
	return json.RawMessage(`{"status":"device_result","passed":true}`), nil
}
func (b *fixture) Count(context.Context) (clientruntime.ProductCount, error) {
	b.calls.Add(1)
	return clientruntime.ProductCount{Status: "product_count", Pongs: 3}, nil
}

func exercise(t *testing.T, ln net.Listener, dial func() (net.Conn, error)) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	b := &fixture{}
	done := make(chan struct{})
	go func() { defer close(done); Serve(ctx, ln, b) }()
	defer func() {
		cancel()
		ln.Close()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("server failed to join")
		}
	}()
	for _, item := range []struct {
		raw, want string
		calls     int32
	}{
		{`{"Op":"stop"}`, "request_rejected", 0},
		{`{"Op":"stop_fault"}`, "request_rejected", 0},
		{`{"Op":"stop_fault","VaultScope":"foreign"}`, "request_rejected", 0},
		{`{"Op":"quiesce_session","VaultScope":"foreign"}`, "request_rejected", 0},
		{`{"Op":"maintenance_prepare"}`, "request_rejected", 0},
		{`{"Op":"product_count","VaultScope":"override"}`, "request_rejected", 0},
		{`{"Op":"device","Device":{"action":"status"},"VaultScope":"override"}`, "request_rejected", 0},
		{`{"Op":"revoke_session","Device":{"grant_id":"foreign"}}`, "request_rejected", 0},
		{`{"Op":"device","Device":{"action":"status"}}`, "device_result", 1},
		{`{"Op":"product_count"}`, "product_count", 2},
		{`{"Op":"revoke_session"}`, "device_result", 3},
		{`{"Op":"quiesce_session"}`, "device_result", 4},
	} {
		c, e := dial()
		if e != nil {
			t.Fatal(e)
		}
		c.SetDeadline(time.Now().Add(3 * time.Second))
		if _, e = c.Write([]byte(item.raw + "\n")); e != nil {
			t.Fatal(e)
		}
		var result struct {
			Status string `json:"status"`
			Passed bool   `json:"passed"`
			Pongs  int    `json:"pongs"`
		}
		// Replies may include the other aggregate counters; inspect only status.
		var raw json.RawMessage
		if e = privatepipe.NewReader(c).Read(&raw); e != nil {
			t.Fatal(e)
		}
		if json.Unmarshal(raw, &result) != nil || result.Status != item.want {
			t.Fatalf("unexpected reply status %s", result.Status)
		}
		c.Close()
		if b.calls.Load() != item.calls {
			t.Fatal("rejected command reached owner")
		}
	}
	// A client stalled before sending a frame must not keep the desktop alive.
	c, e := dial()
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("stalled connection retained server")
	}
}
func TestBusinessRoutingAndJoinedCancellation(t *testing.T) {
	ln, e := net.Listen("tcp4", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	exercise(t, ln, func() (net.Conn, error) { return net.Dial("tcp4", ln.Addr().String()) })
}
