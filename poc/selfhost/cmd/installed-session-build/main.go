package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
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
	fmt.Println("BUILD_READY compiled session entry + pinned helpers; NOT installed or cross-machine accepted")
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
	device := target == "windows-device-test" || target == "darwin-device-test"
	if device {
		if len(os.Args) != 10 {
			return os.ErrInvalid
		}
		if target == "windows-device-test" {
			target = "windows"
		} else {
			target = "darwin"
		}
	}
	borrowed := target == "windows-desktop-borrowed-test"
	macTest := target == "darwin-test"
	if macTest {
		if len(os.Args) != 11 {
			return os.ErrInvalid
		}
		target = "darwin"
	}
	desktop := target == "windows-desktop-test" || borrowed
	if desktop {
		target = "windows"
		if len(os.Args) != 11 {
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
	if desktop || macTest || device {
		var nonce [16]byte
		if _, e = rand.Read(nonce[:]); e != nil {
			return e
		}
		prefix := "desktop-test-"
		if macTest {
			prefix = "compiled-mac-test-"
		}
		if device {
			prefix = "device-client-test-"
		}
		s.Channel = prefix + hex.EncodeToString(nonce[:])
		s.Test = true
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
	if device {
		e = trustinstall.CompileDeviceClient(ctx, goexe, module, target, filepath.Join(dest, "installed-device-client"+suffix), s, pin, time.Now())
	} else if productPin != "" {
		if e = write(filepath.Join(dest, "tls-product"+suffix), product, 0755); e != nil {
			return e
		}
		if desktop {
			e = trustinstall.CompileDesktopTestSession(ctx, goexe, module, filepath.Join(dest, "desktop-session.exe"), s, pin, productPin, time.Now())
		} else {
			e = trustinstall.CompileProductSession(ctx, goexe, module, target, filepath.Join(dest, "installed-session"+suffix), s, pin, productPin, time.Now())
		}
	} else {
		e = trustinstall.CompileSessionBridge(ctx, goexe, module, target, filepath.Join(dest, "installed-session"+suffix), s, pin, time.Now())
	}
	if e != nil {
		return e
	}
	if desktop {
		for _, name := range []string{"desktop-relay", "interactive-vault", "vault-relay"} {
			cmd := exec.CommandContext(ctx, goexe, "build", "-trimpath", "-o", filepath.Join(dest, name+".exe"), "./cmd/"+name)
			cmd.Dir = module
			cmd.Env = append(os.Environ(), "GOENV=off", "GOTOOLCHAIN=local", "GOPROXY=off", "CGO_ENABLED=0", "GOOS=windows", "GOARCH=amd64")
			if cmd.Run() != nil {
				return os.ErrInvalid
			}
		}
		for _, name := range []string{"start-desktop-session.ps1", "probe-desktop-session.ps1"} {
			raw, e := read(filepath.Join(module, "public-server", name), 32768)
			if e != nil {
				return e
			}
			if e = write(filepath.Join(dest, name), raw, 0600); e != nil {
				return e
			}
		}
		if borrowed {
			raw, e := read(filepath.Join(module, "public-server", "start-borrowed-desktop-session.ps1"), 32768)
			if e != nil {
				return e
			}
			if e = write(filepath.Join(dest, "start-borrowed-desktop-session.ps1"), raw, 0600); e != nil {
				return e
			}
		}
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
	if e = write(filepath.Join(dest, "build-public.json"), b, 0600); e != nil {
		return e
	} // runtime ignores this sidecar
	if desktop || macTest || device {
		manifest := map[string]string{}
		entries, e := os.ReadDir(dest)
		if e != nil {
			return e
		}
		for _, entry := range entries {
			raw, e := read(filepath.Join(dest, entry.Name()), 100<<20)
			if e != nil {
				return e
			}
			sum := sha256.Sum256(raw)
			manifest[entry.Name()] = hex.EncodeToString(sum[:])
		}
		raw, e := json.Marshal(manifest)
		if e != nil {
			return e
		}
		return write(filepath.Join(dest, "SHA256.json"), raw, 0600)
	}
	return nil
}
