package clientowner

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestSessionStateRequiresActualCompletion(t *testing.T) {
	var absent *Handle
	if absent.State() != (SessionState{Status: "session_state"}) {
		t.Fatal("absent session claimed completion")
	}
	h := &Handle{done: make(chan struct{}), err: errors.New("private runner detail")}
	if h.State().Joined || h.State().Fault {
		t.Fatal("unfinished runner claimed join/fault")
	}
	close(h.done)
	if s := h.State(); !s.Joined || !s.Fault {
		t.Fatal("fault completion not reported")
	}
	b, _ := json.Marshal(h.State())
	if string(b) != `{"status":"session_state","joined":true,"fault":true}` {
		t.Fatal("state schema exposed runner details")
	}
	normal := &Handle{done: make(chan struct{})}
	close(normal.done)
	if !normal.State().Joined || normal.State().Fault {
		t.Fatal("normal completion classified as fault")
	}
}
