package verifier

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"
)

func TestOverlayFailureStagesNeverEmitPrivateErrorText(t *testing.T) {
	for _, c := range []struct{ message, want string }{
		{"setting SO_BINDTODEVICE TEST_SECRET_NODEKEY", "overlay_up_error_bind"},
		{"operation not permitted /secret/path AUTHKEY", "overlay_up_error_os_permission"},
		{"permission denied sensitive-value", "overlay_up_error_file_permission"},
		{"arbitrary provider credentials", "overlay_up_error_other"},
		{"certificate TEST_PRIVATE_MESSAGE", "overlay_up_error_certificate"},
	} {
		if got := overlayFailureStage(errors.New(c.message)); got != c.want {
			t.Fatal("non-fixed diagnostic")
		}
	}
}

func TestReportedVerifierFailureKeepsOriginalClosedBehavior(t *testing.T) {
	var labels []string
	_, err := OpenReported(context.Background(), filepath.Join(t.TempDir(), "missing"), false, &driver{}, func(string, string) Node { t.Fatal("factory invoked without lease"); return nil }, func(label string) { labels = append(labels, label) })
	if err == nil || !reflect.DeepEqual(labels, []string{"verifier_lease_failed"}) {
		t.Fatal("failure reporting changed lease gate")
	}
}
