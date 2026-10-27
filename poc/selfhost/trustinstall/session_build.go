package trustinstall

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"remoteapp.local/selfhost-poc/trustprofile"
	"time"
)

func validInstallRoot(target, root string) bool {
	if len(root) < 4 || len(root) > 1024 || strings.ContainsRune(root, '\x00') {
		return false
	}
	if target == "darwin" {
		return filepath.IsAbs(root) && filepath.Clean(root) == root && filepath.Dir(root) != root
	}
	if target != "windows" || root[0] < 'A' || root[0] > 'Z' || root[1] != ':' || root[2] != '\\' || strings.Contains(root, "/") || strings.HasSuffix(root, "\\") {
		return false
	}
	for _, component := range strings.Split(root[3:], "\\") {
		if component == "" || component == "." || component == ".." || strings.ContainsAny(component, `<>:"|?*`) {
			return false
		}
	}
	return true
}

// CompileInstallRootProvisioner pins the one directory an elevated installer
// may prepare. The resulting helper accepts no runtime path or policy input.
func CompileInstallRootProvisioner(ctx context.Context, goexe, module, target, dest, installRoot string, test bool) error {
	return compileInstallRootTool(ctx, goexe, module, target, dest, installRoot, test, "", "./cmd/install-root-provisioner")
}

// CompileInstallRootValidation builds a TEST-only verifier that pins the
// sibling provisioner digest and removes only its empty receipt-bound layout.
func CompileInstallRootValidation(ctx context.Context, goexe, module, target, dest, installRoot, provisionerDigest string) error {
	if !canonical(provisionerDigest, 32) {
		return trustprofile.ErrTrust
	}
	return compileInstallRootTool(ctx, goexe, module, target, dest, installRoot, true, provisionerDigest, "./cmd/install-root-validation")
}

func compileInstallRootTool(ctx context.Context, goexe, module, target, dest, installRoot string, test bool, pin, entry string) error {
	if ctx == nil || !filepath.IsAbs(goexe) || !filepath.IsAbs(module) || !filepath.IsAbs(dest) || filepath.Clean(dest) != dest || !validInstallRoot(target, installRoot) {
		return trustprofile.ErrTrust
	}
	if _, err := os.Lstat(dest); !os.IsNotExist(err) {
		return trustprofile.ErrTrust
	}
	encodedRoot := base64.RawURLEncoding.EncodeToString([]byte(installRoot))
	flags := "-X main.embeddedInstallRoot=" + encodedRoot + " -X main.embeddedTest=" + strconv.FormatBool(test)
	if pin != "" {
		flags += " -X main.embeddedProvisionerDigest=" + pin
	}
	arch := runtime.GOARCH
	if target == "windows" {
		arch = "amd64"
	}
	cmd := exec.CommandContext(ctx, goexe, "build", "-trimpath", "-ldflags="+flags, "-o", dest, entry)
	cmd.Dir = module
	cmd.Env = append(os.Environ(), "GOENV=off", "GOTOOLCHAIN=local", "GOPROXY=off", "CGO_ENABLED=0", "GOOS="+target, "GOARCH="+arch)
	cmd.Stdout, cmd.Stderr = io.Discard, io.Discard
	if cmd.Run() != nil {
		return trustprofile.ErrTrust
	}
	return nil
}

// CompileInstalledReleaseLauncher builds a no-argument launcher that derives
// its activation namespace and release root solely from this compiled release.
func CompileInstalledReleaseLauncher(ctx context.Context, goexe, module, target, dest string, s Specification, now time.Time) error {
	return compileReleaseTool(ctx, goexe, module, target, dest, s, "./cmd/installed-release-launcher", "", now)
}

// CompileFixedRootInstalledReleaseLauncher keeps its helper beside itself but
// always resolves releases and activation from one build-fixed install root.
func CompileFixedRootInstalledReleaseLauncher(ctx context.Context, goexe, module, target, dest string, s Specification, installRoot string, now time.Time) error {
	if !validInstallRoot(target, installRoot) {
		return trustprofile.ErrTrust
	}
	extra := " -X main.embeddedInstallRoot=" + base64.RawURLEncoding.EncodeToString([]byte(installRoot))
	return compileReleaseTool(ctx, goexe, module, target, dest, s, "./cmd/installed-release-launcher", extra, now)
}

