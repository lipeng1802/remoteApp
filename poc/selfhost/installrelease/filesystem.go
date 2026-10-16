package installrelease

import (
	"os"
	"path/filepath"
	"strings"
)

const CandidateStaged = "staged"
const CandidatePromoted = "promoted"

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

// ReconcileCandidate is the crash-reopen boundary between an installer bundle
// and the activation manager. Exactly one of the fixed hidden staging path or
// its content-addressed promoted path may exist. Neither causes a fresh import;
// both or any digest/epoch mismatch fail closed.
func ReconcileCandidate(source, installRoot, stagingName, expectedDigest string, expectedEpoch uint64, root []byte, channel, platform string, minimumEpoch uint64, test bool) (Verified, string, error) {
	if len(stagingName) != len(".staging-")+32 || !strings.HasPrefix(stagingName, ".staging-") || !canonicalHex(strings.TrimPrefix(stagingName, ".staging-"), 16) || !canonicalHex(expectedDigest, 32) || expectedEpoch == 0 || expectedEpoch > 1<<53-1 {
		return Verified{}, "", ErrRelease
	}
	sourceVerified, err := VerifyDirectory(source, root, channel, platform, minimumEpoch)
	if err != nil || sourceVerified.ManifestDigest != expectedDigest || sourceVerified.Manifest.Epoch != expectedEpoch {
		return Verified{}, "", ErrRelease
	}
	releases, err := ValidateProtectedLayout(installRoot, test)
	if err != nil {
		return Verified{}, "", err
	}
	stage := filepath.Join(releases, stagingName)
	name, err := ReleaseDirectoryName(sourceVerified)
	if err != nil {
		return Verified{}, "", err
	}
	promoted := filepath.Join(releases, name)
	stageInfo, stageErr := os.Lstat(stage)
	promotedInfo, promotedErr := os.Lstat(promoted)
	stageExists, promotedExists := stageErr == nil, promotedErr == nil
	if (stageErr != nil && !os.IsNotExist(stageErr)) || (promotedErr != nil && !os.IsNotExist(promotedErr)) || (stageExists && promotedExists) {
		return Verified{}, "", ErrRelease
	}
	if stageExists {
		if !stageInfo.IsDir() || stageInfo.Mode()&os.ModeSymlink != 0 {
			return Verified{}, "", ErrRelease
		}
		verified, err := VerifyDirectory(stage, root, channel, platform, minimumEpoch)
		if err != nil || verified.ManifestDigest != expectedDigest || verified.Manifest.Epoch != expectedEpoch || ValidateProtectedRelease(stage, installRoot, test) != nil {
			return Verified{}, "", ErrRelease
		}
		return verified, CandidateStaged, nil
	}
	if promotedExists {
		if !promotedInfo.IsDir() || promotedInfo.Mode()&os.ModeSymlink != 0 {
			return Verified{}, "", ErrRelease
		}
		verified, err := VerifyDirectory(promoted, root, channel, platform, minimumEpoch)
		if err != nil || verified.ManifestDigest != expectedDigest || verified.Manifest.Epoch != expectedEpoch || ValidateProtectedRelease(promoted, installRoot, test) != nil {
			return Verified{}, "", ErrRelease
		}
		return verified, CandidatePromoted, nil
	}
	verified, err := ImportStaged(source, installRoot, stagingName, root, channel, platform, minimumEpoch, test)
	if err != nil || verified.ManifestDigest != expectedDigest || verified.Manifest.Epoch != expectedEpoch {
		return Verified{}, "", ErrRelease
	}
	return verified, CandidateStaged, nil
}

// CleanupVerifiedTestRelease removes one exact inactive TEST release. Callers
// must separately prove that the activation record is absent; production use
// is intentionally unavailable.
func CleanupVerifiedTestRelease(directory, installRoot, expectedDigest string, expectedEpoch uint64, root []byte, channel, platform string, minimumEpoch uint64, test bool) error {
	if !test || !canonicalHex(expectedDigest, 32) || expectedEpoch == 0 || ValidateProtectedRelease(directory, installRoot, true) != nil {
		return ErrRelease
	}
	verified, err := VerifyDirectory(directory, root, channel, platform, minimumEpoch)
	if err != nil || verified.ManifestDigest != expectedDigest || verified.Manifest.Epoch != expectedEpoch {
		return ErrRelease
	}
	for _, name := range append([]string{ManifestName}, mustExpected(platform)...) {
		path := filepath.Join(directory, name)
		file, _, err := openChecked(path, false, true)
		if err != nil || file.Close() != nil || os.Remove(path) != nil {
			return ErrRelease
		}
	}
	if os.Remove(directory) != nil {
		return ErrRelease
	}
	parent, err := os.Open(filepath.Dir(directory))
	if err != nil {
		return ErrRelease
	}
	syncErr, closeErr := parent.Sync(), parent.Close()
	if syncErr != nil || closeErr != nil {
		return ErrRelease
	}
	return nil
}
