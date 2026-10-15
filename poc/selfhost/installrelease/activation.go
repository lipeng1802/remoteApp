package installrelease

import (
	"bytes"
	"encoding/json"
	"io"
	"path/filepath"

	"remoteapp.local/selfhost-poc/privatepipe"
)

type Journal interface {
	Load() ([]byte, error)
	Commit(expected, next []byte) error
	Close() error
}

type Activation struct {
	Version        int    `json:"version"`
	Layout         string `json:"layout"`
	Channel        string `json:"channel"`
	Platform       string `json:"platform"`
	Epoch          uint64 `json:"epoch"`
	Directory      string `json:"directory"`
	ManifestSHA256 string `json:"manifest_sha256"`
}

func activationFor(v Verified) (Activation, error) {
	directory, err := ReleaseDirectoryName(v)
	if err != nil || v.DirectoryName != directory {
		return Activation{}, ErrRelease
	}
	return Activation{Version: 1, Layout: v.Manifest.Layout, Channel: v.Manifest.Channel, Platform: v.Manifest.Platform, Epoch: v.Manifest.Epoch, Directory: directory, ManifestSHA256: v.ManifestDigest}, nil
}

func validActivation(a Activation) bool {
	if a.Version != 1 || a.Layout != "autonomous-flat-v1" || !validChannel(a.Channel) || (a.Platform != "darwin" && a.Platform != "windows") || a.Epoch == 0 || a.Epoch > 1<<53-1 || !canonicalHex(a.ManifestSHA256, 32) {
		return false
	}
	return a.Directory == formatDirectory(a.Epoch, a.ManifestSHA256)
}

func formatDirectory(epoch uint64, digest string) string {
	if len(digest) < 16 {
		return ""
	}
	return fmtDirectory(epoch, digest[:16])
}

func fmtDirectory(epoch uint64, prefix string) string {
	const hexDigits = "0123456789abcdef"
	buf := make([]byte, 16)
	for i := 15; i >= 0; i-- {
		buf[i] = hexDigits[epoch&15]
		epoch >>= 4
	}
	return string(buf) + "-" + prefix
}

func encodeActivation(a Activation) ([]byte, error) {
	if !validActivation(a) {
		return nil, ErrRelease
	}
	raw, err := json.Marshal(a)
	if err != nil || len(raw) > 1024 {
		return nil, ErrRelease
	}
	return raw, nil
}

func decodeActivation(raw []byte) (Activation, error) {
	var a Activation
	var extra struct{}
	reader := privatepipe.NewBoundedReader(bytes.NewReader(append(append([]byte(nil), raw...), '\n')), 1025)
	if len(raw) == 0 || len(raw) > 1024 || reader.Read(&a) != nil || reader.Read(&extra) != io.EOF || !validActivation(a) {
		return Activation{}, ErrRelease
	}
	canonical, err := json.Marshal(a)
	if err != nil || !bytes.Equal(raw, canonical) {
		return Activation{}, ErrRelease
	}
	return a, nil
}

func commit(j Journal, previous []byte, next Activation) error {
	if j == nil {
		return ErrRelease
	}
	raw, err := encodeActivation(next)
	if err != nil || j.Commit(previous, raw) != nil {
		return ErrRelease
	}
	check, err := j.Load()
	if err != nil || !bytes.Equal(check, raw) {
		return ErrRelease
	}
	return nil
}

func ActivateFirst(j Journal, candidate Verified) (Activation, error) {
	next, err := activationFor(candidate)
	if err != nil || commit(j, nil, next) != nil {
		return Activation{}, ErrRelease
	}
	return next, nil
}

func ActivateUpgrade(j Journal, candidate Verified) (Activation, error) {
	if j == nil {
		return Activation{}, ErrRelease
	}
	previousRaw, err := j.Load()
	previous, decodeErr := decodeActivation(previousRaw)
	next, nextErr := activationFor(candidate)
	if err != nil || decodeErr != nil || nextErr != nil || next.Channel != previous.Channel || next.Platform != previous.Platform || next.Layout != previous.Layout || next.Epoch <= previous.Epoch || next.ManifestSHA256 == previous.ManifestSHA256 {
		return Activation{}, ErrRelease
	}
	if commit(j, previousRaw, next) != nil {
		return Activation{}, ErrRelease
	}
	return next, nil
}

// OpenActive treats the activation record only as a pointer. The signed
// manifest and every artifact are reverified before a launcher may use them.
func OpenActive(j Journal, releasesRoot string, root []byte, channel, platform string, minimumEpoch uint64) (Verified, error) {
	if j == nil || !filepath.IsAbs(releasesRoot) || filepath.Clean(releasesRoot) != releasesRoot {
		return Verified{}, ErrRelease
	}
	raw, err := j.Load()
	active, decodeErr := decodeActivation(raw)
	if err != nil || decodeErr != nil || active.Channel != channel || active.Platform != platform || active.Epoch < minimumEpoch {
		return Verified{}, ErrRelease
	}
	directory := filepath.Join(releasesRoot, active.Directory)
	if filepath.Dir(directory) != releasesRoot {
		return Verified{}, ErrRelease
	}
	verified, err := VerifyDirectory(directory, root, channel, platform, minimumEpoch)
	if err != nil || verified.ManifestDigest != active.ManifestSHA256 || verified.Manifest.Epoch != active.Epoch {
		return Verified{}, ErrRelease
	}
	return verified, nil
}