// CompileInstalledBootstrapLauncher creates the immutable system entry. It
// reopens the signed bootstrap pointer and delegates only to the active set.
func CompileInstalledBootstrapLauncher(ctx context.Context, goexe, module, target, dest string, s Specification, installRoot string, now time.Time) error {
	if !validInstallRoot(target, installRoot) {
		return trustprofile.ErrTrust
	}
	extra := " -X main.embeddedInstallRoot=" + base64.RawURLEncoding.EncodeToString([]byte(installRoot))
	return compileReleaseTool(ctx, goexe, module, target, dest, s, "./cmd/installed-bootstrap-launcher", extra, now)
}

// CompileInstalledBootstrapManager pins one signed launcher/helper set and one
// monotonic pointer operation. Runtime cannot choose paths, roots or epochs.
func CompileInstalledBootstrapManager(ctx context.Context, goexe, module, target, dest string, s Specification, installRoot, operation, staging, manifestDigest string, manifestEpoch uint64, pointerNonce string, now time.Time) error {
	if !validInstallRoot(target, installRoot) || (operation != "install" && operation != "upgrade") || len(staging) != len(".staging-")+32 || !strings.HasPrefix(staging, ".staging-") || !canonical(strings.TrimPrefix(staging, ".staging-"), 16) || !canonical(manifestDigest, 32) || manifestEpoch < s.MinimumEpoch || manifestEpoch > 1<<53-1 || !canonical(pointerNonce, 16) {
		return trustprofile.ErrTrust
	}
	extra := " -X main.embeddedInstallRoot=" + base64.RawURLEncoding.EncodeToString([]byte(installRoot)) +
		" -X main.embeddedOperation=" + operation + " -X main.embeddedStaging=" + staging +
		" -X main.embeddedManifestDigest=" + manifestDigest + " -X main.embeddedManifestEpoch=" + strconv.FormatUint(manifestEpoch, 10) +
		" -X main.embeddedPointerNonce=" + pointerNonce
	return compileReleaseTool(ctx, goexe, module, target, dest, s, "./cmd/installed-bootstrap-manager", extra, now)
}

// CompileInstalledReleaseManager pins one hidden candidate name and signed
// manifest digest. Runtime arguments can select install/upgrade/status only.
func CompileInstalledReleaseManager(ctx context.Context, goexe, module, target, dest string, s Specification, staging, manifestDigest string, manifestEpoch uint64, now time.Time) error {
	return compileInstalledReleaseManager(ctx, goexe, module, target, dest, s, "", staging, manifestDigest, manifestEpoch, now)
}

// CompileFixedRootInstalledReleaseManager is an installer-bundle manager. Its
// protected target root is fixed at build time while its pinned journal helper
// remains beside the manager in the authenticated installer bundle.
func CompileFixedRootInstalledReleaseManager(ctx context.Context, goexe, module, target, dest string, s Specification, installRoot, staging, manifestDigest string, manifestEpoch uint64, now time.Time) error {
	if !validInstallRoot(target, installRoot) {
		return trustprofile.ErrTrust
	}
	return compileInstalledReleaseManager(ctx, goexe, module, target, dest, s, installRoot, staging, manifestDigest, manifestEpoch, now)
}

