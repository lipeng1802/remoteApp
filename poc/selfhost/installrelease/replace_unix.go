//go:build !windows

package installrelease

import "os"

func replaceAtomic(source, destination string) error {
	return os.Rename(source, destination)
}
