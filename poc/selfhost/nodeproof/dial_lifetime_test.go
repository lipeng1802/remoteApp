package nodeproof

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"tailscale.com/client/local"
	"testing"
	"time"
)

func TestUpgradedOverlayOutlivesEstablishmentContext(t *testing.T) {
	done := make(chan struct{})
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(done)
		w.Header().Set("Connection", "upgrade")
		w.Header().Set("Upgrade", "ts-dial")
		w.WriteHeader(101)
		conn, rw, e := w.(http.Hijacker).Hijack()
		if e != nil {
			return
		}
		defer conn.Close()
		if rw.Flush() != nil {
			return
		}
		buf := make([]byte, 1)
		if _, e = io.ReadFull(rw, buf); e == nil {
			conn.Write(buf)
		}
	}))
	defer s.Close()
	lc := &local.Client{Dial: func(ctx context.Context, network, addr string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "tcp", strings.TrimPrefix(s.URL, "http://"))
	}}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	conn, e := DialOnlyOverlay(ctx, lc, "100.120.0.2", 47476)
	if e != nil {
		t.Fatal("upgrade rejected")
	}
	defer conn.Close()
	cancel()
	time.Sleep(30 * time.Millisecond)
	conn.SetDeadline(time.Now().Add(time.Second))
	if _, e = conn.Write([]byte("P")); e != nil {
		t.Fatal("establishment cancellation closed upgraded stream")
	}
	b := make([]byte, 1)
	if _, e = io.ReadFull(conn, b); e != nil || b[0] != 'P' {
		t.Fatal("upgraded stream lost after establishment context")
	}
	conn.Close()
	<-done
}