// CompileInstallTransaction pins the protected root, payload identity,
// provisioner and fixed-root manager. The resulting owner accepts no runtime
// paths and can only move one signed release monotonically toward activation.
func CompileInstallTransaction(ctx context.Context, goexe, module, target, dest string, s Specification, installRoot, operation, staging, manifestDigest string, manifestEpoch uint64, provisionerDigest, managerDigest string, now time.Time) error {
	if !validInstallRoot(target, installRoot) || (operation != "install" && operation != "upgrade") || len(staging) != len(".staging-")+32 || !strings.HasPrefix(staging, ".staging-") || !canonical(strings.TrimPrefix(staging, ".staging-"), 16) || !canonical(manifestDigest, 32) || manifestEpoch < s.MinimumEpoch || manifestEpoch > 1<<53-1 || !canonical(provisionerDigest, 32) || !canonical(managerDigest, 32) {
		return trustprofile.ErrTrust
	}
	extra := " -X main.embeddedInstallRoot=" + base64.RawURLEncoding.EncodeToString([]byte(installRoot)) +
		" -X main.embeddedOperation=" + operation + " -X main.embeddedStaging=" + staging +
		" -X main.embeddedManifestDigest=" + manifestDigest + " -X main.embeddedManifestEpoch=" + strconv.FormatUint(manifestEpoch, 10) +
		" -X main.embeddedProvisionerDigest=" + provisionerDigest + " -X main.embeddedManagerDigest=" + managerDigest
	return compileReleaseTool(ctx, goexe, module, target, dest, s, "./cmd/install-transaction", extra, now)
}

func compileInstalledReleaseManager(ctx context.Context, goexe, module, target, dest string, s Specification, installRoot, staging, manifestDigest string, manifestEpoch uint64, now time.Time) error {
	if len(staging) != len(".staging-")+32 || !strings.HasPrefix(staging, ".staging-") || !canonical(strings.TrimPrefix(staging, ".staging-"), 16) || !canonical(manifestDigest, 32) || manifestEpoch < s.MinimumEpoch || manifestEpoch > 1<<53-1 {
		return trustprofile.ErrTrust
	}
	extra := " -X main.embeddedStaging=" + staging + " -X main.embeddedManifestDigest=" + manifestDigest + " -X main.embeddedManifestEpoch=" + strconv.FormatUint(manifestEpoch, 10)
	if installRoot != "" {
		extra += " -X main.embeddedInstallRoot=" + base64.RawURLEncoding.EncodeToString([]byte(installRoot))
	}
	return compileReleaseTool(ctx, goexe, module, target, dest, s, "./cmd/installed-release-manager", extra, now)
}

func compileReleaseTool(ctx context.Context, goexe, module, target, dest string, s Specification, entry, extra string, now time.Time) error {
	if ctx == nil || !filepath.IsAbs(goexe) || !filepath.IsAbs(module) || !filepath.IsAbs(dest) || filepath.Clean(dest) != dest || (target != "darwin" && target != "windows") {
		return trustprofile.ErrTrust
	}
	if _, err := os.Lstat(dest); !os.IsNotExist(err) {
		return trustprofile.ErrTrust
	}
	encoded, err := EncodeForTrustedBuild(s, now)
	if err != nil {
		return err
	}
	arch, cgo := runtime.GOARCH, "0"
	if target == "windows" {
		arch = "amd64"
	}
	cmd := exec.CommandContext(ctx, goexe, "build", "-trimpath", "-ldflags=-X main.embeddedRelease="+encoded+extra, "-o", dest, entry)
	cmd.Dir = module
	cmd.Env = append(os.Environ(), "GOENV=off", "GOTOOLCHAIN=local", "GOPROXY=off", "CGO_ENABLED="+cgo, "GOOS="+target, "GOARCH="+arch)
	cmd.Stdout, cmd.Stderr = io.Discard, io.Discard
	if cmd.Run() != nil {
		return trustprofile.ErrTrust
	}
	return nil
}

// CompileSessionBridge is an explicit trusted build, using public release
// material and a signer-binary digest. Runtime input cannot select either root
// or helper executable. It does not install/init trust or device credentials.
func CompileSessionBridge(ctx context.Context, goexe, module, target, dest string, s Specification, signerDigest string, now time.Time) error {
	return compileSession(ctx, goexe, module, target, dest, s, signerDigest, "", now)
}

// CompileDeviceClient uses the same compiled trust and sibling digest gate,
// but exposes only device consent APIs; no runtime trust/bootstrap override.
func CompileDeviceClient(ctx context.Context, goexe, module, target, dest string, s Specification, signerDigest string, now time.Time) error {
	return compileEntry(ctx, goexe, module, target, dest, s, signerDigest, "", "./cmd/installed-device-client", false, now)
}

