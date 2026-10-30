package nativejournal

import (
	"errors"
	"fmt"
)

// Classification only: callers must still reject the original native failure.
// It does not mean the desktop is locked, or authorize UI, retry or reset.
var ErrAuthentication = fmt.Errorf("%w: native_authentication_required", ErrUnavailable)

func FailureReason(err error) string {
	if errors.Is(err, ErrLogon) {
		return "interactive_logon_required"
	}
	if errors.Is(err, ErrAuthentication) {
		return "native_authentication_required"
	}
	return "native_journal_unavailable"
}
