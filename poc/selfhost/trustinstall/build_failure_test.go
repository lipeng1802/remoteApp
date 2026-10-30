package trustinstall

import (
	"context"
	"errors"
	"remoteapp.local/selfhost-poc/trustprofile"
	"testing"
	"time"
)

func TestBuildFailureClassificationDoesNotWidenBudgetOrLeak(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	e := buildProcessFailure(ctx)
	if !errors.Is(e, trustprofile.ErrTrust) || BuildFailureReason(e) != "trusted_build_interrupted" {
		t.Fatal("deadline lost")
	}
	expired, stop := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer stop()
	if BuildFailureReason(buildProcessFailure(expired)) != "trusted_build_timeout" {
		t.Fatal("deadline not classified")
	}
	if BuildFailureReason(buildProcessFailure(context.Background())) != "trusted_build_process_failed" {
		t.Fatal("process lost")
	}
	if BuildFailureReason(errors.New("PRIVATE")) != "trusted_build_rejected" {
		t.Fatal("raw text leaked")
	}
}
