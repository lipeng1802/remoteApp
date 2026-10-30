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
