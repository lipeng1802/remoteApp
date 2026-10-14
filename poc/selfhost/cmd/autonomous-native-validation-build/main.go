// Trusted TEST builder: server gets TLS leaf key; client binary gets public
// release/profile only, NEVER the issuer or TLS private key.
package main

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"remoteapp.local/selfhost-poc/devicevalidation"
	"remoteapp.local/selfhost-poc/trustinstall"
	"time"
)

func write(p string, b []byte, mode os.FileMode) error {
	f, e := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if e != nil {
		return e
	}
	_, e = f.Write(b)
	syncErr := f.Sync()
	closeErr := f.Close()
	if e != nil {
		return e
	}
	if syncErr != nil {
		return syncErr
	}
	return closeErr
}
func run() error {
	if len(os.Args) != 8 {
		return os.ErrInvalid
	}
	goexe, module, dest, journal, signer, bootstrap, tlsOut := os.Args[1], os.Args[2], os.Args[3], os.Args[4], os.Args[5], os.Args[6], os.Args[7]
	for _, p := range []string{goexe, module, dest, journal, signer, bootstrap, tlsOut} {
		if !filepath.IsAbs(p) || filepath.Clean(p) != p {
			return os.ErrInvalid
		}
	}
	var public struct {
		Version int
		Issuer  string
	}
	b, e := os.ReadFile(bootstrap)
	if e != nil || json.Unmarshal(b, &public) != nil || public.Version != 1 {
		return os.ErrInvalid
	}
	pub, e := hex.DecodeString(public.Issuer)
	if e != nil || len(pub) != 32 || hex.EncodeToString(pub) != public.Issuer {
		return os.ErrInvalid
	}
	j, e := os.ReadFile(journal)
	if e != nil {
		return e
	}
	s, e := os.ReadFile(signer)
	if e != nil {
		return e
	}
	if os.Mkdir(dest, 0700) != nil {
		return os.ErrInvalid
	}
	spec, pair, e := devicevalidation.NewAuthorityMaterial(j, time.Now(), ed25519.PublicKey(pub))
	if e != nil {
		return e
	}
	encoded, e := trustinstall.EncodeForTrustedBuild(spec, time.Now())
	if e != nil {
		return e
	}
	private, _ := json.Marshal(pair)
	if write(tlsOut, private, 0600) != nil {
		return os.ErrInvalid
	}
	if write(filepath.Join(dest, "trust-journal"), j, 0700) != nil || write(filepath.Join(dest, "identity-vault"), s, 0700) != nil {
		return os.ErrInvalid
	}
	h := sha256.Sum256(s)
	digest := hex.EncodeToString(h[:])
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	c := exec.CommandContext(ctx, goexe, "build", "-trimpath", "-ldflags=-X main.embeddedRelease="+encoded+" -X main.embeddedSignerDigest="+digest, "-o", filepath.Join(dest, "autonomous-native-validation"), "./cmd/autonomous-native-validation")
	c.Dir = module
	c.Env = append(os.Environ(), "GOENV=off", "GOTOOLCHAIN=local", "GOPROXY=off", "CGO_ENABLED=1", "GOOS=darwin")
	if c.Run() != nil {
		return os.ErrInvalid
	}
	meta, _ := json.Marshal(struct {
		Version          int
		Endpoint, Issuer string
	}{1, spec.Bundled.Claims.Endpoint, public.Issuer})
	return write(filepath.Join(dest, "public-endpoint.json"), meta, 0600)
}
func main() {
	if run() != nil {
		fmt.Println("BUILD FAILED autonomous_native_bundle; retain scoped files")
		os.Exit(1)
	}
	fmt.Println("BUILD READY autonomous_native_bundle; private TLS is separate")
}
