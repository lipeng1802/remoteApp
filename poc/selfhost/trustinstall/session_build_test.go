package trustinstall

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/trustprofile"
)

func TestCompileAutonomousProductRejectsMissingOrNoncanonicalPinBeforeBuild(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "entry")
	for _, digest := range []string{"", "bad", strings.Repeat("A", 64), strings.Repeat("0", 62)} {
		err := CompileAutonomousProductClient(context.Background(), "/unused/go", "/unused/module", "darwin", dest, Specification{}, strings.Repeat("1", 64), digest, time.Now())
		if !errors.Is(err, trustprofile.ErrTrust) {
			t.Fatalf("digest %q was not rejected as trust failure: %v", digest, err)
		}
	}
}

func TestCompileInstalledReleaseManagerRejectsRuntimeSelectableCandidate(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "entry")
	s := Specification{MinimumEpoch: 2}
	for _, fixture := range []struct {
		staging string
		digest  string
		epoch   uint64
	}{
		{"candidate", strings.Repeat("1", 64), 2},
		{".staging-" + strings.Repeat("A", 32), strings.Repeat("1", 64), 2},
		{".staging-" + strings.Repeat("1", 32), strings.Repeat("A", 64), 2},
		{".staging-" + strings.Repeat("1", 32), strings.Repeat("1", 64), 1},
	} {
		err := CompileInstalledReleaseManager(context.Background(), "/unused/go", "/unused/module", "darwin", dest, s, fixture.staging, fixture.digest, fixture.epoch, time.Now())
		if !errors.Is(err, trustprofile.ErrTrust) {
			t.Fatalf("invalid fixed candidate was not rejected: %v", err)
		}
	}
}
