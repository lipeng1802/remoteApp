//go:build windows

package trustinstall

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/trustprofile"
)

// A real child executable, not a compiler or native-resource creator. The
// builder must discard its arbitrary diagnostic and retain only the category.
func init() {
	if os.Getenv("REMOTEAPP_BUILD_FAILURE_FIXTURE") == "exit23" && len(os.Args) > 1 && os.Args[1] == "build" {
		fmt.Fprintln(os.Stderr, "PRIVATE_COMPILER_DIAGNOSTIC")
		os.Exit(23)
	}
}

func TestWindowsBuildProcessFailureCategories(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("REMOTEAPP_BUILD_FAILURE_FIXTURE", "exit23")
	for _, mode := range []string{"process", "cancel", "timeout"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			want := "trusted_build_process_failed"
			if mode == "cancel" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
				want = "trusted_build_interrupted"
			}
			if mode == "timeout" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithDeadline(ctx, time.Now().Add(-time.Second))
				defer cancel()
				want = "trusted_build_timeout"
			}
			dir := t.TempDir()
			dest := filepath.Join(dir, "unused.exe")
			err := CompileInstallRootProvisioner(ctx, exe, dir, "windows", dest, filepath.Join(dir, "root"), true)
			if !errors.Is(err, trustprofile.ErrTrust) || BuildFailureReason(err) != want || strings.Contains(err.Error(), "PRIVATE") {
				t.Fatalf("incorrect fixed build result: %v", err)
			}
			if _, err = os.Stat(dest); !os.IsNotExist(err) {
				t.Fatal("failed build created output")
			}
			if _, err = os.Stat(filepath.Join(dir, "root")); !os.IsNotExist(err) {
				t.Fatal("compile-only path created installation")
			}
		})
	}
}
