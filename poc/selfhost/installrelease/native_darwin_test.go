//go:build darwin

package installrelease

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"testing"

	"remoteapp.local/selfhost-poc/nativejournal"
)

type nativeActivation struct{ *nativejournal.Store }

func (n nativeActivation) Commit(expected, next []byte) error {
	digest := ""
	if expected != nil {
		digest = nativejournal.Digest(expected)
	}
	return n.Store.Commit(digest, next)
}

func TestNativeActivationUpgradeAndReopen(t *testing.T) {
	var scopeRaw [16]byte
	if _, err := rand.Read(scopeRaw[:]); err != nil {
		t.Fatal(err)
	}
	scope := hex.EncodeToString(scopeRaw[:])
	defer func() {
		if err := nativejournal.CleanupTest(scope); err != nil {
			t.Error("native cleanup", err)
		}
	}()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	releases := t.TempDir()
	channel := "native-activation-test"
	first, err := PromoteStaged(makeStage(t, releases, channel, "darwin", 1, private, 1), releases, public, channel, "darwin", 1)
	if err != nil {
		t.Fatal(err)
	}
	second, err := PromoteStaged(makeStage(t, releases, channel, "darwin", 2, private, 2), releases, public, channel, "darwin", 2)
	if err != nil {
		t.Fatal(err)
	}
	store, err := nativejournal.Open(scope, true, true)
	if err != nil {
		t.Fatal("native create", err)
	}
	if _, err = ActivateFirst(nativeActivation{store}, first); err != nil {
		_ = store.Close()
		t.Fatal("native first activation", err)
	}
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = nativejournal.Open(scope, false, true)
	if err != nil {
		t.Fatal("native reopen", err)
	}
	if opened, openErr := OpenActive(nativeActivation{store}, releases, public, channel, "darwin", 1); openErr != nil || opened.Manifest.Epoch != 1 {
		_ = store.Close()
		t.Fatal("native open active", openErr)
	}
	if _, err = ActivateUpgrade(nativeActivation{store}, second); err != nil {
		_ = store.Close()
		t.Fatal("native upgrade", err)
	}
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = nativejournal.Open(scope, false, true)
	if err != nil {
		t.Fatal("native final reopen", err)
	}
	opened, err := OpenActive(nativeActivation{store}, releases, public, channel, "darwin", 2)
	closeErr := store.Close()
	if err != nil || closeErr != nil || opened.Manifest.Epoch != 2 || opened.Directory != second.Directory {
		t.Fatal("native final active", err, closeErr)
	}
}
