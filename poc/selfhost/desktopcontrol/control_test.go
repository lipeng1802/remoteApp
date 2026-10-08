package desktopcontrol

import (
	"context"
	"encoding/json"
	"net"
	"remoteapp.local/selfhost-poc/clientruntime"
	"remoteapp.local/selfhost-poc/privatepipe"
	"remoteapp.local/selfhost-poc/trustprofile"
	"sync/atomic"
	"testing"
	"time"
)

type backend struct {
	probes            atomic.Int32
	stopping, release chan struct{}
}

func (b *backend) Probe(context.Context) error { b.probes.Add(1); return nil }
func (b *backend) Count(context.Context) (clientruntime.ProductCount, error) {
	return clientruntime.ProductCount{Status: "product_count", Pongs: 3}, nil
}
func (b *backend) Start(ctx context.Context, c clientruntime.Config, out func(any) error) error {
	if !c.Server {
		return privatepipe.ErrProtocol
	}
	if e := out(map[string]string{"status": "product_ready", "Key": "private-test-key"}); e != nil {
		return e
	}
	return out(map[string]any{"status": "ready", "port": 47476})
}
func (b *backend) Stop(ctx context.Context) error {
	close(b.stopping)
	select {
	case <-b.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (b *backend) Upgrade(context.Context, trustprofile.Profile) error {
	return privatepipe.ErrProtocol
}
func serveTest(t *testing.T, b Backend) (net.Conn, context.CancelFunc, <-chan struct{}) {
	t.Helper()
	ln, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); Serve(ctx, ln, "scope", "public", b) }()
	c, e := net.Dial("tcp", ln.Addr().String())
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		c.Close()
		cancel()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("server did not join")
		}
	})
	return c, cancel, done
}
func status(t *testing.T, r *privatepipe.Reader, want string) {
	t.Helper()
	var reply struct {
		Status, Key string
		Port        int
		PublicKey   string `json:"public_key"`
	}
	if e := r.Read(&reply); e != nil || reply.Status != want {
		t.Fatalf("reply %v %s, want %s", e, reply.Status, want)
	}
}
func TestPrivateStartAndStopAckOnlyAfterJoin(t *testing.T) {
	b := &backend{stopping: make(chan struct{}), release: make(chan struct{})}
	c, _, _ := serveTest(t, b)
	w := json.NewEncoder(c)
	r := privatepipe.NewReader(c)
	w.Encode(map[string]string{"Scope": "scope", "Expected": "public"})
	status(t, r, "ready")
	w.Encode(Request{Op: "probe"})
	status(t, r, "probe_passed")
	w.Encode(Request{Op: "start", Config: clientruntime.Config{Server: true}})
	status(t, r, "product_ready")
	status(t, r, "ready")
	for i := 0; i < 2; i++ {
		w.Encode(Request{Op: "count"})
		var count clientruntime.ProductCount
		if e := r.Read(&count); e != nil || count.Status != "product_count" || count.Pongs != 3 {
			t.Fatal("count relay", e)
		}
	}
	w.Encode(Request{Op: "stop_session"})
	<-b.stopping
	c.SetReadDeadline(time.Now().Add(20 * time.Millisecond))
	var raw json.RawMessage
	if json.NewDecoder(c).Decode(&raw) == nil {
		t.Fatal("ack before join")
	}
	c.SetReadDeadline(time.Time{})
	close(b.release)
	status(t, r, "session_closed")
}
func TestRejectUnknownFieldsAndWrongIdentity(t *testing.T) {
	for _, hello := range []string{`{"Scope":"wrong","Expected":"public"}`, `{"Scope":"scope","Expected":"public","Create":true}`} {
		b := &backend{}
		c, _, _ := serveTest(t, b)
		c.Write([]byte(hello + "\n"))
		status(t, privatepipe.NewReader(c), "failed")
		if b.probes.Load() != 0 {
			t.Fatal("backend called")
		}
	}
	b := &backend{}
	c, _, _ := serveTest(t, b)
	w := json.NewEncoder(c)
	r := privatepipe.NewReader(c)
	w.Encode(map[string]string{"Scope": "scope", "Expected": "public"})
	status(t, r, "ready")
	c.Write([]byte("{\"Op\":\"probe\",\"Executable\":\"untrusted\"}\n"))
	var raw json.RawMessage
	if r.Read(&raw) == nil {
		t.Fatal("unknown field accepted")
	}
}
func TestCancellationJoinsIdleConnections(t *testing.T) {
	b := &backend{}
	c, cancel, done := serveTest(t, b)
	json.NewEncoder(c).Encode(map[string]string{"Scope": "scope", "Expected": "public"})
	status(t, privatepipe.NewReader(c), "ready")
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("idle workers not joined")
	}
}

func TestCountRejectsRuntimeAndProfileOverrides(t *testing.T) {
	for _, req := range []Request{
		{Op: "count", Config: clientruntime.Config{Server: true}},
		{Op: "count", Profile: trustprofile.Profile{Signature: "untrusted"}},
	} {
		c, _, _ := serveTest(t, &backend{})
		w := json.NewEncoder(c)
		r := privatepipe.NewReader(c)
		w.Encode(map[string]string{"Scope": "scope", "Expected": "public"})
		status(t, r, "ready")
		w.Encode(req)
		status(t, r, "failed")
	}
}
