package trustinstall

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"remoteapp.local/selfhost-poc/privatepipe"
	"remoteapp.local/selfhost-poc/trustprofile"
)

type PublicAnchor struct {
	Root         string `json:"release_root"`
	Service      string `json:"service"`
	MinimumEpoch uint64 `json:"minimum_epoch"`
}

// Build inputs are explicitly trusted local PUBLIC material. No private key
// reader, root generation, signing, server access or download exists here.
func PublicSpecification(anchor, profile, helper []byte, channel string, now time.Time) (Specification, error) {
	var a PublicAnchor
	var extra struct{}
	r := privatepipe.NewReader(bytes.NewReader(append(append([]byte(nil), anchor...), '\n')))
	if len(anchor) > 2048 || r.Read(&a) != nil || r.Read(&extra) != io.EOF {
		return Specification{}, trustprofile.ErrTrust
	}
	p, e := trustprofile.Decode(profile)
	if e != nil || len(helper) == 0 {
		return Specification{}, trustprofile.ErrTrust
	}
	h := sha256.Sum256(helper)
	s := Specification{Version: 1, Channel: channel, Root: a.Root, Service: a.Service, MinimumEpoch: a.MinimumEpoch, HelperDigest: hex.EncodeToString(h[:]), Bundled: p}
	if _, e := EncodeForTrustedBuild(s, now); e != nil {
		return Specification{}, e
	}
	return s, nil
}

// Compile uses an explicitly selected Go executable and a new output only.
// The base64 payload cannot inject additional shell or linker arguments.
func Compile(ctx context.Context, goExecutable, module, goos, output string, s Specification, now time.Time) error {
	if ctx == nil || !filepath.IsAbs(goExecutable) || !filepath.IsAbs(module) || !filepath.IsAbs(output) || (goos != "darwin" && goos != "windows") || filepath.Clean(output) != output {
		return trustprofile.ErrTrust
	}
	if _, e := os.Lstat(output); !os.IsNotExist(e) {
		return trustprofile.ErrTrust
	}
	encoded, e := EncodeForTrustedBuild(s, now)
	if e != nil {
		return e
	}
	cmd := exec.CommandContext(ctx, goExecutable, "build", "-trimpath", "-ldflags=-X main.embeddedRelease="+encoded, "-o", output, "./cmd/trust-install")
	cmd.Dir = module
	env := os.Environ()
	env = append(env, "GOENV=off", "GOTOOLCHAIN=local", "GOPROXY=off", "GOOS="+goos)
	if goos == "windows" {
		env = append(env, "CGO_ENABLED=0", "GOARCH=amd64")
	} else {
		env = append(env, "CGO_ENABLED=0", "GOARCH="+runtime.GOARCH)
	}
	cmd.Env = env
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if cmd.Run() != nil {
		return trustprofile.ErrTrust
	}
	return nil
}
