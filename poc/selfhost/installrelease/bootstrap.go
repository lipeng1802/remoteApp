package installrelease

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"remoteapp.local/selfhost-poc/privatepipe"
)

const BootstrapManifestName = "bootstrap-manifest.json"
const BootstrapPointerName = "current.json"

type BootstrapManifest struct {
	Version  int    `json:"version"`
	Layout   string `json:"layout"`
	Channel  string `json:"channel"`
	Platform string `json:"platform"`
	Epoch    uint64 `json:"epoch"`
	Files    []File `json:"files"`
}

type SignedBootstrap struct {
	Manifest  BootstrapManifest `json:"manifest"`
	Signature string            `json:"signature"`
}

type VerifiedBootstrap struct {
	Directory      string
	DirectoryName  string
	Manifest       BootstrapManifest
	ManifestDigest string
}

type BootstrapPointer struct {
	Version        int    `json:"version"`
	Layout         string `json:"layout"`
	Channel        string `json:"channel"`
	Platform       string `json:"platform"`
	Epoch          uint64 `json:"epoch"`
	Directory      string `json:"directory"`
	ManifestSHA256 string `json:"manifest_sha256"`
}

func BootstrapExpectedFiles(platform string) ([]string, error) {
	suffix := ""
	if platform == "windows" {
		suffix = ".exe"
	} else if platform != "darwin" {
		return nil, ErrRelease
	}
	return []string{"installed-release-launcher" + suffix, "trust-journal" + suffix}, nil
}

func validateBootstrapManifest(manifest BootstrapManifest) error {
	expected, err := BootstrapExpectedFiles(manifest.Platform)
	if err != nil || manifest.Version != 1 || manifest.Layout != "bootstrap-set-v1" || !validChannel(manifest.Channel) || manifest.Epoch == 0 || manifest.Epoch > 1<<53-1 || len(manifest.Files) != len(expected) {
		return ErrRelease
	}
	for index, file := range manifest.Files {
		if file.Name != expected[index] || !canonicalHex(file.SHA256, 32) || file.Size < 1 || file.Size > maximumFile {
			return ErrRelease
		}
	}
	return nil
}

func bootstrapMessage(manifest BootstrapManifest) ([]byte, error) {
	if validateBootstrapManifest(manifest) != nil {
		return nil, ErrRelease
	}
	raw, err := json.Marshal(manifest)
	if err != nil || len(raw) > maximumManifest {
		return nil, ErrRelease
	}
	digest := sha256.Sum256(append([]byte("remoteapp-installed-bootstrap-manifest-v1\n"), raw...))
	return digest[:], nil
}

func SignBootstrapForBuild(private ed25519.PrivateKey, manifest BootstrapManifest) ([]byte, error) {
	message, err := bootstrapMessage(manifest)
	if err != nil || len(private) != ed25519.PrivateKeySize {
		return nil, ErrRelease
	}
	raw, err := json.Marshal(SignedBootstrap{Manifest: manifest, Signature: hex.EncodeToString(ed25519.Sign(private, message))})
	if err != nil || len(raw) > maximumManifest {
		return nil, ErrRelease
	}
	return raw, nil
}

func BuildBootstrapManifest(directory, channel, platform string, epoch uint64) (BootstrapManifest, error) {
	files, err := BootstrapExpectedFiles(platform)
	if err != nil || !filepath.IsAbs(directory) || filepath.Clean(directory) != directory {
		return BootstrapManifest{}, ErrRelease
	}
	manifest := BootstrapManifest{Version: 1, Layout: "bootstrap-set-v1", Channel: channel, Platform: platform, Epoch: epoch}
	for _, name := range files {
		raw, err := readRegular(filepath.Join(directory, name), maximumFile)
		if err != nil {
			return BootstrapManifest{}, err
		}
		digest := sha256.Sum256(raw)
		manifest.Files = append(manifest.Files, File{Name: name, SHA256: hex.EncodeToString(digest[:]), Size: int64(len(raw))})
	}
	if validateBootstrapManifest(manifest) != nil {
		return BootstrapManifest{}, ErrRelease
	}
	return manifest, nil
}

