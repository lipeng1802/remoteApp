package clientruntime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/session"
	"remoteapp.local/selfhost-poc/trustinstall"
	"remoteapp.local/selfhost-poc/trustprofile"
)

func TestSupervisorJoinsListenerProofAndStream(t *testing.T) {
	ln, e := net.Listen("tcp4", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	proofCancelled, streamCancelled := make(chan struct{}), make(chan struct{})
	proofRelease, streamRelease := make(chan struct{}), make(chan struct{})
	accepted, ready := make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- supervise(ctx, ln, func(ctx context.Context) error { <-ctx.Done(); close(proofCancelled); <-proofRelease; return nil }, func(ctx context.Context, c net.Conn) error {
			close(accepted)
			<-ctx.Done()
			close(streamCancelled)
			<-streamRelease
			return nil
		}, func() { close(ready) })
	}()
	<-ready
	c, e := net.Dial("tcp4", ln.Addr().String())
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	<-accepted
	cancel()
	<-proofCancelled
	<-streamCancelled
	select {
	case <-done:
		t.Fatal("returned before child join")
	default:
	}
	deadline := time.Now().Add(time.Second)
	for {
		other, err := net.DialTimeout("tcp4", ln.Addr().String(), 100*time.Millisecond)
		if err != nil {
			break
		}
		other.Close()
		if time.Now().After(deadline) {
			t.Fatal("listener survived cancellation")
		}
		time.Sleep(time.Millisecond)
	}
	close(proofRelease)
	select {
	case <-done:
		t.Fatal("stream not joined")
	default:
	}
	close(streamRelease)
	select {
	case e = <-done:
		if e != nil {
			t.Fatal(e)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("join timeout")
	}
	c.SetReadDeadline(time.Now().Add(time.Second))
	var b [1]byte
	if _, e = c.Read(b[:]); e == nil {
		t.Fatal("old stream alive")
	}
}

func TestSupervisorProofFaultClosesListener(t *testing.T) {
	ln, e := net.Listen("tcp4", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	if e = supervise(context.Background(), ln, func(context.Context) error { return session.ErrDenied }, func(context.Context, net.Conn) error { return nil }, nil); e == nil {
		t.Fatal("fault accepted")
	}
	if c, e := net.DialTimeout("tcp4", ln.Addr().String(), time.Second); e == nil {
		c.Close()
		t.Fatal("listener still active")
	}
}

func TestRuntimeRejectsMissingStateWithoutOpeningSigner(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "not-prepared")
	c := Config{StateDir: dir, AuthKey: "test", Verifier: "nodekey:" + strings.Repeat("a", 64), Target: strings.Repeat("a", 64), Controller: strings.Repeat("b", 64)}
	c.Binding.TargetNode = "nodekey:" + strings.Repeat("a", 64)
	c.Binding.ControllerNode = "nodekey:" + strings.Repeat("b", 64)
	var opened atomic.Int64
	p := trustprofile.Profile{Claims: trustprofile.Claims{Issuer: strings.Repeat("c", 64)}}
	e := Runner(trustinstall.Release{}, c, func() (Signer, error) { opened.Add(1); return nil, session.ErrDenied }, nil)(context.Background(), p)
	if e == nil || opened.Load() != 0 {
		t.Fatal("missing state admitted")
	}
	if _, e = os.Lstat(dir); !os.IsNotExist(e) {
		t.Fatal("runtime initialized missing state")
	}
}

func TestNativeSignerDigestRejectsBeforeProcessStart(t *testing.T) {
	dir, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	file := filepath.Join(dir, "not-a-signer")
	b := []byte("untrusted executable fixture")
	if e = os.WriteFile(file, b, 0700); e != nil {
		t.Fatal(e)
	}
	sum := sha256.Sum256([]byte("different trusted executable"))
	if s, e := NativeSigner(file, hex.EncodeToString(sum[:]), strings.Repeat("a", 32), strings.Repeat("b", 64))(); e == nil {
		if s != nil {
			s.Close()
		}
		t.Fatal("wrong digest started")
	}
}
