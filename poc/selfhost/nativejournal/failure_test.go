package nativejournal

import (
	"errors"
	"fmt"
	"testing"
)

func TestNativeFailureClassificationNeverSerializesRawCause(t *testing.T) {
	for _, v := range []struct {
		err  error
		want string
	}{
		{ErrAuthentication, "native_authentication_required"},
		{fmt.Errorf("PRIVATE: %w", ErrAuthentication), "native_authentication_required"},
		{ErrLogon, "interactive_logon_required"},
		{errors.New("PRIVATE"), "native_journal_unavailable"},
		{ErrUnavailable, "native_journal_unavailable"},
	} {
		if got := FailureReason(v.err); got != v.want {
			t.Fatal("unsafe failure label")
		}
	}
	if !errors.Is(ErrAuthentication, ErrUnavailable) {
		t.Fatal("auth failure no longer unavailable")
	}
}