// CompileAutonomousProductClient is an explicit opt-in installed entry. It
// pins both native helpers and the sibling TLS product, while the ordinary
// installed-device-client remains device-only. Runtime frames cannot select a
// product, endpoint, trust root or enrollment namespace.
func CompileAutonomousProductClient(ctx context.Context, goexe, module, target, dest string, s Specification, signerDigest, productDigest string, now time.Time) error {
	if !canonical(productDigest, 32) || productDigest == "" {
		return trustprofile.ErrTrust
	}
	return compileEntry(ctx, goexe, module, target, dest, s, signerDigest, productDigest, "./cmd/installed-autonomous-product", true, now)
}

// CompileAutonomousEnrollmentInstall builds the only entry allowed to create
// the immutable local identity/enrollment binding. It requires native OS
// credential support on macOS and never runs as part of the online client.
func CompileAutonomousEnrollmentInstall(ctx context.Context, goexe, module, target, dest string, s Specification, signerDigest string, now time.Time) error {
	return compileEntry(ctx, goexe, module, target, dest, s, signerDigest, "", "./cmd/autonomous-enrollment-install", true, now)
}

// CompileProductSession pins one trusted sibling product inside the compiled
// owner. Empty pin is not a runtime switch back to bridge-only operation.
func CompileProductSession(ctx context.Context, goexe, module, target, dest string, s Specification, signerDigest, productDigest string, now time.Time) error {
	b, e := hex.DecodeString(productDigest)
	if e != nil || len(b) != 32 || hex.EncodeToString(b) != productDigest {
		return trustprofile.ErrTrust
	}
	return compileSession(ctx, goexe, module, target, dest, s, signerDigest, productDigest, now)
}
func compileSession(ctx context.Context, goexe, module, target, dest string, s Specification, signerDigest, productDigest string, now time.Time) error {
	return compileEntry(ctx, goexe, module, target, dest, s, signerDigest, productDigest, "./cmd/installed-session", false, now)
}

// CompileDesktopTestSession is a separate test-only Windows desktop entry.
// It cannot initialize a production journal or choose another root at runtime.
func CompileDesktopTestSession(ctx context.Context, goexe, module, dest string, s Specification, signerDigest, productDigest string, now time.Time) error {
	if !s.Test || !canonical(productDigest, 32) || productDigest == "" {
		return trustprofile.ErrTrust
	}
	return compileEntry(ctx, goexe, module, "windows", dest, s, signerDigest, productDigest, "./cmd/desktop-session", false, now)
}
func compileEntry(ctx context.Context, goexe, module, target, dest string, s Specification, signerDigest, productDigest, entry string, native bool, now time.Time) error {
	if ctx == nil || !filepath.IsAbs(goexe) || !filepath.IsAbs(module) || !filepath.IsAbs(dest) || filepath.Clean(dest) != dest || (target != "darwin" && target != "windows") {
		return trustprofile.ErrTrust
	}
	if _, e := os.Lstat(dest); !os.IsNotExist(e) {
		return trustprofile.ErrTrust
	}
	b, e := hex.DecodeString(signerDigest)
	if e != nil || len(b) != 32 || hex.EncodeToString(b) != signerDigest {
		return trustprofile.ErrTrust
	}
	encoded, e := EncodeForTrustedBuild(s, now)
	if e != nil {
		return e
	}
	cmd := exec.CommandContext(ctx, goexe, "build", "-trimpath", "-ldflags=-X main.embeddedRelease="+encoded+" -X main.embeddedSignerDigest="+signerDigest+" -X main.embeddedProductDigest="+productDigest, "-o", dest, entry)
	cmd.Dir = module
	arch := runtime.GOARCH
	if target == "windows" {
		arch = "amd64"
	}
	cgo := "0"
	if native && target == "darwin" {
		// nativejournal uses Security.framework. A macOS autonomous product must
		// be built on the matching Mac architecture; a CGO-disabled cross-build
		// would compile but could never open its installed enrollment state.
		cgo = "1"
	}
	cmd.Env = append(os.Environ(), "GOENV=off", "GOTOOLCHAIN=local", "GOPROXY=off", "CGO_ENABLED="+cgo, "GOOS="+target, "GOARCH="+arch)
	if cmd.Run() != nil {
		return trustprofile.ErrTrust
	}
	return nil
}
