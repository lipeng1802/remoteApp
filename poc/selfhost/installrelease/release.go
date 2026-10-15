// Package installrelease verifies and activates immutable installed-product
// release directories. Distribution signing and privileged directory ACLs are
// external responsibilities; this package never treats a checksum sidecar as
// publisher authentication.
package installrelease

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"remoteapp.local/selfhost-poc/privatepipe"
)

var ErrRelease = errors.New("installed_release_rejected")

const ManifestName = "release-manifest.json"
const maximumManifest = 16384
const maximumFile = int64(100 << 20)
const maximumTotal = int64(500 << 20)

type File struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

type Manifest struct {
	Version  int    `json:"version"`
	Layout   string `json:"layout"`
	Channel  string `json:"channel"`
	Platform string `json:"platform"`
	Epoch    uint64 `json:"epoch"`
	Files    []File `json:"files"`
}

type Signed struct {
	Manifest  Manifest `json:"manifest"`
	Signature string   `json:"signature"`
}

type Verified struct {
	Directory      string
	DirectoryName  string
	Manifest       Manifest
	ManifestDigest string
}

func ExpectedFiles(platform string) ([]string, error) {
	suffix := ""
	if platform == "windows" {
		suffix = ".exe"
	} else if platform != "darwin" {
		return nil, ErrRelease
	}
	return []string{
		"autonomous-enrollment-install" + suffix,
		"identity-vault" + suffix,
		"installed-autonomous-product" + suffix,
		"tls-product" + suffix,
		"trust-install" + suffix,
		"trust-journal" + suffix,
	}, nil
}

func canonicalHex(value string, size int) bool {
	raw, err := hex.DecodeString(value)
	return err == nil && len(raw) == size && hex.EncodeToString(raw) == value
}

func validChannel(channel string) bool {
	return len(channel) > 0 && len(channel) <= 64 && channel[0] != '-' && strings.Trim(channel, "abcdefghijklmnopqrstuvwxyz0123456789-") == ""
}

func validateManifest(m Manifest) error {
	expected, err := ExpectedFiles(m.Platform)
	if err != nil || m.Version != 1 || m.Layout != "autonomous-flat-v1" || !validChannel(m.Channel) || m.Epoch == 0 || m.Epoch > 1<<53-1 || len(m.Files) != len(expected) {
		return ErrRelease
	}
	total := int64(0)
	for i, file := range m.Files {
		if file.Name != expected[i] || !canonicalHex(file.SHA256, 32) || file.Size < 1 || file.Size > maximumFile || total > maximumTotal-file.Size {
			return ErrRelease
		}
		total += file.Size
	}
	return nil
}

func manifestMessage(m Manifest) ([]byte, error) {
	if err := validateManifest(m); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(m)
	if err != nil || len(raw) > maximumManifest {
		return nil, ErrRelease
	}
	hash := sha256.Sum256(append([]byte("remoteapp-installed-release-manifest-v1\n"), raw...))
	return hash[:], nil
}

// SignForBuild signs public release metadata. The caller owns and must erase
// the offline private key; this function never writes it or generates one.
func SignForBuild(private ed25519.PrivateKey, m Manifest) ([]byte, error) {
	message, err := manifestMessage(m)
	if err != nil || len(private) != ed25519.PrivateKeySize {
		return nil, ErrRelease
	}
	value := Signed{Manifest: m, Signature: hex.EncodeToString(ed25519.Sign(private, message))}
	raw, err := json.Marshal(value)
	if err != nil || len(raw) > maximumManifest {
		return nil, ErrRelease
	}
	return raw, nil
}

func decode(raw []byte) (Signed, error) {
	var value Signed
	var extra struct{}
	if len(raw) == 0 || len(raw) > maximumManifest {
		return value, ErrRelease
	}
	reader := privatepipe.NewBoundedReader(bytes.NewReader(append(append([]byte(nil), raw...), '\n')), maximumManifest+1)
	if reader.Read(&value) != nil || reader.Read(&extra) != io.EOF || validateManifest(value.Manifest) != nil || !canonicalHex(value.Signature, ed25519.SignatureSize) {
		return Signed{}, ErrRelease
	}
	canonical, err := json.Marshal(value)
	if err != nil || !bytes.Equal(raw, canonical) {
		return Signed{}, ErrRelease
	}
	return value, nil
}

func Verify(raw, root []byte, channel, platform string, minimumEpoch uint64) (Signed, string, error) {
	value, err := decode(raw)
	if err != nil || len(root) != ed25519.PublicKeySize || value.Manifest.Channel != channel || value.Manifest.Platform != platform || value.Manifest.Epoch < minimumEpoch || minimumEpoch == 0 {
		return Signed{}, "", ErrRelease
	}
	message, err := manifestMessage(value.Manifest)
	signature, sigErr := hex.DecodeString(value.Signature)
	if err != nil || sigErr != nil || !ed25519.Verify(ed25519.PublicKey(root), message, signature) {
		return Signed{}, "", ErrRelease
	}
	digest := sha256.Sum256(raw)
	return value, hex.EncodeToString(digest[:]), nil
}

