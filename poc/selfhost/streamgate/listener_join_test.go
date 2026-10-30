package streamgate

import (
	"net"
	"testing"
	"time"
)

func TestOverlayListenerCloseWaitsForOwnCancellationWorker(t *testing.T) {
	l, e := net.Listen("tcp4", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	wrapped := &overlayListener{Listener: l, done: make(chan struct{}), workerDone: make(chan struct{})}
	release := make(chan struct{})
	go func() { <-wrapped.done; <-release; close(wrapped.workerDone) }()
	returned := make(chan error, 1)
	go func() { returned <- wrapped.Close() }()
	<-wrapped.done
	select {
	case <-returned:
		t.Fatal("close was only cancellation")
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	if e := <-returned; e != nil {
		t.Fatal(e)
	}
	if e := wrapped.Close(); e != nil {
		t.Fatal("repeat close lost original result", e)
	}
}
