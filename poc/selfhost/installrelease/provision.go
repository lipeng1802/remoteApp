package installrelease

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
)

// LayoutReceipt is a process-local installer handoff receipt. It contains no
// path or credential, and is useful only when immediately reopened and checked
// by ValidateLayoutReceipt. Publisher authentication remains code signing.
type LayoutReceipt struct {
	Version        int    `json:"version"`
	Status         string `json:"status"`
	Platform       string `json:"platform"`
	Policy         string `json:"policy"`
	Test           bool   `json:"test"`
	RootSHA256     string `json:"root_sha256"`
	Volume         string `json:"volume"`
	RootObject     string `json:"root_object"`
	ReleasesObject string `json:"releases_object"`
}

func pathDigest(path string) string {
	digest := sha256.Sum256([]byte("remoteapp-install-root-path-v1\n" + path))
	return hex.EncodeToString(digest[:])
}

func layoutReceipt(installRoot string, test bool) (LayoutReceipt, error) {
	releases, err := ValidateProtectedLayout(installRoot, test)
	if err != nil {
		return LayoutReceipt{}, err
	}
	root, rootIdentity, err := openChecked(installRoot, true, test)
	if err != nil {
		return LayoutReceipt{}, err
	}
	defer root.Close()
	directory, releasesIdentity, err := openChecked(releases, true, test)
	if err != nil {
		return LayoutReceipt{}, err
	}
	defer directory.Close()
	if rootIdentity.volume != releasesIdentity.volume {
		return LayoutReceipt{}, ErrRelease
	}
	return LayoutReceipt{
		Version:        1,
		Status:         "prepared",
		Platform:       runtime.GOOS,
		Policy:         platformPolicy(test),
		Test:           test,
		RootSHA256:     pathDigest(installRoot),
		Volume:         hex.EncodeToString(uint64Bytes(rootIdentity.volume)),
		RootObject:     hex.EncodeToString(uint64Bytes(rootIdentity.index)),
		ReleasesObject: hex.EncodeToString(uint64Bytes(releasesIdentity.index)),
	}, nil
}

func uint64Bytes(value uint64) []byte {
	return []byte{byte(value >> 56), byte(value >> 48), byte(value >> 40), byte(value >> 32), byte(value >> 24), byte(value >> 16), byte(value >> 8), byte(value)}
}

func ensureDirectory(path string, test bool) error {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		if os.Mkdir(path, 0700) != nil {
			return ErrRelease
		}
	} else if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return ErrRelease
	}
	if platformProtectDirectory(path, test) != nil {
		return ErrRelease
	}
	directory, _, err := openChecked(path, true, test)
	if err != nil {
		return err
	}
	return directory.Close()
}

// PrepareProtectedLayout is the privileged installer's only directory recipe.
// It never creates missing parents and never accepts a relative or linked root.
func PrepareProtectedLayout(installRoot string, test bool) (LayoutReceipt, error) {
	if !filepath.IsAbs(installRoot) || filepath.Clean(installRoot) != installRoot || filepath.Dir(installRoot) == installRoot {
		return LayoutReceipt{}, ErrRelease
	}
	parentPath := filepath.Dir(installRoot)
	if !test {
		resolved, err := filepath.EvalSymlinks(parentPath)
		if err != nil || resolved != parentPath {
			return LayoutReceipt{}, ErrRelease
		}
	}
	parent, parentIdentity, err := openChecked(parentPath, true, test)
	if err != nil {
		return LayoutReceipt{}, err
	}
	parent.Close()
	if err = ensureDirectory(installRoot, test); err != nil {
		return LayoutReceipt{}, err
	}
	root, rootIdentity, err := openChecked(installRoot, true, test)
	if err != nil || parentIdentity.volume != rootIdentity.volume {
		if root != nil {
			root.Close()
		}
		return LayoutReceipt{}, ErrRelease
	}
	root.Close()
	if err = ensureDirectory(filepath.Join(installRoot, "releases"), test); err != nil {
		return LayoutReceipt{}, err
	}
	return layoutReceipt(installRoot, test)
}

// ValidateLayoutReceipt reopens the fixed root; a copied or stale receipt for
// another filesystem object is rejected.
func ValidateLayoutReceipt(receipt LayoutReceipt, installRoot string, test bool) error {
	current, err := layoutReceipt(installRoot, test)
	if err != nil || !reflect.DeepEqual(receipt, current) {
		return ErrRelease
	}
	return nil
}

// CleanupProtectedTestLayout removes only the exact empty TEST layout named by
// a still-current receipt. Production layouts and nonempty roots are refused.
func CleanupProtectedTestLayout(receipt LayoutReceipt, installRoot string) error {
	if !receipt.Test || ValidateLayoutReceipt(receipt, installRoot, true) != nil {
		return ErrRelease
	}
	releases := filepath.Join(installRoot, "releases")
	entries, err := os.ReadDir(releases)
	if err != nil || len(entries) != 0 {
		return ErrRelease
	}
	entries, err = os.ReadDir(installRoot)
	if err != nil || len(entries) != 1 || entries[0].Name() != "releases" || !entries[0].IsDir() {
		return ErrRelease
	}
	if os.Remove(releases) != nil || os.Remove(installRoot) != nil {
		return ErrRelease
	}
	return nil
}
