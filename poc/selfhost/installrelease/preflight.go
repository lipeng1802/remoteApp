package installrelease

import (
	"errors"
	"os"
	"path/filepath"
)

var errPreflightPath = errors.New("test_install_path_invalid")
var errPreflightExists = errors.New("test_install_root_not_fresh")
var errPreflightParent = errors.New("test_install_parent_unprotected")

// CheckFreshTestInstallRoot only observes the candidate's parent and absence.
// It creates no files, repairs no ACL and opens no native credential store.
// This is not a reservation or a durable receipt: setup must recheck everything
// at the actual mutation boundary, even after this check succeeds.
func CheckFreshTestInstallRoot(root string) error {
	if !filepath.IsAbs(root) || filepath.Clean(root) != root || filepath.Dir(root) == root {
		return errors.Join(ErrRelease, errPreflightPath)
	}
	if _, err := os.Lstat(root); !os.IsNotExist(err) {
		return errors.Join(ErrRelease, errPreflightExists)
	}
	parent, _, err := openChecked(filepath.Dir(root), true, true)
	if err != nil {
		return errors.Join(ErrRelease, errPreflightParent)
	}
	if err = parent.Close(); err != nil {
		return errors.Join(ErrRelease, errPreflightParent)
	}
	return nil
}

// InstallPreflightReason is fixed, non-secret diagnostic output only.
func InstallPreflightReason(err error) string {
	for _, known := range []error{errPreflightPath, errPreflightExists, errPreflightParent} {
		if errors.Is(err, known) {
			return known.Error()
		}
	}
	return ""
}
