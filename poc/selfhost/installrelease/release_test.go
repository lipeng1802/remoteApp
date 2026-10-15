package installrelease

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

type memoryJournal struct {
	raw       []byte
	fail      bool
	uncertain bool
}

func (m *memoryJournal) Load() ([]byte, error) {
	if m.fail && !m.uncertain {
		return nil, ErrRelease
	}
	return append([]byte(nil), m.raw...), nil
}
func (m *memoryJournal) Commit(expected, next []byte) error {
	if m.fail && !m.uncertain || !bytes.Equal(expected, m.raw) {
		return ErrRelease
	}
	m.raw = append([]byte(nil), next...)
	if m.fail {
		return ErrRelease
	}
	return nil
}
func (m *memoryJournal) Close() error { return nil }

func makeStage(t *testing.T, root, channel, platform string, epoch uint64, key ed25519.PrivateKey, marker byte) string {
	t.Helper()
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(root, ".staging-"+hex.EncodeToString(nonce[:]))
	if err := os.Mkdir(directory, 0700); err != nil {
		t.Fatal(err)
	}
	files, _ := ExpectedFiles(platform)
	for i, name := range files {
		if err := os.WriteFile(filepath.Join(directory, name), []byte{marker, byte(i + 1)}, 0700); err != nil {
			t.Fatal(err)
		}
	}
	manifest, err := BuildManifest(directory, channel, platform, epoch)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := SignForBuild(key, manifest)
	if err != nil || os.WriteFile(filepath.Join(directory, ManifestName), raw, 0600) != nil {
		t.Fatal("manifest")
	}
	return directory
}

func TestSignedImmutableReleaseActivationAndUpgrade(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	channel := "stable-autonomous"
	stage1 := makeStage(t, root, channel, "darwin", 1, private, 1)
	first, err := PromoteStaged(stage1, root, public, channel, "darwin", 1)
	if err != nil || first.DirectoryName == filepath.Base(stage1) {
		t.Fatal("promote first", err)
	}
	journal := &memoryJournal{}
	active1, err := ActivateFirst(journal, first)
	if err != nil || active1.Epoch != 1 {
		t.Fatal("activate first", err)
	}
	opened, err := OpenActive(journal, root, public, channel, "darwin", 1)
	if err != nil || opened.Directory != first.Directory {
		t.Fatal("open first", err)
	}
	stage2 := makeStage(t, root, channel, "darwin", 2, private, 2)
	second, err := PromoteStaged(stage2, root, public, channel, "darwin", 2)
	if err != nil {
		t.Fatal("promote second", err)
	}
	active2, err := ActivateUpgrade(journal, second)
	if err != nil || active2.Epoch != 2 || active2.Directory == active1.Directory {
		t.Fatal("activate second", err)
	}
	if _, err = ActivateUpgrade(journal, first); err == nil {
		t.Fatal("rollback accepted")
	}
	opened, err = OpenActive(journal, root, public, channel, "darwin", 2)
	if err != nil || opened.Directory != second.Directory {
		t.Fatal("open second", err)
	}
	if _, err = os.Stat(first.Directory); err != nil {
		t.Fatal("old immutable release removed")
	}
	if err = os.WriteFile(filepath.Join(second.Directory, "tls-product"), []byte("tampered"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err = OpenActive(journal, root, public, channel, "darwin", 2); err == nil {
		t.Fatal("tampered active release accepted")
	}
}

func TestActivationScopeIsCanonicalStableAndSeparated(t *testing.T) {
	input := "0123456789abcdef0123456789abcdef"
	first, err := Scope(input)
	if err != nil || len(first) != 32 || first == input {
		t.Fatal("invalid activation scope", first, err)
	}
	second, err := Scope(input)
	if err != nil || second != first {
		t.Fatal("unstable activation scope", second, err)
	}
	other, err := Scope("1123456789abcdef0123456789abcdef")
	if err != nil || other == first {
		t.Fatal("scope collision", other, err)
	}
	for _, bad := range []string{"", "ABCDEF0123456789ABCDEF0123456789", "0123", input + "00"} {
		if _, err = Scope(bad); err == nil {
			t.Fatal("invalid release scope accepted", bad)
		}
	}
}

func TestManifestAndActivationFailuresStayClosed(t *testing.T) {
	public, private, _ := ed25519.GenerateKey(rand.Reader)
	otherPublic, _, _ := ed25519.GenerateKey(rand.Reader)
	root := t.TempDir()
	channel := "stable-autonomous"
	stage := makeStage(t, root, channel, "windows", 3, private, 3)
	manifestRaw, err := os.ReadFile(filepath.Join(stage, ManifestName))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = Verify(manifestRaw, public, "other-channel", "windows", 1); err == nil {
		t.Fatal("wrong channel accepted")
	}
	if _, _, err = Verify(manifestRaw, public, channel, "windows", 4); err == nil {
		t.Fatal("epoch below floor accepted")
	}
	if _, _, err = Verify(append(manifestRaw, ' '), public, channel, "windows", 1); err == nil {
		t.Fatal("noncanonical signed envelope accepted")
	}
	if _, err := VerifyDirectory(stage, otherPublic, channel, "windows", 1); err == nil {
		t.Fatal("wrong signer accepted")
	}
	if err := os.WriteFile(filepath.Join(stage, "extra"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyDirectory(stage, public, channel, "windows", 1); err == nil {
		t.Fatal("extra file accepted")
	}
	if err := os.Remove(filepath.Join(stage, "extra")); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(stage, "trust-journal.exe")
	if err := os.Rename(missing, missing+".missing"); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyDirectory(stage, public, channel, "windows", 1); err == nil {
		t.Fatal("missing file accepted")
	}
	if err := os.Rename(missing+".missing", missing); err != nil {
		t.Fatal(err)
	}
	verified, err := PromoteStaged(stage, root, public, channel, "windows", 1)
	if err != nil {
		t.Fatal(err)
	}
	journal := &memoryJournal{fail: true}
	if _, err = ActivateFirst(journal, verified); err == nil || len(journal.raw) != 0 {
		t.Fatal("failed commit selected release")
	}
	journal = &memoryJournal{fail: true, uncertain: true}
	if _, err = ActivateFirst(journal, verified); err == nil || len(journal.raw) == 0 {
		t.Fatal("uncertain commit not reported")
	}
	if _, err = OpenActive(journal, root, public, channel, "windows", 1); err != nil {
		t.Fatal("explicit reopen could not resolve committed receipt", err)
	}
	journal.raw = append(journal.raw, ' ')
	if _, err = OpenActive(journal, root, public, channel, "windows", 1); err == nil {
		t.Fatal("noncanonical activation accepted")
	}
}
