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

func TestInstallRootBuildConfigurationIsCanonicalAndFixed(t *testing.T) {
	valid := []struct {
		target string
		root   string
	}{
		{"darwin", "/Library/Application Support/PersonalRemoteDesktop"},
		{"windows", `C:\Program Files\Personal Remote Desktop`},
	}
	for _, fixture := range valid {
		if !validInstallRoot(fixture.target, fixture.root) {
			t.Fatalf("valid fixed root rejected: %#v", fixture)
		}
	}
	invalid := []struct {
		target string
		root   string
	}{
		{"darwin", "relative/root"},
		{"darwin", "/"},
		{"darwin", "/fixed/../root"},
		{"darwin", "/fixed/root/"},
		{"darwin", "//fixed/root"},
		{"darwin", `C:\Program Files\RemoteApp`},
		{"windows", `c:\Program Files\RemoteApp`},
		{"windows", `C:\Program Files\..\RemoteApp`},
		{"windows", `C:/Program Files/RemoteApp`},
		{"windows", `C:\Program Files\RemoteApp\`},
		{"linux", "/opt/remoteapp"},
	}
	for _, fixture := range invalid {
		if validInstallRoot(fixture.target, fixture.root) {
			t.Fatalf("invalid fixed root accepted: %#v", fixture)
		}
	}

	destination := filepath.Join(t.TempDir(), "validation")
	err := CompileInstallRootValidation(context.Background(), "/unused/go", "/unused/module", "darwin", destination, "/fixed/root", strings.Repeat("A", 64))
	if !errors.Is(err, trustprofile.ErrTrust) {
		t.Fatalf("noncanonical provisioner pin accepted: %v", err)
	}
}

func TestInstallTransactionRejectsRuntimeSelectableOrUnpinnedInputs(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "transaction")
	specification := Specification{MinimumEpoch: 2}
	validStage, validDigest := ".staging-"+strings.Repeat("1", 32), strings.Repeat("2", 64)
	for _, fixture := range []struct {
		root      string
		operation string
		stage     string
		digest    string
		epoch     uint64
		pin       string
	}{
		{"relative", "install", validStage, validDigest, 2, validDigest},
		{"/fixed/root", "repair", validStage, validDigest, 2, validDigest},
		{"/fixed/root", "install", "candidate", validDigest, 2, validDigest},
		{"/fixed/root", "install", validStage, "bad", 2, validDigest},
		{"/fixed/root", "install", validStage, validDigest, 1, validDigest},
		{"/fixed/root", "install", validStage, validDigest, 2, "bad"},
	} {
		err := CompileInstallTransaction(context.Background(), "/unused/go", "/unused/module", "darwin", destination, specification, fixture.root, fixture.operation, fixture.stage, fixture.digest, fixture.epoch, fixture.pin, validDigest, time.Now())
		if !errors.Is(err, trustprofile.ErrTrust) {
			t.Fatalf("invalid transaction input accepted: %#v err=%v", fixture, err)
		}
	}
}

func TestBootstrapBuildRejectsSelectableRootOrPointer(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "bootstrap")
	specification := Specification{MinimumEpoch: 2}
	stage, digest, nonce := ".staging-"+strings.Repeat("1", 32), strings.Repeat("2", 64), strings.Repeat("3", 32)
	for _, fixture := range []struct {
		root      string
		operation string
		stage     string
		digest    string
		epoch     uint64
		nonce     string
	}{
		{"relative", "install", stage, digest, 2, nonce},
		{"/fixed/root", "repair", stage, digest, 2, nonce},
		{"/fixed/root", "install", "candidate", digest, 2, nonce},
		{"/fixed/root", "install", stage, "bad", 2, nonce},
		{"/fixed/root", "install", stage, digest, 1, nonce},
		{"/fixed/root", "install", stage, digest, 2, "bad"},
	} {
		err := CompileInstalledBootstrapManager(context.Background(), "/unused/go", "/unused/module", "darwin", destination, specification, fixture.root, fixture.operation, fixture.stage, fixture.digest, fixture.epoch, fixture.nonce, time.Now())
		if !errors.Is(err, trustprofile.ErrTrust) {
			t.Fatalf("invalid bootstrap input accepted: %#v err=%v", fixture, err)
		}
	}
	if err := CompileInstalledBootstrapLauncher(context.Background(), "/unused/go", "/unused/module", "windows", destination, specification, `c:\bad`, time.Now()); !errors.Is(err, trustprofile.ErrTrust) {
		t.Fatalf("invalid dispatcher root accepted: %v", err)
	}
}
