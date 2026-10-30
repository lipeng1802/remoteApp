package trustprofile

import (
	"errors"
	"strings"
	"testing"
)

func TestNativeAuthenticationRemainsTrustFailure(t *testing.T) {
	e := journalFailure("native_authentication_required")
	if !errors.Is(e, ErrTrust) || !errors.Is(e, ErrNativeAuthentication) {
		t.Fatal("failure upgraded to trust")
	}
	if errors.Is(e, ErrDesktopLogon) {
		t.Fatal("authentication falsely proved desktop logoff")
	}
	if journalFailure("interactive_logon_required") != ErrDesktopLogon {
		t.Fatal("lost native logon")
	}
	e = journalFailure("PRIVATE")
	if e != ErrTrust || strings.Contains(e.Error(), "PRIVATE") {
		t.Fatal("unknown reason leaked")
	}
}
