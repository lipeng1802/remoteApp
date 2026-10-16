package installrelease

import (
	"os"
	"path/filepath"
	"strings"
)

type fileIdentity struct {
	volume uint64
	index  uint64
	links  uint64
}

func openChecked(path string, directory, test bool) (*os.File, fileIdentity, error) {
	before, err := os.Lstat(path)
	if err != nil || before.Mode()&os.ModeSymlink != 0 || before.IsDir() != directory || (!directory && !before.Mode().IsRegular()) {
		return nil, fileIdentity{}, ErrRelease
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fileIdentity{}, ErrRelease
	}
	after, err := file.Stat()
	if err != nil || !os.SameFile(before, after) || after.IsDir() != directory || (!directory && !after.Mode().IsRegular()) {
		file.Close()
		return nil, fileIdentity{}, ErrRelease
	}
	identity, err := platformIdentity(path, file, after, directory, test)
	if err != nil || (!directory && identity.links != 1) {
		file.Close()
		return nil, fileIdentity{}, ErrRelease
	}
	return file, identity, nil
}

// ValidateProtectedLayout requires an absolute, non-link installation root and
// its direct releases child to be protected by the current platform and reside
// on one volume. The caller still owns product code-signing and elevation.
func ValidateProtectedLayout(installRoot string, test bool) (string, error) {
	if !filepath.IsAbs(installRoot) || filepath.Clean(installRoot) != installRoot {
		return "", ErrRelease
	}
	root, rootIdentity, err := openChecked(installRoot, true, test)
	if err != nil {
		return "", err
	}
	defer root.Close()
	releases := filepath.Join(installRoot, "releases")
	releaseDir, releaseIdentity, err := openChecked(releases, true, test)
	if err != nil {
		return "", err
	}
	defer releaseDir.Close()
	if rootIdentity.volume != releaseIdentity.volume {
		return "", ErrRelease
	}
	return releases, nil
}

// ValidateProtectedRelease binds one direct child directory to the already
// protected releases root and rejects ownership, permission, link or volume
// changes. Content authenticity is still supplied by VerifyDirectory.
func ValidateProtectedRelease(directory, installRoot string, test bool) error {
	releases, err := ValidateProtectedLayout(installRoot, test)
	if err != nil || !filepath.IsAbs(directory) || filepath.Clean(directory) != directory || filepath.Dir(directory) != releases {
		return ErrRelease
	}
	parent, parentIdentity, err := openChecked(releases, true, test)
	if err != nil {
		return err
	}
	parent.Close()
	child, childIdentity, err := openChecked(directory, true, test)
	if err != nil {
		return err
	}
	child.Close()
	if parentIdentity.volume != childIdentity.volume {
		return ErrRelease
	}
	return nil
}

// ImportStaged copies one already-signed external payload into a newly-created
// hidden directory under the protected releases root. The destination is thus
// on the promotion filesystem and every file is a fresh single-link inode.
func ImportStaged(source, installRoot, stagingName string, root []byte, channel, platform string, minimumEpoch uint64, test bool) (Verified, error) {
	if !filepath.IsAbs(source) || filepath.Clean(source) != source || len(stagingName) != len(".staging-")+32 || !strings.HasPrefix(stagingName, ".staging-") || !canonicalHex(strings.TrimPrefix(stagingName, ".staging-"), 16) {
		return Verified{}, ErrRelease
	}
	sourceVerified, err := VerifyDirectory(source, root, channel, platform, minimumEpoch)
	if err != nil {
		return Verified{}, err
	}
	releases, err := ValidateProtectedLayout(installRoot, test)
	if err != nil {
		return Verified{}, err
	}
	destination := filepath.Join(releases, stagingName)
	directoryMode := os.FileMode(0755)
	manifestMode, executableMode := os.FileMode(0444), os.FileMode(0555)
	if test {
		directoryMode, manifestMode, executableMode = 0700, 0400, 0500
	}
	if filepath.Dir(destination) != releases || os.Mkdir(destination, directoryMode) != nil || platformProtectDirectory(destination, test) != nil {
		return Verified{}, ErrRelease
	}
	names := append([]string{ManifestName}, mustExpected(platform)...)
	for _, name := range names {
		limit, mode := maximumFile, executableMode
		if name == ManifestName {
			limit, mode = maximumManifest, manifestMode
		}
		raw, err := readRegular(filepath.Join(source, name), limit)
		if err != nil {
			return Verified{}, err
		}
		file, err := os.OpenFile(filepath.Join(destination, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
		if err != nil {
			return Verified{}, ErrRelease
		}
		_, writeErr := file.Write(raw)
		syncErr := file.Sync()
		closeErr := file.Close()
		if writeErr != nil || syncErr != nil || closeErr != nil {
			return Verified{}, ErrRelease
		}
	}
	directory, err := os.Open(destination)
	if err != nil {
		return Verified{}, ErrRelease
	}
	syncErr, closeErr := directory.Sync(), directory.Close()
	parent, parentErr := os.Open(releases)
	if parentErr != nil {
		return Verified{}, ErrRelease
	}
	parentSync, parentClose := parent.Sync(), parent.Close()
	if syncErr != nil || closeErr != nil || parentSync != nil || parentClose != nil {
		return Verified{}, ErrRelease
	}
	if _, err = ValidateProtectedLayout(installRoot, test); err != nil {
		return Verified{}, err
	}
	if err = ValidateProtectedRelease(destination, installRoot, test); err != nil {
		return Verified{}, err
	}
	verified, err := VerifyDirectory(destination, root, channel, platform, minimumEpoch)
	if err != nil || verified.ManifestDigest != sourceVerified.ManifestDigest || verified.Manifest.Epoch != sourceVerified.Manifest.Epoch {
		return Verified{}, ErrRelease
	}
	return verified, nil
}
