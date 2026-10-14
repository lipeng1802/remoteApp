// Trusted TEST builder: server gets TLS leaf key; client binary gets public
// release/profile only, NEVER the issuer or TLS private key.
package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"remoteapp.local/selfhost-poc/devicevalidation"
	"remoteapp.local/selfhost-poc/trustinstall"
	"runtime"
	"strings"
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
func targetBuild(target string) (suffix, cgo, arch string, err error) {
	switch target {
	case "darwin":
		return "", "1", runtime.GOARCH, nil
	case "windows":
		return ".exe", "0", "amd64", nil
	default:
		return "", "", "", os.ErrInvalid
	}
}
func publicIssuer(b []byte) (ed25519.PublicKey, error) {
	var p struct {
		Version int
		Issuer  string
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(&p) != nil || d.Decode(new(any)) != io.EOF || p.Version != 1 {
		return nil, os.ErrInvalid
	}
	key, e := hex.DecodeString(p.Issuer)
	if e != nil || len(key) != ed25519.PublicKeySize || hex.EncodeToString(key) != p.Issuer || bytes.Equal(key, make([]byte, ed25519.PublicKeySize)) {
		return nil, os.ErrInvalid
	}
	return ed25519.PublicKey(key), nil
}
func privateOutputAllowed(dest, tlsOut string) bool {
	rel, e := filepath.Rel(dest, tlsOut)
	if e != nil || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))) {
		return false
	}
	for _, path := range []string{dest, tlsOut} {
		parent, e := filepath.EvalSymlinks(filepath.Dir(path))
		if e != nil || parent != filepath.Dir(path) {
			return false
		}
	}
	return true
}
func run() error {
	if len(os.Args) != 9 {
		return os.ErrInvalid
	}
	goexe, module, target, dest, journal, signer, bootstrap, tlsOut := os.Args[1], os.Args[2], os.Args[3], os.Args[4], os.Args[5], os.Args[6], os.Args[7], os.Args[8]
	suffix, cgo, arch, e := targetBuild(target)
	if e != nil {
		return os.ErrInvalid
	}
	for _, p := range []string{goexe, module, dest, journal, signer, bootstrap, tlsOut} {
		if !filepath.IsAbs(p) || filepath.Clean(p) != p {
			return os.ErrInvalid
		}
	}
	// Server-only private output must never land in the distributable bundle.
	if !privateOutputAllowed(dest, tlsOut) {
		return os.ErrInvalid
	}
	b, e := os.ReadFile(bootstrap)
	if e != nil {
		return os.ErrInvalid
	}
	pub, e := publicIssuer(b)
	if e != nil {
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
	if len(j) == 0 || len(s) == 0 || write(filepath.Join(dest, "trust-journal"+suffix), j, 0700) != nil || write(filepath.Join(dest, "identity-vault"+suffix), s, 0700) != nil {
		return os.ErrInvalid
	}
	h := sha256.Sum256(s)
	digest := hex.EncodeToString(h[:])
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	c := exec.CommandContext(ctx, goexe, "build", "-trimpath", "-ldflags=-X main.embeddedRelease="+encoded+" -X main.embeddedSignerDigest="+digest, "-o", filepath.Join(dest, "autonomous-native-validation"+suffix), "./cmd/autonomous-native-validation")
	c.Dir = module
	c.Env = append(os.Environ(), "GOENV=off", "GOTOOLCHAIN=local", "GOPROXY=off", "CGO_ENABLED="+cgo, "GOOS="+target, "GOARCH="+arch)
	if c.Run() != nil {
		return os.ErrInvalid
	}
	meta, _ := json.Marshal(struct {
		Version          int
		Endpoint, Issuer string
	}{1, spec.Bundled.Claims.Endpoint, hex.EncodeToString(pub)})
	if write(filepath.Join(dest, "public-endpoint.json"), meta, 0600) != nil {
		return os.ErrInvalid
	}
	if target == "windows" {
		b, e := os.ReadFile(filepath.Join(module, "public-server/run-autonomous-native-validation.ps1"))
		if e != nil || write(filepath.Join(dest, "run-autonomous-native-validation.ps1"), b, 0600) != nil {
			return os.ErrInvalid
		}
	}
	hashes := map[string]string{}
	entries, e := os.ReadDir(dest)
	if e != nil {
		return e
	}
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			return os.ErrInvalid
		}
		b, e := os.ReadFile(filepath.Join(dest, entry.Name()))
		if e != nil {
			return e
		}
		h := sha256.Sum256(b)
		hashes[entry.Name()] = hex.EncodeToString(h[:])
	}
	b, e = json.Marshal(hashes)
	if e != nil {
		return e
	}
	return write(filepath.Join(dest, "SHA256.json"), b, 0600)
}
func main() {
	if run() != nil {
		fmt.Println("BUILD FAILED autonomous_native_bundle; retain scoped files")
		os.Exit(1)
	}
	fmt.Println("BUILD READY autonomous_native_bundle; private TLS is separate")
}
