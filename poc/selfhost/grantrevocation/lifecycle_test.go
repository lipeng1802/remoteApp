package grantrevocation

import (
	"context"
	"remoteapp.local/selfhost-poc/invite"
	"testing"
	"time"
)

func TestCloseCannotOutrunInflightIndependentRevocation(t *testing.T) {
	c, g, api, _ := fixture(t)
	pin(t, c, g)
	entered, finish, closed := make(chan struct{}), make(chan struct{}), make(chan struct{})
	api.reply = func(invite.Body) (invite.Result, error) {
		close(entered)
		<-finish
		return invite.Result{Status: "revoked"}, nil
	}
	result := make(chan error, 1)
	go func() { result <- c.Revoke(context.Background()) }()
	<-entered
	go func() { c.Close(); close(closed) }()
	select {
	case <-closed:
		t.Fatal("transport/signer ownership abandoned during RPC")
	case <-time.After(20 * time.Millisecond):
	}
	close(finish)
	if e := <-result; e != nil {
		t.Fatal(e)
	}
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("close did not join request")
	}
	if !api.closed || c.Revoke(context.Background()) == nil {
		t.Fatal("closed channel reused")
	}
}
