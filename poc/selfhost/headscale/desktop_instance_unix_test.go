//go:build darwin || linux

package headscale

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func desktopStage(t *testing.T) string {
	t.Helper()
	parent, err := os.MkdirTemp("/tmp", "remoteapp-hs-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(parent) })
	parent, err = filepath.EvalSymlinks(parent)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(parent, "remoteapp-desktop-test-1234567890abcdef")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestDesktopTestInstanceOnlyUsesFreshExactStage(t *testing.T) {
	root := desktopStage(t)
	instance, err := PrepareDesktopTest(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = PrepareDesktopTest(root); err == nil {
		t.Fatal("prepared stage replaced")
	}
	if _, err = OpenDesktopTest(root); err != nil {
		t.Fatal("prepared stage rejected", err)
	}
	if _, err = instance.Run(context.Background(), "nodes", "list"); err == nil {
		t.Fatal("missing isolated admin socket accepted")
	}
	config := desktopConfig(root)
	if bytes.Contains(config, []byte("/var/lib/remoteapp-poc")) || bytes.Contains(config, []byte("/etc/remoteapp-poc")) {
		t.Fatal("shared control-plane path in desktop config")
	}
	tampered := bytes.Replace(config, []byte("db.sqlite"), []byte("db.shared"), 1)
	if err = os.WriteFile(instance.configPath(), tampered, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = OpenDesktopTest(root); err == nil {
		t.Fatal("modified config accepted")
	}
}

func TestDesktopTestConfigAgainstHeadscale(t *testing.T) {
	exe := os.Getenv("REMOTEAPP_HEADSCALE_CONFIG_TEST")
	if exe == "" {
		t.Skip("explicit trusted Headscale binary required")
	}
	instance, err := PrepareDesktopTest(desktopStage(t))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, "-c", instance.configPath(), "configtest")
	cmd.Env = []string{"LANG=C"}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Headscale configtest: %v\n%s", err, out)
	}
}
