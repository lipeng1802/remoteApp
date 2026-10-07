package vaultbridge

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/privatepipe"
)

type testSigner struct {
	key   ed25519.PrivateKey
	calls atomic.Int32
}

func (s *testSigner) Sign(raw []byte) ([]byte, error) {
	s.calls.Add(1)
	return ed25519.Sign(s.key, raw), nil
}
func TestBridgePinnedReopenAndRestrictions(t *testing.T) {
	pub, key, _ := ed25519.GenerateKey(rand.Reader)
	s := &testSigner{key: key}
	scope := strings.Repeat("a", 32)
	public := hex.EncodeToString(pub)
	for _, bad := range []string{
		`{"Scope":"` + scope + `","Expected":"` + public + `","Create":true}`,
		`{"Scope":"other","Expected":"` + public + `"}`,
		`{"Scope":"` + scope + `","Expected":"wrong"}`,
		`{"Scope":"` + scope + `","Scope":"` + scope + `","Expected":"` + public + `"}`,
	} {
		a, b := net.Pipe()
		done := make(chan struct{})
		go func() { handle(a, s, scope, public); a.Close(); close(done) }()
		b.SetDeadline(time.Now().Add(time.Second))
		b.Write([]byte(bad + "\n"))
		var r privatepipe.VaultReply
		if json.NewDecoder(b).Decode(&r) == nil && r.Status == "ready" {
			t.Fatal("bad reopen accepted")
		}
		b.Close()
		<-done
	}
	for _, op := range []string{"sign", "delete", "export", "create", "arbitrary"} {
		a, b := net.Pipe()
		done := make(chan struct{})
		go func() { handle(a, s, scope, public); a.Close(); close(done) }()
		b.SetDeadline(time.Now().Add(time.Second))
		enc := json.NewEncoder(b)
		read := privatepipe.NewReader(b).Read
		enc.Encode(map[string]string{"Scope": scope, "Expected": public})
		var r privatepipe.VaultReply
		if read(&r) != nil || r.Status != "ready" {
			t.Fatal("reopen failed")
		}
		message := []byte("remoteapp-provision-intent-v1\n{}")
		actual := op
		if op == "arbitrary" {
			actual = "sign"
			message = []byte("arbitrary")
		}
		enc.Encode(map[string]string{"Op": actual, "Message": base64.StdEncoding.EncodeToString(message)})
		if read(&r) != nil {
			t.Fatal("reply missing")
		}
		if op == "sign" {
			sig, _ := hex.DecodeString(r.Signature)
			if r.Status != "signed" || !ed25519.Verify(pub, message, sig) {
				t.Fatal("signature invalid")
			}
			enc.Encode(map[string]string{"Op": "stop"})
			if read(&r) != nil || r.Status != "closed" {
				t.Fatal("stop")
			}
		} else if r.Status != "failed" {
			t.Fatal("forbidden operation")
		}
		b.Close()
		<-done
	}
	if s.calls.Load() != 1 {
		t.Fatal("forbidden request reached native signer")
	}
}

func TestBridgeCancellationJoinsBlockedReader(t *testing.T) {
	ln, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() { Serve(ctx, ln, &testSigner{}, "scope", "public"); close(done) }()
	c, e := net.Dial("tcp", ln.Addr().String())
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	// Partial frame blocks the worker until cancellation closes its connection.
	c.Write([]byte(`{"Scope":`))
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("workers not joined")
	}
}

func TestBridgeConnectionLimit(t *testing.T) {
	ln, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { Serve(ctx, ln, &testSigner{}, "scope", "public"); close(done) }()
	defer func() { cancel(); <-done }()
	var connections []net.Conn
	defer func() {
		for _, c := range connections {
			c.Close()
		}
	}()
	for i := 0; i < 4; i++ {
		c, e := net.Dial("tcp", ln.Addr().String())
		if e != nil {
			t.Fatal(e)
		}
		connections = append(connections, c)
		c.SetDeadline(time.Now().Add(time.Second))
		json.NewEncoder(c).Encode(map[string]string{"Scope": "scope", "Expected": "public"})
		var reply privatepipe.VaultReply
		if privatepipe.NewReader(c).Read(&reply) != nil || reply.Status != "ready" {
			t.Fatal("allowed slot failed")
		}
	}
	fifth, e := net.Dial("tcp", ln.Addr().String())
	if e != nil {
		t.Fatal(e)
	}
	defer fifth.Close()
	fifth.SetReadDeadline(time.Now().Add(time.Second))
	var b [1]byte
	if _, e = fifth.Read(b[:]); e == nil {
		t.Fatal("unbounded admission")
	}
	if e, ok := e.(net.Error); ok && e.Timeout() {
		t.Fatal("excess connection queued, not rejected")
	}
}
