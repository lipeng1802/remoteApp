package main

import (
	"strings"
	"testing"
)

func TestChildFailureOnlyAcceptsFixedStrictStage(t *testing.T) {
	for _, v := range []struct{ raw, want string }{
		{`{"status":"failed","stage":"identity_create","reason":"installation_rejected"}`, "native_identity_create"},
		{`{"status":"failed","stage":"PRIVATE"}`, "native_enrollment"},
		{`{"status":"failed","stage":"binding_commit","PRIVATE":true}`, "native_enrollment"},
		{`{"status":"failed","stage":"binding_commit","stage":"identity_create"}`, "native_enrollment"},
		{`{"status":"usable","stage":"binding_commit"}`, "native_enrollment"},
		{"{\"status\":\"failed\",\"stage\":\"identity_create\"}\n{}", "native_enrollment"},
		{strings.Repeat("PRIVATE", 1000), "native_enrollment"},
	} {
		if got := enrollmentFailureStage([]byte(v.raw + "\n")); got != v.want {
			t.Fatalf("incorrect fixed stage %s", got)
		}
	}
}

func TestEnrollmentReasonRejectsRawUnknownAndMixedReplies(t *testing.T) {
	for _, v := range []struct{ raw, want string }{
		{`{"status":"failed","stage":"identity_intent","reason":"native_helper_timeout"}`, "native_helper_timeout"},
		{`{"status":"failed","stage":"identity_create","reason":"interactive_logon_required"}`, "interactive_logon_required"},
		{`{"status":"failed","stage":"PRIVATE","reason":"native_helper_timeout"}`, ""},
		{`{"status":"failed","stage":"identity_create","reason":"PRIVATE"}`, ""},
		{`{"status":"failed","stage":"identity_create","reason":"native_helper_timeout","PRIVATE":true}`, ""},
	} {
		if got := enrollmentFailureReason([]byte(v.raw + "\n")); got != v.want {
			t.Fatal("unsafe enrollment reason")
		}
	}
}
