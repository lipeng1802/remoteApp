//go:build !windows

package installrelease

import (
	"os"
	"testing"
)

func makeBroadlyWritableParent(t *testing.T, path string) {
	t.Helper()
	if err := os.Chmod(path, 0777); err != nil {
		t.Fatal(err)
	}
}