func VerifyBootstrapDirectory(directory string, root []byte, channel, platform string, minimumEpoch uint64) (VerifiedBootstrap, error) {
	if !filepath.IsAbs(directory) || filepath.Clean(directory) != directory || len(root) != ed25519.PublicKeySize || minimumEpoch == 0 {
		return VerifiedBootstrap{}, ErrRelease
	}
	raw, err := readRegular(filepath.Join(directory, BootstrapManifestName), maximumManifest)
	if err != nil {
		return VerifiedBootstrap{}, err
	}
	var signed SignedBootstrap
	var extra struct{}
	reader := privatepipe.NewBoundedReader(bytes.NewReader(append(append([]byte(nil), raw...), '\n')), maximumManifest+1)
	if reader.Read(&signed) != nil || reader.Read(&extra) != io.EOF || validateBootstrapManifest(signed.Manifest) != nil || !canonicalHex(signed.Signature, ed25519.SignatureSize) {
		return VerifiedBootstrap{}, ErrRelease
	}
	canonical, err := json.Marshal(signed)
	message, messageErr := bootstrapMessage(signed.Manifest)
	signature, signatureErr := hex.DecodeString(signed.Signature)
	if err != nil || !bytes.Equal(raw, canonical) || messageErr != nil || signatureErr != nil || !ed25519.Verify(ed25519.PublicKey(root), message, signature) || signed.Manifest.Channel != channel || signed.Manifest.Platform != platform || signed.Manifest.Epoch < minimumEpoch {
		return VerifiedBootstrap{}, ErrRelease
	}
	expected := append([]string{BootstrapManifestName}, mustBootstrapExpected(platform)...)
	sort.Strings(expected)
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != len(expected) {
		return VerifiedBootstrap{}, ErrRelease
	}
	for index, entry := range entries {
		if entry.Name() != expected[index] || entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			return VerifiedBootstrap{}, ErrRelease
		}
	}
	for _, file := range signed.Manifest.Files {
		content, err := readRegular(filepath.Join(directory, file.Name), maximumFile)
		digest := sha256.Sum256(content)
		if err != nil || int64(len(content)) != file.Size || hex.EncodeToString(digest[:]) != file.SHA256 {
			return VerifiedBootstrap{}, ErrRelease
		}
	}
	digest := sha256.Sum256(raw)
	return VerifiedBootstrap{Directory: directory, DirectoryName: filepath.Base(directory), Manifest: signed.Manifest, ManifestDigest: hex.EncodeToString(digest[:])}, nil
}

func mustBootstrapExpected(platform string) []string {
	files, err := BootstrapExpectedFiles(platform)
	if err != nil {
		panic(err)
	}
	return files
}

func BootstrapDirectoryName(verified VerifiedBootstrap) (string, error) {
	if verified.Manifest.Epoch == 0 || !canonicalHex(verified.ManifestDigest, 32) {
		return "", ErrRelease
	}
	return fmtDirectory(verified.Manifest.Epoch, verified.ManifestDigest[:16]), nil
}

