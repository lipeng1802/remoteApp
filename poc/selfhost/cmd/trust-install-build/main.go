// Trusted local build input -> new standalone maintenance bundle. Public only.
package main

import (
	"context"
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
	if ce := f.Close(); e == nil {
		e = ce
	}
	return e
}
func main() {
	if e := run(); e != nil {
		fmt.Println("BUILD_FAILED existing/partial material preserved; no trust installed")
		os.Exit(1)
	}
	fmt.Println("BUILD_READY embedded release anchor; standalone maintenance only, not installed")
}
func run() error {
	// go-executable, module, target, new-output-dir, public-anchor, public-profile,
	// trusted-helper, channel. No caller secret/root-key path is supported.
	if len(os.Args) != 9 {
		return os.ErrInvalid
	}
	goexe, module, target, dest := os.Args[1], os.Args[2], os.Args[3], os.Args[4]
	for _, p := range []string{goexe, module, dest, os.Args[5], os.Args[6], os.Args[7]} {
		if !filepath.IsAbs(p) || filepath.Clean(p) != p {
			return os.ErrInvalid
		}
	}
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
	h, e := read(os.Args[7], 100<<20)
	if e != nil {
		return e
	}
	s, e := trustinstall.PublicSpecification(a, p, h, os.Args[8], time.Now())
	if e != nil {
		return e
	}
	if e := os.Mkdir(dest, 0700); e != nil {
		return e
	}
	helper, program := "trust-journal", "trust-install"
	if target == "windows" {
		helper += ".exe"
		program += ".exe"
	}
	if e := write(filepath.Join(dest, helper), h, 0755); e != nil {
		return e
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if e := trustinstall.Compile(ctx, goexe, module, target, filepath.Join(dest, program), s, time.Now()); e != nil {
		return e
	}
	// Public trace metadata is NOT read by the runtime and grants no authority.
	b, e := json.Marshal(s)
	if e != nil {
		return e
	}
	return write(filepath.Join(dest, "build-public.json"), b, 0600)
}
