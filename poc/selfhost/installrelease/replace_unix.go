//go:build !windows

package installrelease

import "os"

func atomicPointerMode(test bool) os.FileMode {
	if test {
		return 0400
	}
	return 0444
}

func replaceAtomic(source, destination string) error {
	return os.Rename(source, destination)
}