func ImportBootstrapStaged(source, installRoot, stagingName string, root []byte, channel, platform string, minimumEpoch uint64, test bool) (VerifiedBootstrap, error) {
	if !filepath.IsAbs(source) || filepath.Clean(source) != source || len(stagingName) != len(".staging-")+32 || !strings.HasPrefix(stagingName, ".staging-") || !canonicalHex(strings.TrimPrefix(stagingName, ".staging-"), 16) {
		return VerifiedBootstrap{}, ErrRelease
	}
	sourceVerified, err := VerifyBootstrapDirectory(source, root, channel, platform, minimumEpoch)
	if err != nil {
		return VerifiedBootstrap{}, err
	}
	sets, err := ValidateProtectedBootstrapLayout(installRoot, test)
	if err != nil {
		return VerifiedBootstrap{}, err
	}
	destination := filepath.Join(sets, stagingName)
	directoryMode, manifestMode, executableMode := os.FileMode(0755), os.FileMode(0444), os.FileMode(0555)
	if test {
		directoryMode, manifestMode, executableMode = 0700, 0400, 0500
	}
	if filepath.Dir(destination) != sets || os.Mkdir(destination, directoryMode) != nil || platformProtectDirectory(destination, test) != nil {
		return VerifiedBootstrap{}, ErrRelease
	}
	for _, name := range append([]string{BootstrapManifestName}, mustBootstrapExpected(platform)...) {
		limit, mode := maximumFile, executableMode
		if name == BootstrapManifestName {
			limit, mode = maximumManifest, manifestMode
		}
		raw, err := readRegular(filepath.Join(source, name), limit)
		if err != nil {
			return VerifiedBootstrap{}, err
		}
		file, err := os.OpenFile(filepath.Join(destination, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
		if err != nil {
			return VerifiedBootstrap{}, ErrRelease
		}
		_, writeErr := file.Write(raw)
		syncErr, closeErr := file.Sync(), file.Close()
		if writeErr != nil || syncErr != nil || closeErr != nil {
			return VerifiedBootstrap{}, ErrRelease
		}
	}
	directory, err := os.Open(destination)
	if err != nil {
		return VerifiedBootstrap{}, ErrRelease
	}
	syncErr, closeErr := directory.Sync(), directory.Close()
	parent, err := os.Open(sets)
	if err != nil {
		return VerifiedBootstrap{}, ErrRelease
	}
	parentSync, parentClose := parent.Sync(), parent.Close()
	if syncErr != nil || closeErr != nil || parentSync != nil || parentClose != nil || ValidateProtectedBootstrapSet(destination, installRoot, test) != nil {
		return VerifiedBootstrap{}, ErrRelease
	}
	verified, err := VerifyBootstrapDirectory(destination, root, channel, platform, minimumEpoch)
	if err != nil || verified.ManifestDigest != sourceVerified.ManifestDigest || verified.Manifest.Epoch != sourceVerified.Manifest.Epoch {
		return VerifiedBootstrap{}, ErrRelease
	}
	return verified, nil
}

func PromoteBootstrapStaged(staging, installRoot string, root []byte, channel, platform string, minimumEpoch uint64, test bool) (VerifiedBootstrap, error) {
	sets, err := ValidateProtectedBootstrapLayout(installRoot, test)
	if err != nil || filepath.Dir(staging) != sets || ValidateProtectedBootstrapSet(staging, installRoot, test) != nil {
		return VerifiedBootstrap{}, ErrRelease
	}
	verified, err := VerifyBootstrapDirectory(staging, root, channel, platform, minimumEpoch)
	if err != nil {
		return VerifiedBootstrap{}, err
	}
	name, err := BootstrapDirectoryName(verified)
	if err != nil {
		return VerifiedBootstrap{}, err
	}
	destination := filepath.Join(sets, name)
	if _, err = os.Lstat(destination); !os.IsNotExist(err) || os.Rename(staging, destination) != nil {
		return VerifiedBootstrap{}, ErrRelease
	}
	if ValidateProtectedBootstrapSet(destination, installRoot, test) != nil {
		return VerifiedBootstrap{}, ErrRelease
	}
	return VerifyBootstrapDirectory(destination, root, channel, platform, minimumEpoch)
}

func ReconcileBootstrapCandidate(source, installRoot, stagingName, expectedDigest string, expectedEpoch uint64, root []byte, channel, platform string, minimumEpoch uint64, test bool) (VerifiedBootstrap, string, error) {
	if !canonicalHex(expectedDigest, 32) || expectedEpoch == 0 || len(stagingName) != len(".staging-")+32 || !strings.HasPrefix(stagingName, ".staging-") || !canonicalHex(strings.TrimPrefix(stagingName, ".staging-"), 16) {
		return VerifiedBootstrap{}, "", ErrRelease
	}
	sourceVerified, err := VerifyBootstrapDirectory(source, root, channel, platform, minimumEpoch)
	if err != nil || sourceVerified.ManifestDigest != expectedDigest || sourceVerified.Manifest.Epoch != expectedEpoch {
		return VerifiedBootstrap{}, "", ErrRelease
	}
	sets, err := ValidateProtectedBootstrapLayout(installRoot, test)
	if err != nil {
		return VerifiedBootstrap{}, "", err
	}
	stage := filepath.Join(sets, stagingName)
	name, err := BootstrapDirectoryName(sourceVerified)
	if err != nil {
		return VerifiedBootstrap{}, "", err
	}
	promoted := filepath.Join(sets, name)
	stageInfo, stageErr := os.Lstat(stage)
	promotedInfo, promotedErr := os.Lstat(promoted)
	stageExists, promotedExists := stageErr == nil, promotedErr == nil
	if (stageErr != nil && !os.IsNotExist(stageErr)) || (promotedErr != nil && !os.IsNotExist(promotedErr)) || (stageExists && promotedExists) {
		return VerifiedBootstrap{}, "", ErrRelease
	}
	if stageExists {
		if !stageInfo.IsDir() || stageInfo.Mode()&os.ModeSymlink != 0 || ValidateProtectedBootstrapSet(stage, installRoot, test) != nil {
			return VerifiedBootstrap{}, "", ErrRelease
		}
		verified, err := VerifyBootstrapDirectory(stage, root, channel, platform, minimumEpoch)
		if err != nil || verified.ManifestDigest != expectedDigest || verified.Manifest.Epoch != expectedEpoch {
			return VerifiedBootstrap{}, "", ErrRelease
		}
		return verified, CandidateStaged, nil
	}
	if promotedExists {
		if !promotedInfo.IsDir() || promotedInfo.Mode()&os.ModeSymlink != 0 || ValidateProtectedBootstrapSet(promoted, installRoot, test) != nil {
			return VerifiedBootstrap{}, "", ErrRelease
		}
		verified, err := VerifyBootstrapDirectory(promoted, root, channel, platform, minimumEpoch)
		if err != nil || verified.ManifestDigest != expectedDigest || verified.Manifest.Epoch != expectedEpoch {
			return VerifiedBootstrap{}, "", ErrRelease
		}
		return verified, CandidatePromoted, nil
	}
	verified, err := ImportBootstrapStaged(source, installRoot, stagingName, root, channel, platform, minimumEpoch, test)
	if err != nil {
		return VerifiedBootstrap{}, "", err
	}
	return verified, CandidateStaged, nil
}

func CleanupBootstrapPointerTest(installRoot string, expected VerifiedBootstrap, root []byte, channel, platform string) error {
	if _, err := ValidateProtectedBootstrapLayout(installRoot, true); err != nil {
		return err
	}
	pointer := filepath.Join(installRoot, "bootstrap", BootstrapPointerName)
	active, err := OpenBootstrap(installRoot, root, channel, platform, 1, true)
	if err != nil || active.ManifestDigest != expected.ManifestDigest || active.Manifest.Epoch != expected.Manifest.Epoch || os.Remove(pointer) != nil {
		return ErrRelease
	}
	return nil
}

func CleanupVerifiedBootstrapSet(installRoot string, candidate VerifiedBootstrap, root []byte, channel, platform string) error {
	if _, err := os.Lstat(filepath.Join(installRoot, "bootstrap", BootstrapPointerName)); !os.IsNotExist(err) || ValidateProtectedBootstrapSet(candidate.Directory, installRoot, true) != nil {
		return ErrRelease
	}
	verified, err := VerifyBootstrapDirectory(candidate.Directory, root, channel, platform, 1)
	if err != nil || verified.ManifestDigest != candidate.ManifestDigest || verified.Manifest.Epoch != candidate.Manifest.Epoch {
		return ErrRelease
	}
	for _, name := range append([]string{BootstrapManifestName}, mustBootstrapExpected(platform)...) {
		if os.Remove(filepath.Join(candidate.Directory, name)) != nil {
			return ErrRelease
		}
	}
	if os.Remove(candidate.Directory) != nil {
		return ErrRelease
	}
	return nil
}

func CleanupBootstrapTestState(installRoot string, candidates []VerifiedBootstrap, root []byte, channel, platform string) error {
	if len(candidates) == 0 || CleanupBootstrapPointerTest(installRoot, candidates[0], root, channel, platform) != nil {
		return ErrRelease
	}
	for _, candidate := range candidates {
		if CleanupVerifiedBootstrapSet(installRoot, candidate, root, channel, platform) != nil {
			return ErrRelease
		}
	}
	return nil
}

func pointerFor(verified VerifiedBootstrap) (BootstrapPointer, error) {
	name, err := BootstrapDirectoryName(verified)
	if err != nil || verified.DirectoryName != name {
		return BootstrapPointer{}, ErrRelease
	}
	return BootstrapPointer{Version: 1, Layout: "bootstrap-set-v1", Channel: verified.Manifest.Channel, Platform: verified.Manifest.Platform, Epoch: verified.Manifest.Epoch, Directory: name, ManifestSHA256: verified.ManifestDigest}, nil
}

func validBootstrapPointer(pointer BootstrapPointer) bool {
	return pointer.Version == 1 && pointer.Layout == "bootstrap-set-v1" && validChannel(pointer.Channel) && (pointer.Platform == "darwin" || pointer.Platform == "windows") && pointer.Epoch > 0 && pointer.Epoch <= 1<<53-1 && canonicalHex(pointer.ManifestSHA256, 32) && pointer.Directory == fmtDirectory(pointer.Epoch, pointer.ManifestSHA256[:16])
}

func readBootstrapPointer(path string, test bool) (BootstrapPointer, []byte, error) {
	raw, err := readRegular(path, 2048)
	if err != nil {
		return BootstrapPointer{}, nil, err
	}
	file, _, err := openChecked(path, false, test)
	if err != nil {
		return BootstrapPointer{}, nil, err
	}
	file.Close()
	var pointer BootstrapPointer
	var extra struct{}
	reader := privatepipe.NewBoundedReader(bytes.NewReader(append(append([]byte(nil), raw...), '\n')), 2049)
	if reader.Read(&pointer) != nil || reader.Read(&extra) != io.EOF || !validBootstrapPointer(pointer) {
		return BootstrapPointer{}, nil, ErrRelease
	}
	canonical, err := json.Marshal(pointer)
	if err != nil || !bytes.Equal(canonical, raw) {
		return BootstrapPointer{}, nil, ErrRelease
	}
	return pointer, raw, nil
}

func OpenBootstrap(installRoot string, root []byte, channel, platform string, minimumEpoch uint64, test bool) (VerifiedBootstrap, error) {
	sets, err := ValidateProtectedBootstrapLayout(installRoot, test)
	if err != nil {
		return VerifiedBootstrap{}, err
	}
	pointer, _, err := readBootstrapPointer(filepath.Join(installRoot, "bootstrap", BootstrapPointerName), test)
	if err != nil || pointer.Channel != channel || pointer.Platform != platform || pointer.Epoch < minimumEpoch {
		return VerifiedBootstrap{}, ErrRelease
	}
	directory := filepath.Join(sets, pointer.Directory)
	if ValidateProtectedBootstrapSet(directory, installRoot, test) != nil {
		return VerifiedBootstrap{}, ErrRelease
	}
	verified, err := VerifyBootstrapDirectory(directory, root, channel, platform, minimumEpoch)
	if err != nil || verified.ManifestDigest != pointer.ManifestSHA256 || verified.Manifest.Epoch != pointer.Epoch {
		return VerifiedBootstrap{}, ErrRelease
	}
	return verified, nil
}

func ActivateBootstrap(installRoot, stagingNonce string, candidate VerifiedBootstrap, operation string, root []byte, test bool) error {
	if operation != "install" && operation != "upgrade" || !canonicalHex(stagingNonce, 16) || ValidateProtectedBootstrapSet(candidate.Directory, installRoot, test) != nil {
		return ErrRelease
	}
	next, err := pointerFor(candidate)
	if err != nil {
		return err
	}
	currentPath := filepath.Join(installRoot, "bootstrap", BootstrapPointerName)
	currentInfo, currentStatErr := os.Lstat(currentPath)
	if currentStatErr == nil {
		if currentInfo.IsDir() || currentInfo.Mode()&os.ModeSymlink != 0 {
			return ErrRelease
		}
		current, _, currentErr := readBootstrapPointer(currentPath, test)
		if currentErr != nil {
			return ErrRelease
		}
		opened, openErr := OpenBootstrap(installRoot, root, current.Channel, current.Platform, 1, test)
		if openErr != nil || opened.ManifestDigest != current.ManifestSHA256 || opened.Manifest.Epoch != current.Epoch {
			return ErrRelease
		}
		if current == next {
			return nil
		}
		if operation != "upgrade" || current.Channel != next.Channel || current.Platform != next.Platform || next.Epoch <= current.Epoch || current.ManifestSHA256 == next.ManifestSHA256 {
			return ErrRelease
		}
	} else if !os.IsNotExist(currentStatErr) || operation != "install" {
		return ErrRelease
	}
	raw, err := json.Marshal(next)
	if err != nil {
		return ErrRelease
	}
	temporary := filepath.Join(installRoot, "bootstrap", ".current-"+stagingNonce)
	temporaryInfo, temporaryStatErr := os.Lstat(temporary)
	if temporaryStatErr == nil {
		if temporaryInfo.IsDir() || temporaryInfo.Mode()&os.ModeSymlink != 0 {
			return ErrRelease
		}
		existing, _, existingErr := readBootstrapPointer(temporary, test)
		if existingErr != nil {
			return ErrRelease
		}
		if existing != next {
			return ErrRelease
		}
	} else {
		if !os.IsNotExist(temporaryStatErr) {
			return ErrRelease
		}
		mode := atomicPointerMode(test)
		file, createErr := os.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
		if createErr != nil {
			return ErrRelease
		}
		_, writeErr := file.Write(raw)
		syncErr, closeErr := file.Sync(), file.Close()
		if writeErr != nil || syncErr != nil || closeErr != nil {
			return ErrRelease
		}
	}
	if replaceAtomic(temporary, currentPath) != nil {
		return ErrRelease
	}
	directory, err := os.Open(filepath.Dir(currentPath))
	if err != nil {
		return ErrRelease
	}
	syncErr, closeErr := directory.Sync(), directory.Close()
	verified, openErr := OpenBootstrap(installRoot, root, next.Channel, next.Platform, next.Epoch, test)
	if syncErr != nil || closeErr != nil || openErr != nil || verified.ManifestDigest != next.ManifestSHA256 {
		return ErrRelease
	}
	return nil
}
