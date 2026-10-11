package clientruntime

import (
	"context"
	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/processowner"
	"remoteapp.local/selfhost-poc/trustinstall"
	"strings"
	"testing"
)

func TestCertificateArgumentsAreCanonicalAndImmutable(t *testing.T) {
	scope := strings.Repeat("a", 64)
	original := processowner.Spec{}
	for _, reopen := range []bool{false, true} {
		got, e := certificateSpec(original, scope, reopen)
		if e != nil || len(got.Args) != 2 || got.Args[1] != scope {
			t.Fatal("canonical scope rejected")
		}
		mode := "--create-test-certificate"
		if reopen {
			mode = "--reopen-test-certificate"
		}
		if got.Args[0] != mode || len(original.Args) != 0 {
			t.Fatal("mode or source spec changed")
		}
	}
	for _, bad := range []string{"", strings.Repeat("A", 64), strings.Repeat("a", 63), strings.Repeat("g", 64), scope + "\n"} {
		if _, e := certificateSpec(original, bad, false); e == nil {
			t.Fatal("noncanonical scope accepted")
		}
	}
	if _, e := certificateSpec(processowner.Spec{Args: []string{"peer-supplied"}}, scope, false); e == nil {
		t.Fatal("existing arguments accepted")
	}
}

func TestCertificateMaintenanceRejectsUntrustedModesBeforeLaunch(t *testing.T) {
	for _, tc := range [][2]string{{"--delete-production", ""}, {"--test-certificate-selftest", "foreign"}, {"--cleanup-test-certificate", "invalid"}} {
		if e := certificateMaintenance(context.Background(), processowner.Spec{}, tc[0], tc[1]); e == nil {
			t.Fatal("untrusted maintenance accepted")
		}
	}
	if _, e := TestCertificateScope(trustinstall.Release{}, invite.Grant{}); e == nil {
		t.Fatal("production/empty release accepted")
	}
}
