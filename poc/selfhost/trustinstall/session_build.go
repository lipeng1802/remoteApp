package trustinstall

import (
	"context"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"remoteapp.local/selfhost-poc/trustprofile"
	"time"
)

// CompileSessionBridge is an explicit trusted build, using public release
// material and a signer-binary digest. Runtime input cannot select either root
// or helper executable. It does not install/init trust or device credentials.
func CompileSessionBridge(ctx context.Context, goexe, module, target, dest string, s Specification, signerDigest string, now time.Time) error {
	return compileSession(ctx, goexe, module, target, dest, s, signerDigest, "", now)
}

// CompileAutonomousInstaller builds offline initialization/upgrade IPC with the
// same fixed trust root and identity helper as the autonomous product entry.
func CompileAutonomousInstaller(ctx context.Context, goexe, module, target, dest string, s Specification, signerDigest string, now time.Time) error {
	return compileEntry(ctx, goexe, module, target, dest, s, signerDigest, "", "./cmd/autonomous-install", now)
}

// CompileDeviceClient uses the same compiled trust and sibling digest gate,
// but exposes only device consent APIs; no runtime trust/bootstrap override.
func CompileDeviceClient(ctx context.Context, goexe, module, target, dest string, s Specification, signerDigest string, now time.Time) error {
	return compileEntry(ctx, goexe, module, target, dest, s, signerDigest, "", "./cmd/installed-device-client", now)
}

// CompileAutonomousProductClient is an explicit opt-in installed entry. It
// pins both native helpers and the sibling TLS product, while the ordinary
// installed-device-client remains device-only. Runtime frames cannot select a
// product, endpoint, trust root or enrollment namespace.
func CompileAutonomousProductClient(ctx context.Context, goexe, module, target, dest string, s Specification, signerDigest, productDigest string, now time.Time) error {
	if !canonical(productDigest, 32) || productDigest == "" {
		return trustprofile.ErrTrust
	}
	return compileEntry(ctx, goexe, module, target, dest, s, signerDigest, productDigest, "./cmd/installed-autonomous-product", now)
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
	return compileEntry(ctx, goexe, module, target, dest, s, signerDigest, productDigest, "./cmd/installed-session", now)
}

// CompileDesktopTestSession is a separate test-only Windows desktop entry.
// It cannot initialize a production journal or choose another root at runtime.
func CompileDesktopTestSession(ctx context.Context, goexe, module, dest string, s Specification, signerDigest, productDigest string, now time.Time) error {
	if !s.Test || !canonical(productDigest, 32) || productDigest == "" {
		return trustprofile.ErrTrust
	}
	return compileEntry(ctx, goexe, module, "windows", dest, s, signerDigest, productDigest, "./cmd/desktop-session", now)
}
func compileEntry(ctx context.Context, goexe, module, target, dest string, s Specification, signerDigest, productDigest, entry string, now time.Time) error {
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
	cmd.Env = append(os.Environ(), "GOENV=off", "GOTOOLCHAIN=local", "GOPROXY=off", "CGO_ENABLED=0", "GOOS="+target, "GOARCH="+arch)
	if cmd.Run() != nil {
		return trustprofile.ErrTrust
	}
	return nil
}
