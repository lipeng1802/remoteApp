package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDeviceBuildRejectsProductWithoutCreatingDestination(t *testing.T) {
	previous := os.Args
	defer func() { os.Args = previous }()
	for _, target := range []string{"windows-device-test", "darwin-device-test"} {
		out := filepath.Join(t.TempDir(), "unused")
		os.Args = []string{"build", "/unused/go", "/unused/module", target, out, "/unused/anchor", "/unused/profile", "/unused/journal", "/unused/signer", "unused-channel", "/unused/product"}
		if run() == nil {
			t.Fatal("device-only build accepted product")
		}
		if _, err := os.Stat(out); !os.IsNotExist(err) {
			t.Fatal("invalid mode created output")
		}
	}
}
