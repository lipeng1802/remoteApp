package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"remoteapp.local/selfhost-poc/trustinstall"
)

func read(path string, limit int64) ([]byte, error) {
	f, e := os.Open(path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, limit+1))
	if e != nil || int64(len(b)) > limit {
		return nil, os.ErrInvalid
	}
	return b, nil
}
func write(path string, b []byte, mode os.FileMode) error {
	f, e := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if e != nil {
		return e
	}
	_, e = f.Write(b)
	if e == nil {
		e = f.Sync()
	}
	if closed := f.Close(); e == nil {
		e = closed
	}
	return e
}
func main() {
	if e := run(); e != nil {
		fmt.Println("BUILD_FAILED partial material preserved; no installation or session started")
		os.Exit(1)
	}
	fmt.Println("BUILD_READY compiled installed-session + pinned helpers; NOT installed or cross-machine accepted")
}
func run() error {
	// Optional last arg: trusted standalone TLS product (no runtime peer path).
	// .NET callers must publish a complete self-contained single-file product;
	// copying only an apphost from a multi-file publish is NOT a usable bundle.
	if len(os.Args) != 10 && len(os.Args) != 11 {
		return os.ErrInvalid
	}
	for _, p := range []string{os.Args[1], os.Args[2], os.Args[4], os.Args[5], os.Args[6], os.Args[7], os.Args[8]} {
		if !filepath.IsAbs(p) || filepath.Clean(p) != p {
			return os.ErrInvalid
		}
	}
	goexe, module, target, dest := os.Args[1], os.Args[2], os.Args[3], os.Args[4]
	if target != "darwin" && target != "windows" {
		return os.ErrInvalid
	}
	a, e := read(os.Args[5], 2048)
	if e != nil {
		return e
	}
	p, e := read(os.Args[6], 8192)
	if e != nil {
		return e
	}
	j, e := read(os.Args[7], 100<<20)
	if e != nil || len(j) == 0 {
		return os.ErrInvalid
	}
	v, e := read(os.Args[8], 100<<20)
	if e != nil || len(v) == 0 {
		return os.ErrInvalid
	}
	var product []byte
	productPin := ""
	if len(os.Args) == 11 {
		if !filepath.IsAbs(os.Args[10]) || filepath.Clean(os.Args[10]) != os.Args[10] {
			return os.ErrInvalid
		}
		product, e = read(os.Args[10], 100<<20)
		if e != nil || len(product) == 0 {
			return os.ErrInvalid
		}
		digest := sha256.Sum256(product)
		productPin = hex.EncodeToString(digest[:])
	}
	s, e := trustinstall.PublicSpecification(a, p, j, os.Args[9], time.Now())
	if e != nil {
		return e
	}
	digest := sha256.Sum256(v)
	pin := hex.EncodeToString(digest[:])
	if e = os.Mkdir(dest, 0700); e != nil {
		return e
	}
	suffix := ""
	if target == "windows" {
		suffix = ".exe"
	}
	if e = write(filepath.Join(dest, "trust-journal"+suffix), j, 0755); e != nil {
		return e
	}
	if e = write(filepath.Join(dest, "identity-vault"+suffix), v, 0755); e != nil {
		return e
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	if productPin != "" {
		if e = write(filepath.Join(dest, "tls-product"+suffix), product, 0755); e != nil {
			return e
		}
		e = trustinstall.CompileProductSession(ctx, goexe, module, target, filepath.Join(dest, "installed-session"+suffix), s, pin, productPin, time.Now())
	} else {
		e = trustinstall.CompileSessionBridge(ctx, goexe, module, target, filepath.Join(dest, "installed-session"+suffix), s, pin, time.Now())
	}
	if e != nil {
		return e
	}
	if e = trustinstall.Compile(ctx, goexe, module, target, filepath.Join(dest, "trust-install"+suffix), s, time.Now()); e != nil {
		return e
	}
	b, e := json.Marshal(struct {
		Release       trustinstall.Specification `json:"release"`
		SignerDigest  string                     `json:"signer_sha256"`
		ProductDigest string                     `json:"product_sha256,omitempty"`
	}{s, pin, productPin})
	if e != nil {
		return e
	}
	return write(filepath.Join(dest, "build-public.json"), b, 0600) // runtime ignores this sidecar
}
