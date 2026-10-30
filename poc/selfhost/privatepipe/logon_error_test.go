package privatepipe

import (
	"errors"
	"fmt"
	"testing"
)

func TestInteractiveLogonClassificationDoesNotTrustErrorText(t *testing.T) {
	if !IsInteractiveLogonError(fmt.Errorf("wrapped: %w", deviceLogonError)) {
		t.Fatal("typed cause lost")
	}
	if IsInteractiveLogonError(errors.New("interactive_logon_required")) {
		t.Fatal("arbitrary text classified")
	}
}
