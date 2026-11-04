package programremoval

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/autonomousbinding"
	"remoteapp.local/selfhost-poc/clientruntime"
	"remoteapp.local/selfhost-poc/installrelease"
	"remoteapp.local/selfhost-poc/trustinstall"
	"remoteapp.local/selfhost-poc/trustprofile"
)

// Real native credentials/leases, with no service, payload or deletion permit.
// Windows invocation must run on a logged-in desktop/Interactive task.
func TestNativeArmedRemovalReleasesBindingBeforePayloadLeaseAndJoinsSigner(t *testing.T) {
	helper, vault := os.Getenv("REMOTEAPP_JOURNAL_TEST_EXECUTABLE"), os.Getenv("REMOTEAPP_VAULT_TEST_EXECUTABLE")
	if helper == "" || vault == "" {
		t.Skip("explicit pinned native helpers required")
	}
	hraw, e := os.ReadFile(helper)
	if e != nil {
		t.Fatal("journal helper unavailable")
	}
	vraw, e := os.ReadFile(vault)
	if e != nil {
		t.Fatal("vault helper unavailable")
	}
	h, v := sha256.Sum256(hraw), sha256.Sum256(vraw)
	vd := hex.EncodeToString(v[:])
	r, _, _ := boundRelease(t)
	spec := r.Specification()
	spec.HelperDigest = hex.EncodeToString(h[:])
	encoded, e := trustinstall.EncodeForTrustedBuild(spec, time.Now())
	if e != nil {
		t.Fatal(e)
	}
	r, e = trustinstall.DecodeCompiled(encoded)
	if e != nil {
		t.Fatal(e)
	}
	var nonce [16]byte
	rand.Read(nonce[:])
	scope := hex.EncodeToString(nonce[:])
	created, e := clientruntime.CreateNativeIdentity(vault, vd, scope)
	if e != nil {
		t.Fatal("fresh native identity creation rejected")
	}
	public := hex.EncodeToString(created.PublicKey())
	if created.CloseJoined() != nil {
		t.Fatal("creator did not join")
	}
	t.Cleanup(func() {
		if clientruntime.DeleteNativeTestIdentity(vault, vd, scope, public) != nil {
			t.Error("exact TEST identity cleanup unconfirmed")
		}
	})
	rf, e := r.NativeRecordFactory(helper)
	if e != nil {
		t.Fatal(e)
	}
	records := func(s string, c, t bool) (autonomousbinding.Journal, error) { return rf(s, c, t) }
	installer, e := autonomousbinding.BeginWithJournal(r.Scope(), true, records)
	if e != nil {
		t.Fatal("fresh native binding creation rejected")
	}
	t.Cleanup(func() {
		if autonomousbinding.CleanupTestWith(r.Scope(), func(s string) error { return trustprofile.DeleteTestDesktopJournal(helper, s) }) != nil {
			t.Error("exact native binding cleanup unconfirmed")
		}
	})
	binding, e := installer.Commit(scope, public)
	if e != nil {
		installer.Close()
		t.Fatal("binding commit rejected")
	}
	record := binding.Record()
	if binding.Close() != nil {
		t.Fatal("installer binding did not join")
	}
	parent, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	root := filepath.Join(parent, "install")
	if _, e = installrelease.PrepareProtectedLayout(root, true); e != nil {
		t.Fatal(e)
	}
	factory, e := r.NativeFactory(helper)
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var payloadBinding *autonomousbinding.Store
	defer func() {
		if payloadBinding != nil {
			payloadBinding.Close()
		}
	}()
	_, e = ArmedAuthenticatedBoundTest(ctx, r, factory, records, root, "127.0.0.1:1", func(got autonomousbinding.Record) (BoundSigner, error) {
		if got != record {
			t.Fatal("binding identity changed")
		}
		s, e := clientruntime.NativeSigner(vault, vd, got.VaultScope, got.PublicKey)()
		if e != nil {
			return nil, e
		}
		return s.(BoundSigner), nil
	}, func(life context.Context) error {
		payloadBinding, e = autonomousbinding.OpenWithJournal(r.Scope(), true, records)
		if e != nil {
			t.Fatal("armed helper still holds native binding")
		}
		cancel()
		return life.Err()
	})
	if e == nil || payloadBinding == nil || payloadBinding.Check() != nil {
		t.Fatal("cancel or native binding preservation failed")
	}
	if _, e = os.Lstat(filepath.Join(root, installrelease.RemovalPlanName)); !os.IsNotExist(e) {
		t.Fatal("cancel wrote plan")
	}
	// A new process can reopen the exact key after the armed signer actually joined.
	probe, e := clientruntime.NativeSigner(vault, vd, scope, public)()
	if e != nil {
		t.Fatal("original identity not preserved")
	}
	if probe.(BoundSigner).CloseJoined() != nil {
		t.Fatal("native probe did not join")
	}
}