func readRegular(path string, limit int64) ([]byte, error) {
	before, err := os.Lstat(path)
	if err != nil || !before.Mode().IsRegular() || before.Mode()&os.ModeSymlink != 0 || before.Size() < 1 || before.Size() > limit {
		return nil, ErrRelease
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, ErrRelease
	}
	defer file.Close()
	after, err := file.Stat()
	if err != nil || !after.Mode().IsRegular() || !os.SameFile(before, after) {
		return nil, ErrRelease
	}
	raw, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil || int64(len(raw)) > limit || int64(len(raw)) != after.Size() {
		return nil, ErrRelease
	}
	return raw, nil
}

func VerifyDirectory(directory string, root []byte, channel, platform string, minimumEpoch uint64) (Verified, error) {
	if !filepath.IsAbs(directory) || filepath.Clean(directory) != directory {
		return Verified{}, ErrRelease
	}
	info, err := os.Lstat(directory)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return Verified{}, ErrRelease
	}
	manifestRaw, err := readRegular(filepath.Join(directory, ManifestName), maximumManifest)
	if err != nil {
		return Verified{}, err
	}
	signed, digest, err := Verify(manifestRaw, root, channel, platform, minimumEpoch)
	if err != nil {
		return Verified{}, err
	}
	expected := append([]string{ManifestName}, mustExpected(platform)...)
	sort.Strings(expected)
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != len(expected) {
		return Verified{}, ErrRelease
	}
	for i, entry := range entries {
		if entry.Name() != expected[i] || entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			return Verified{}, ErrRelease
		}
	}
	for _, artifact := range signed.Manifest.Files {
		raw, err := readRegular(filepath.Join(directory, artifact.Name), maximumFile)
		if err != nil || int64(len(raw)) != artifact.Size {
			return Verified{}, ErrRelease
		}
		hash := sha256.Sum256(raw)
		if hex.EncodeToString(hash[:]) != artifact.SHA256 {
			return Verified{}, ErrRelease
		}
	}
	return Verified{Directory: directory, DirectoryName: filepath.Base(directory), Manifest: signed.Manifest, ManifestDigest: digest}, nil
}

func mustExpected(platform string) []string {
	files, err := ExpectedFiles(platform)
	if err != nil {
		panic(err)
	}
	return files
}

// BuildManifest reads only the fixed flat layout. It does not sign binaries or
// infer a platform from their contents.
func BuildManifest(directory, channel, platform string, epoch uint64) (Manifest, error) {
	files, err := ExpectedFiles(platform)
	if err != nil || !filepath.IsAbs(directory) || filepath.Clean(directory) != directory {
		return Manifest{}, ErrRelease
	}
	m := Manifest{Version: 1, Layout: "autonomous-flat-v1", Channel: channel, Platform: platform, Epoch: epoch}
	for _, name := range files {
		raw, err := readRegular(filepath.Join(directory, name), maximumFile)
		if err != nil {
			return Manifest{}, err
		}
		hash := sha256.Sum256(raw)
		m.Files = append(m.Files, File{Name: name, SHA256: hex.EncodeToString(hash[:]), Size: int64(len(raw))})
	}
	if validateManifest(m) != nil {
		return Manifest{}, ErrRelease
	}
	return m, nil
}

// ReleaseDirectoryName is content-addressed and epoch ordered. A caller may
// promote a verified staging directory to this immutable name, then commit an
// Activation through an atomic journal. Existing versions remain untouched.
func ReleaseDirectoryName(v Verified) (string, error) {
	if v.Manifest.Epoch == 0 || !canonicalHex(v.ManifestDigest, 32) {
		return "", ErrRelease
	}
	return fmt.Sprintf("%016x-%s", v.Manifest.Epoch, v.ManifestDigest[:16]), nil
}

// PromoteStaged moves a verified hidden sibling into its immutable,
// content-addressed name. This does not make it active; the activation journal
// is the only commit point, so a crash here can leave at most an unused release.
func PromoteStaged(staging, releasesRoot string, root []byte, channel, platform string, minimumEpoch uint64) (Verified, error) {
	if !filepath.IsAbs(staging) || !filepath.IsAbs(releasesRoot) || filepath.Clean(staging) != staging || filepath.Clean(releasesRoot) != releasesRoot || filepath.Dir(staging) != releasesRoot {
		return Verified{}, ErrRelease
	}
	base := filepath.Base(staging)
	if !strings.HasPrefix(base, ".staging-") || !canonicalHex(strings.TrimPrefix(base, ".staging-"), 16) {
		return Verified{}, ErrRelease
	}
	rootInfo, err := os.Lstat(releasesRoot)
	if err != nil || !rootInfo.IsDir() || rootInfo.Mode()&os.ModeSymlink != 0 {
		return Verified{}, ErrRelease
	}
	verified, err := VerifyDirectory(staging, root, channel, platform, minimumEpoch)
	if err != nil {
		return Verified{}, err
	}
	name, err := ReleaseDirectoryName(Verified{Manifest: verified.Manifest, ManifestDigest: verified.ManifestDigest})
	if err != nil {
		return Verified{}, err
	}
	destination := filepath.Join(releasesRoot, name)
	if _, err = os.Lstat(destination); !os.IsNotExist(err) {
		return Verified{}, ErrRelease
	}
	if err = os.Rename(staging, destination); err != nil {
		return Verified{}, ErrRelease
	}
	return VerifyDirectory(destination, root, channel, platform, minimumEpoch)
}
