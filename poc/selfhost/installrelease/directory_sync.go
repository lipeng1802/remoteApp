package installrelease

import "os"

// openDirectoryForSync opens the checked directory with the platform access
// required for a real flush. It does not suppress unsupported/denied flushes.
func openDirectoryForSync(path string) (*os.File, error) {
	before, err := os.Lstat(path)
	if err != nil || !before.IsDir() || before.Mode()&os.ModeSymlink != 0 {
		return nil, ErrRelease
	}
	f, err := platformOpenDirectoryForSync(path)
	if err != nil {
		return nil, ErrRelease
	}
	after, err := f.Stat()
	if err != nil || !after.IsDir() || !os.SameFile(before, after) {
		f.Close()
		return nil, ErrRelease
	}
	return f, nil
}
