package nodeproof

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"remoteapp.local/selfhost-poc/startuptrace"
	"syscall"
	"tailscale.com/client/local"
	"testing"
)

func TestRequestFailureUsesTypedCausesOnly(t *testing.T) {
	for _, item := range []struct {
		err   error
		stage startuptrace.Stage
	}{
		{context.DeadlineExceeded, startuptrace.OverlayRequestDeadline},
		{context.Canceled, startuptrace.OverlayRequestCanceled},
		{io.EOF, startuptrace.OverlayRequestEOF},
		{io.ErrUnexpectedEOF, startuptrace.OverlayRequestEOF},
		{syscall.ECONNREFUSED, startuptrace.OverlayRequestRefused},
		{syscall.ECONNRESET, startuptrace.OverlayRequestReset},
		{&net.DNSError{IsTimeout: true}, startuptrace.OverlayRequestTimeout},
		{errors.New("context deadline exceeded PRIVATE"), startuptrace.OverlayRequestFailed},
		{errors.New("connection refused PRIVATE"), startuptrace.OverlayRequestFailed},
		{nil, startuptrace.OverlayRequestFailed},
	} {
		if got := classifyRequestFailure(fmt.Errorf("PRIVATE: %w", item.err)); got != item.stage {
			t.Fatalf("fixed stage mismatch: %s", got)
		}
	}
}

func TestLocalRequestTransportFailureRemainsNonRetryable(t *testing.T) {
	for _, cause := range []error{context.Canceled, context.DeadlineExceeded, syscall.ECONNREFUSED, io.EOF} {
		calls := 0
		lc := &local.Client{Dial: func(context.Context, string, string) (net.Conn, error) { calls++; return nil, cause }}
		conn, err := DialOnlyOverlay(context.Background(), lc, "100.120.0.2", 47476)
		if conn != nil || !errors.Is(err, ErrOverlayDial) || errors.Is(err, ErrOverlayNotListening) || calls != 1 {
			t.Fatal("transport denial or attempt count changed")
		}
	}
}
