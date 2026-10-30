package main

import (
	"encoding/json"
	"errors"
	"remoteapp.local/selfhost-poc/trustprofile"
	"strings"
	"testing"
)

func TestInitializationFailureIsFixedAndPreservesLogonCause(t *testing.T) {
	for _, stage := range []string{"identity_reservation", "identity_scope", "identity_create", "enrollment_scope", "enrollment_initialize", "binding_commit", "identity_join", "PRIVATE"} {
		err := &initializationFailure{stage: stage, cause: errors.New("PRIVATE")}
		v := failureReply(err)
		raw, _ := json.Marshal(v)
		if strings.Contains(string(raw), "PRIVATE") {
			t.Fatal("raw error leaked")
		}
		if stage != "PRIVATE" && v.Stage != stage {
			t.Fatal("lost stage")
		}
	}
	v := failureReply(&initializationFailure{stage: "identity_create", cause: trustprofile.ErrDesktopLogon})
	if v.Reason != "interactive_logon_required" || v.Stage != "identity_create" {
		t.Fatal("cause lost")
	}
}
