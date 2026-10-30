package streamgate

import (
	"context"
	"errors"
	"net"
	"remoteapp.local/selfhost-poc/nodeproof"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

func TestDialWaitsForActualDelayedTCPListener(t *testing.T) {
	reserved, e := net.Listen("tcp4", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	addr := reserved.Addr().String()
	reserved.Close()
	first := make(chan struct{})
	listened := make(chan net.Listener, 1)
	var once sync.Once
	var calls atomic.Int32
	go func() { <-first; ln, _ := net.Listen("tcp4", addr); listened <- ln }()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	conn, e := dialUntilListening(ctx, func(c context.Context) (net.Conn, error) {
		calls.Add(1)
		v, e := (&net.Dialer{}).DialContext(c, "tcp4", addr)
		if errors.Is(e, syscall.ECONNREFUSED) {
			once.Do(func() { close(first) })
			return nil, nodeproof.ErrOverlayNotListening
		}
		return v, e
	}, func(context.Context) bool { return true })
	once.Do(func() { close(first) })
	ln := <-listened
	if ln != nil {
		defer ln.Close()
	}
	if conn != nil {
		defer conn.Close()
	}
	if e != nil || conn == nil || ln == nil || calls.Load() < 2 {
		t.Fatal("delayed listener failed", e)
	}
}
func TestDialDoesNotRetryUnknownRouteOrRevocation(t *testing.T) {
	for _, failure := range []error{nodeproof.ErrNoOverlayRoute, nodeproof.ErrOverlayDial, errors.New("unknown")} {
		calls := 0
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		conn, e := dialUntilListening(ctx, func(context.Context) (net.Conn, error) { calls++; return nil, failure }, func(context.Context) bool { return true })
		cancel()
		if e == nil || conn != nil || calls != 1 {
			t.Fatal("non-refusal retried")
		}
	}
	calls := 0
	_, e := dialUntilListening(context.Background(), func(context.Context) (net.Conn, error) { calls++; return nil, nodeproof.ErrOverlayNotListening }, func(context.Context) bool { return calls == 0 })
	if e == nil || calls != 1 {
		t.Fatal("revoked grant retried")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	_, e = dialUntilListening(ctx, func(context.Context) (net.Conn, error) { return nil, nodeproof.ErrOverlayNotListening }, func(context.Context) bool { return true })
	if e == nil || ctx.Err() == nil {
		t.Fatal("deadline not respected")
	}
}

func TestDialClosesUnverifiedSocketOnRevocationAndKeepsOriginalDeadline(t *testing.T) {
	a, b := net.Pipe()
	defer b.Close()
	active := true
	conn, e := dialUntilListening(context.Background(), func(context.Context) (net.Conn, error) { active = false; return a, nil }, func(context.Context) bool { return active })
	if conn != nil || e == nil {
		t.Fatal("revoked socket exposed")
	}
	b.SetReadDeadline(time.Now().Add(time.Second))
	if _, e = b.Read(make([]byte, 1)); e == nil {
		t.Fatal("unverified socket retained")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Millisecond)
	defer cancel()
	expected, _ := ctx.Deadline()
	calls := 0
	_, e = dialUntilListening(ctx, func(c context.Context) (net.Conn, error) {
		calls++
		d, ok := c.Deadline()
		if !ok || d != expected {
			t.Fatal("attempt renewed deadline")
		}
		return nil, nodeproof.ErrOverlayNotListening
	}, func(context.Context) bool { return true })
	if e == nil || calls < 2 || ctx.Err() == nil {
		t.Fatal("budget not shared")
	}
}
