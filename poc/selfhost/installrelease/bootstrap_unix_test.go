//go:build !windows

package installrelease

import (
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"
)

func bootstrapFixture(t *testing.T, private ed25519.PrivateKey, channel, platform string, epoch uint64, marker byte) (string, VerifiedBootstrap) {
	t.Helper()
	directory := t.TempDir()
	for index, name := range mustBootstrapExpected(platform) {
		if err := os.WriteFile(filepath.Join(directory, name), []byte{marker, byte(index + 1)}, 0700); err != nil {
			t.Fatal(err)
		}
	}
	manifest, err := BuildBootstrapManifest(directory, channel, platform, epoch)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := SignBootstrapForBuild(private, manifest)
	if err != nil || os.WriteFile(filepath.Join(directory, BootstrapManifestName), raw, 0600) != nil {
		t.Fatal("sign bootstrap")
	}
	verified, err := VerifyBootstrapDirectory(directory, private.Public().(ed25519.PublicKey), channel, platform, 1)
	if err != nil {
		t.Fatal(err)
	}
	return directory, verified
}

func TestBootstrapSetAtomicActivationAndOldSetFollowsCurrent(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "install")
	if _, err = PrepareProtectedLayout(root, true); err != nil {
		t.Fatal(err)
	}
	channel := "bootstrap-test"
	source1, original1 := bootstrapFixture(t, private, channel, "darwin", 1, 1)
	source2, original2 := bootstrapFixture(t, private, channel, "darwin", 2, 2)
	staging1 := ".staging-11111111111111111111111111111111"
	staging2 := ".staging-22222222222222222222222222222222"
	first, state, err := ReconcileBootstrapCandidate(source1, root, staging1, original1.ManifestDigest, 1, public, channel, "darwin", 1, true)
	if err != nil || state != CandidateStaged {
		t.Fatal("first bootstrap import", state, err)
	}
	first, err = PromoteBootstrapStaged(first.Directory, root, public, channel, "darwin", 1, true)
	if err != nil || ActivateBootstrap(root, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", first, "install", public, true) != nil {
		t.Fatal("first bootstrap activation", err)
	}
	second, state, err := ReconcileBootstrapCandidate(source2, root, staging2, original2.ManifestDigest, 2, public, channel, "darwin", 2, true)
	if err != nil || state != CandidateStaged {
		t.Fatal("second bootstrap import", state, err)
	}
	second, err = PromoteBootstrapStaged(second.Directory, root, public, channel, "darwin", 2, true)
	if err != nil || ActivateBootstrap(root, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", second, "upgrade", public, true) != nil {
		t.Fatal("second bootstrap activation", err)
	}
	active, err := OpenBootstrap(root, public, channel, "darwin", 1, true)
	if err != nil || active.ManifestDigest != second.ManifestDigest || active.Manifest.Epoch != 2 {
		t.Fatal("old floor did not follow current bootstrap", err)
	}
	if ActivateBootstrap(root, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", second, "upgrade", public, true) != nil {
		t.Fatal("exact activation was not idempotent")
	}
	if ActivateBootstrap(root, "cccccccccccccccccccccccccccccccc", first, "upgrade", public, true) == nil {
		t.Fatal("bootstrap rollback accepted")
	}
	if CleanupBootstrapTestState(root, []VerifiedBootstrap{second, first}, public, channel, "darwin") != nil {
		t.Fatal("bootstrap cleanup")
	}
	receipt, err := layoutReceipt(root, true)
	if err != nil || CleanupProtectedTestLayout(receipt, root) != nil {
		t.Fatal("layout cleanup", err)
	}
}
