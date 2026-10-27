//go:build windows

package installrelease

import (
	"os"

	"golang.org/x/sys/windows"
)

// Windows maps a write-bit-free os.FileMode to FILE_ATTRIBUTE_READONLY.
// MoveFileEx cannot atomically replace such a destination even though its
// parent directory has the correct protected DACL. Keep the pointer writable
// by its trusted owner; the install-root ACL remains the authorization boundary.
func atomicPointerMode(test bool) os.FileMode {
	if test {
		return 0600
	}
	return 0644
}

func replaceAtomic(source, destination string) error {
	from, err := windows.UTF16PtrFromString(source)
	if err != nil {
		return err
	}
	to, err := windows.UTF16PtrFromString(destination)
	if err != nil {
		return err
	}
	return windows.MoveFileEx(from, to, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH)
}
