package verifier

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"path/filepath"
	"remoteapp.local/selfhost-poc/invite"
	"testing"
	"time"
)

type proofNode struct {
	node
	started, canceled, release chan struct{}
}

func (n *proofNode) Verify(ctx context.Context, _ invite.ProvisionRecord, _ invite.ProvisionObservation, _ ed25519.PublicKey) bool {
	close(n.started)
	<-ctx.Done()
	close(n.canceled)
	<-n.release
	return true
}
func TestRuntimeProofCancellationJoinsBeforeOverlayCloseAndUnlock(t *testing.T) {
	dir := path(t)
	d := &driver{}
	var n *proofNode
	r, e := Open(context.Background(), dir, true, d, func(_ string, key string) Node {
		n = &proofNode{node: node{key: key}, started: make(chan struct{}), canceled: make(chan struct{}), release: make(chan struct{})}
		return n
	})
	if e != nil {
		t.Fatal(e)
	}
	pub, _, _ := ed25519.GenerateKey(rand.Reader)
	result := make(chan bool, 1)
	go func() {
		result <- r.Verify(context.Background(), invite.ProvisionRecord{}, invite.ProvisionObservation{}, pub)
	}()
	select {
	case <-n.started:
	case <-time.After(time.Second):
		t.Fatal("proof not started")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	if r.Stop(ctx) == nil {
		t.Fatal("proof worker falsely joined")
	}
	cancel()
	select {
	case <-n.canceled:
	case <-time.After(time.Second):
		t.Fatal("proof not canceled")
	}
	n.mu.Lock()
	closed := n.closed
	n.mu.Unlock()
	if closed {
		t.Fatal("overlay closed before proof join")
	}
	if _, e := os.Stat(filepath.Join(dir, "owner.lock")); e != nil {
		t.Fatal("lease released early")
	}
	if r.Verify(context.Background(), invite.ProvisionRecord{}, invite.ProvisionObservation{}, pub) {
		t.Fatal("stopped runtime accepted proof")
	}
	close(n.release)
	if <-result {
		t.Fatal("canceled success accepted")
	}
	stop(t, r)
	n.mu.Lock()
	closed = n.closed
	n.mu.Unlock()
	if !closed {
		t.Fatal("overlay not closed")
	}
	if _, e := os.Stat(filepath.Join(dir, "owner.lock")); !os.IsNotExist(e) {
		t.Fatal("lease not released")
	}
}
