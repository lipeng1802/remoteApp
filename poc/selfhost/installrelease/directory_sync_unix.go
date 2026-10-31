//go:build !windows

package installrelease

import "os"

func platformOpenDirectoryForSync(path string) (*os.File, error) {
	return os.Open(path)
}
