package trustinstall

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/trustprofile"
)

func TestDesktopBuildRejectsScopeProductionAndUnpinnedInputsBeforeCompiler(t *testing.T) {
	spec, _, _, now := releaseFixture(t)
	digest := strings.Repeat("1", 64)
	for _, scope := range []invite.DesktopPermission{"", "fixed-payload", "clipboard", invite.DesktopView, invite.DesktopControl} {
		for _, bad := range []string{"scope", "production", "product", "signer", "port", "address", "remover", "path"} {
			s := spec
			p, sign, port := digest, digest, uint16(0)
			address, remover, path := "", "", "/fixed/desktop-owner"
			switch bad {
			case "scope":
				if scope == invite.DesktopView || scope == invite.DesktopControl {
					continue
				}
			case "production":
				s.Test = false
			case "product":
				p = "invalid"
			case "signer":
				sign = "invalid"
			case "port":
				port = 443
			case "address":
				address, remover = "192.0.2.1:19445", digest
			case "remover":
				address = "127.0.0.1:19445"
			case "path":
				path = "relative"
			}
			dest := filepath.Join(t.TempDir(), "desktop")
			if bad != "address" && bad != "remover" && bad != "path" {
				if e := CompileAutonomousDesktopProductTest(context.Background(), "/nonexistent/compiler", "/unused/module", "darwin", dest, s, sign, p, scope, now, port); !errors.Is(e, trustprofile.ErrTrust) {
					t.Fatal("client input reached compiler", bad, e)
				}
			}
			if e := CompileDesktopProductSupervisorTest(context.Background(), "/nonexistent/compiler", "/unused/module", "darwin", dest, s, path, p, sign, scope, address, remover, now, port); !errors.Is(e, trustprofile.ErrTrust) {
				t.Fatal("supervisor input reached compiler", bad, e)
			}
			if _, e := os.Lstat(dest); !os.IsNotExist(e) {
				t.Fatal("rejected build created output")
			}
		}
	}
}

func TestDesktopBuildPinsSourceScopeAndRetainsNativeAndTunnelFlags(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX compiler fixture")
	}
	dir := t.TempDir()
	compiler := filepath.Join(dir, "compiler")
	trace := filepath.Join(dir, "arguments")
	if e := os.WriteFile(compiler, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$REMOTEAPP_DESKTOP_BUILD_TRACE\"\nprintf 'CGO=%s\\nOS=%s\\nARCH=%s\\n' \"$CGO_ENABLED\" \"$GOOS\" \"$GOARCH\" >> \"$REMOTEAPP_DESKTOP_BUILD_TRACE\"\n"), 0700); e != nil {
		t.Fatal(e)
	}
	t.Setenv("REMOTEAPP_DESKTOP_BUILD_TRACE", trace)
	// Direct and tunnel builds each use their own correctly signed fixture.
	for _, scope := range []invite.DesktopPermission{invite.DesktopView, invite.DesktopControl} {
		for _, target := range []string{"darwin", "windows"} {
			for _, port := range []uint16{0, 19444} {
				s, _, key, n := releaseFixture(t)
				if port != 0 {
					s.Bundled.Claims.Endpoint = "https://mk.fengmap.com:8444"
					s.Bundled, _ = trustprofile.Sign(key, s.Bundled.Claims)
				}
				for _, supervised := range []bool{false, true} {
					digest := strings.Repeat("1", 64)
					dest := filepath.Join(dir, "output")
					var e error
					if supervised {
						path := "/fixed/desktop-owner"
						if target == "windows" {
							path = `C:\RemoteApp\desktop-owner.exe`
						}
						e = CompileDesktopProductSupervisorTest(context.Background(), compiler, dir, target, dest, s, path, digest, digest, scope, "127.0.0.1:19445", digest, n, port)
					} else {
						e = CompileAutonomousDesktopProductTest(context.Background(), compiler, dir, target, dest, s, digest, digest, scope, n, port)
					}
					if e != nil {
						t.Fatal(e)
					}
					raw, e := os.ReadFile(trace)
					if e != nil {
						t.Fatal(e)
					}
					args := string(raw)
					tag, _ := desktopBuildTag(scope)
					if !strings.Contains(args, "-tags="+tag) || strings.Contains(args, "main.compiledDesktopPermission") || strings.Contains(args, "main.embeddedPermission") || !strings.Contains(args, "main.embeddedSignerDigest="+digest) || !strings.Contains(args, "main.embeddedProductDigest="+digest) {
						t.Fatal("scope/pins were runtime selectable or missing")
					}
					if target == "windows" && (!strings.Contains(args, "CGO=0") || !strings.Contains(args, "ARCH=amd64")) {
						t.Fatal("Windows target changed")
					}
					if target == "darwin" && !supervised && !strings.Contains(args, "CGO=1") {
						t.Fatal("native journal silently disabled")
					}
					if port != 0 && (!strings.Contains(args, "main.embeddedTestTunnel=loopback19444") || (!supervised && !strings.Contains(args, tag+",remoteapp_test_capture"))) {
						t.Fatal("tunnel/capture lost scope")
					}
					if port == 0 && strings.Contains(args, "embeddedTestTunnel") {
						t.Fatal("direct build silently tunneled")
					}
				}
			}
		}
	}
}
