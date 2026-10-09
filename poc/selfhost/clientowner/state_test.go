package clientowner

import (
	"context"
	"encoding/json"
	"errors"
	"remoteapp.local/selfhost-poc/trustprofile"
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

func TestFaultRetirementRequiresJoinAndCannotRestart(t *testing.T) {
	o, j, _ := setup(t)
	if o.RetireFault(context.Background()) == nil {
		t.Fatal("no session retired")
	}
	release := make(chan struct{})
	h := mustStart(t, o, func(context.Context, trustprofile.Profile) error { <-release; return errors.New("runner fault") })
	if o.RetireFault(context.Background()) == nil {
		t.Fatal("pending runner retired")
	}
	j.mu.Lock()
	closed := j.closed
	j.mu.Unlock()
	if closed {
		t.Fatal("pending lease released")
	}
	close(release)
	<-h.Done()
	if e := o.RetireFault(context.Background()); e != nil {
		t.Fatal(e)
	}
	j.mu.Lock()
	closed = j.closed
	j.mu.Unlock()
	if !closed {
		t.Fatal("native journal not released")
	}
	if _, e := o.Start(context.Background(), func(context.Context, trustprofile.Profile) error { return nil }); e == nil {
		t.Fatal("fault owner restarted")
	}
	if o.RetireFault(context.Background()) == nil {
		t.Fatal("duplicate retirement accepted")
	}
	if e := o.Close(context.Background()); e != nil {
		t.Fatal("retired owner final close", e)
	}
}

func TestNormalCompletionIsNotFaultRetirement(t *testing.T) {
	o, j, _ := setup(t)
	h := mustStart(t, o, func(context.Context, trustprofile.Profile) error { return nil })
	<-h.Done()
	if o.RetireFault(context.Background()) == nil {
		t.Fatal("normal completion accepted as fault")
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.closed {
		t.Fatal("normal journal unexpectedly retired")
	}
}

type failedRetirementJournal struct{ *journal }

func (j failedRetirementJournal) Close() error { return errors.New("journal release failed") }

func TestFaultRetirementDoesNotConfirmFailedJournalClose(t *testing.T) {
	o, j, _ := setup(t)
	h := mustStart(t, o, func(context.Context, trustprofile.Profile) error { return errors.New("fault") })
	<-h.Done()
	o.journal = failedRetirementJournal{j}
	if o.RetireFault(context.Background()) == nil || o.closed {
		t.Fatal("failed journal release confirmed")
	}
	if _, e := o.Start(context.Background(), func(context.Context, trustprofile.Profile) error { return nil }); e == nil {
		t.Fatal("fault owner reusable after failed release")
	}
	o.journal = j // fixture restoration only; explicit test teardown still closes.
}
