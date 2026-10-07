//go:build linux || darwin

package broker

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"net"
	"os"
	"path/filepath"
	"remoteapp.local/selfhost-poc/invite"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type counter struct{ calls atomic.Int32 }

func (c *counter) Execute(context.Context, Request) Reply { c.calls.Add(1); return Reply{Status: 403} }
func socket(t *testing.T) string {
	t.Helper()
	d, e := os.MkdirTemp("", "rb-")
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { os.RemoveAll(d) })
	d, e = filepath.EvalSymlinks(d)
	if e != nil {
		t.Fatal(e)
	}
	return filepath.Join(d, "authority.sock")
}
func start(t *testing.T, c Config) (context.CancelFunc, <-chan error) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	ready := make(chan struct{}, 1)
	go func() { done <- Serve(ctx, c, func() { ready <- struct{}{} }) }()
	select {
	case <-ready:
	case e := <-done:
		cancel()
		t.Fatal(e)
	case <-time.After(3 * time.Second):
		cancel()
		t.Fatal("startup")
	}
	return cancel, done
}
func stop(t *testing.T, cancel context.CancelFunc, done <-chan error) {
	t.Helper()
	cancel()
	select {
	case e := <-done:
		if e != nil {
			t.Fatal(e)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("not joined")
	}
}
func TestBrokerKernelPeerAndStrictFrames(t *testing.T) {
	path := socket(t)
	execute := &counter{}
	cfg := Config{path, uint32(os.Geteuid()), uint32(os.Getegid()), execute}
	cancel, done := start(t, cfg)
	defer cancel()
	client := Client{path, uint32(os.Geteuid())}
	r, e := client.Call(context.Background(), Request{Operation: "forbidden"})
	if e != nil || r.Status != 403 || execute.calls.Load() != 1 {
		t.Fatal("peer transport", e)
	}
	wrong := client
	wrong.ServerUID++
	if _, e = wrong.Call(context.Background(), Request{Operation: "device"}); e == nil {
		t.Fatal("wrong server uid")
	}
	for _, frame := range []string{`{"operation":"device","Operation":"network"}` + "\n", `{"operation":"device","unknown":"secret"}` + "\n", `{"operation":"device"}` + "\n{}\n", strings.Repeat("x", 8200) + "\n"} {
		c, e := net.DialUnix("unix", nil, &net.UnixAddr{Name: path, Net: "unix"})
		if e != nil {
			t.Fatal(e)
		}
		c.SetDeadline(time.Now().Add(time.Second))
		c.Write([]byte(frame))
		c.CloseWrite()
		var b [128]byte
		n, _ := c.Read(b[:])
		c.Close()
		if n != 0 {
			t.Fatal("invalid frame replied")
		}
	}
	if execute.calls.Load() != 1 {
		t.Fatal("malformed reached authority")
	}
	stop(t, cancel, done)
	if _, e = os.Lstat(path); !os.IsNotExist(e) {
		t.Fatal("socket retained")
	}
	cfg.PeerUID++
	cancel, done = start(t, cfg)
	if _, e = client.Call(context.Background(), Request{Operation: "device"}); e == nil {
		t.Fatal("unauthorized peer uid")
	}
	stop(t, cancel, done)
	if execute.calls.Load() != 1 {
		t.Fatal("unauthorized reached authority")
	}
}
func TestBrokerSignedAuthorityReplayAndNetworkClosed(t *testing.T) {
	path := socket(t)
	s, e := invite.Open(filepath.Join(filepath.Dir(path), "state"), time.Now)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	a := &Authority{Store: s}
	cancel, done := start(t, Config{path, uint32(os.Geteuid()), uint32(os.Getegid()), a})
	defer cancel()
	_, key, _ := ed25519.GenerateKey(rand.Reader)
	nonce := make([]byte, 16)
	rand.Read(nonce)
	env := invite.Sign(key, invite.Body{Action: "register", Nonce: hex.EncodeToString(nonce), IssuedAt: time.Now().Unix()})
	client := Client{path, uint32(os.Geteuid())}
	r, e := client.Call(context.Background(), Request{Operation: "device", Device: &env})
	if e != nil || r.Status != 200 || r.Device.DeviceID == "" {
		t.Fatal("register", e)
	}
	pub, e := s.DevicePublicKey(r.Device.DeviceID)
	if e != nil || hex.EncodeToString(pub) != env.PublicKey {
		t.Fatal("key binding")
	}
	if r, e = client.Call(context.Background(), Request{Operation: "device", Device: &env}); e != nil || r.Status != 403 {
		t.Fatal("replay")
	}
	if r, e = client.Call(context.Background(), Request{Operation: "network", Network: &invite.NetworkRequest{}}); e != nil || r.Status != 503 {
		t.Fatal("network not closed")
	}
	if r, e = client.Call(context.Background(), Request{Operation: "device", Device: &env, Network: &invite.NetworkRequest{}}); e != nil || r.Status != 403 {
		t.Fatal("mixed operation")
	}
	stop(t, cancel, done)
	ctx, c := context.WithCancel(context.Background())
	c()
	if _, e = client.Call(ctx, Request{}); e == nil {
		t.Fatal("post-close")
	}
}
func TestBrokerSocketCollisionPermissionsAndStopPendingReader(t *testing.T) {
	path := socket(t)
	executor := &counter{}
	cfg := Config{path, uint32(os.Geteuid()), uint32(os.Getegid()), executor}
	if os.WriteFile(path, []byte("preserve"), 0600) != nil {
		t.Fatal("write")
	}
	if Serve(context.Background(), cfg, nil) == nil {
		t.Fatal("collision overwritten")
	}
	b, _ := os.ReadFile(path)
	if string(b) != "preserve" {
		t.Fatal("collision modified")
	}
	os.Remove(path)
	os.Chmod(filepath.Dir(path), 0770)
	if Serve(context.Background(), cfg, nil) == nil {
		t.Fatal("writable parent accepted")
	}
	os.Chmod(filepath.Dir(path), 0700)
	cancel, done := start(t, cfg)
	c, e := net.DialUnix("unix", nil, &net.UnixAddr{Name: path, Net: "unix"})
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	c.Write([]byte(`{"operation":`))
	stop(t, cancel, done)
	if executor.calls.Load() != 0 {
		t.Fatal("incomplete dispatched")
	}
}

type blocking struct {
	calls   atomic.Int32
	started chan struct{}
}

func (b *blocking) Execute(ctx context.Context, _ Request) Reply {
	b.calls.Add(1)
	b.started <- struct{}{}
	<-ctx.Done()
	return Reply{Status: 503}
}
func TestBrokerWorkBoundAndCancellationJoin(t *testing.T) {
	path := socket(t)
	executor := &blocking{started: make(chan struct{}, 4)}
	cancel, done := start(t, Config{path, uint32(os.Geteuid()), uint32(os.Getegid()), executor})
	defer cancel()
	client := Client{path, uint32(os.Geteuid())}
	clientsDone := make(chan struct{}, 4)
	for i := 0; i < 4; i++ {
		go func() { client.Call(context.Background(), Request{Operation: "device"}); clientsDone <- struct{}{} }()
	}
	for i := 0; i < 4; i++ {
		select {
		case <-executor.started:
		case <-time.After(2 * time.Second):
			t.Fatal("workers not started")
		}
	}
	r, e := client.Call(context.Background(), Request{Operation: "device"})
	if e != nil || r.Status != 503 || executor.calls.Load() != 4 {
		t.Fatal("work bound", e)
	}
	stop(t, cancel, done)
	for i := 0; i < 4; i++ {
		select {
		case <-clientsDone:
		case <-time.After(2 * time.Second):
			t.Fatal("client not released")
		}
	}
	if executor.calls.Load() != 4 {
		t.Fatal("extra authority work")
	}
}
