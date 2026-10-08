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
	cmd := exec.CommandContext(ctx, goexe, "build", "-trimpath", "-ldflags=-X main.embeddedRelease="+encoded+" -X main.embeddedSignerDigest="+signerDigest, "-o", dest, "./cmd/installed-session")
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
