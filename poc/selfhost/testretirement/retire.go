// Package testretirement explicitly retires an OFFLINE, idle TEST installation.
// This deletes test identities/floors and is never production program removal.
package testretirement

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"remoteapp.local/selfhost-poc/autonomousbinding"
	"remoteapp.local/selfhost-poc/clientruntime"
	"remoteapp.local/selfhost-poc/enrollmentstore"
	"remoteapp.local/selfhost-poc/installrelease"
	"remoteapp.local/selfhost-poc/trustinstall"
	"remoteapp.local/selfhost-poc/trustprofile"
	"runtime"
	"strings"
	"time"
)

// Idle verifies historical trust without Current(), expiry bypass or session
// startup. All native records must exist; unknown/partial outcomes stay closed.
// The caller and pinned helpers must live outside the isolated root.
func Idle(r trustinstall.Release, root, helper, vault, vaultDigest string) (result error) {
	return retire(r, root, helper, vault, vaultDigest, nil)
}

// Registered requires live issuer proof that this exact native node's grant
// was explicitly revoked and its remote provision already cleaned. It never
// accepts an operator-supplied grant/node/identity or treats expiry as cleanup.
func Registered(ctx context.Context, r trustinstall.Release, root, helper, vault, vaultDigest string, tunnel bool) error {
	if ctx == nil || ctx.Err() != nil {
		return errOnlineRetirement
	}
	return retire(r, root, helper, vault, vaultDigest, func(profile trustprofile.Profile, identity autonomousbinding.Record, node enrollmentstore.Record) error {
		return confirmNodeRetirement(ctx, r, profile, identity, node, vault, vaultDigest, tunnel)
	})
}

func retire(r trustinstall.Release, root, helper, vault, vaultDigest string, confirm func(trustprofile.Profile, autonomousbinding.Record, enrollmentstore.Record) error) (result error) {
	stage := "preflight"
	defer func() {
		if result != nil {
			result = fmt.Errorf("test_retirement_%s", stage)
		}
	}()
	if !r.Specification().Test || !filepath.IsAbs(root) || filepath.Clean(root) != root {
		return trustprofile.ErrTrust
	}
	exe, e := os.Executable()
	if e != nil {
		return e
	}
	for _, p := range []string{exe, helper, vault} {
		rel, e := filepath.Rel(root, p)
		if e != nil || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
			return trustprofile.ErrTrust
		}
	}
	factory, e := r.NativeFactory(helper)
	stage = "trust_lease"
	if e != nil {
		return e
	}
	trust, e := factory(false)
	if e != nil {
		return e
	}
	defer trust.Close()
	installation, e := trustprofile.OpenInstallation(r.Anchor(), trust, time.Now)
	if e != nil {
		return e
	}
	records, e := r.NativeRecordFactory(helper)
	stage = "binding_idle"
	if confirm != nil {
		stage = "binding_registered"
	}
	if e != nil {
		return e
	}
	binding, e := autonomousbinding.OpenWithJournal(r.Scope(), true, func(s string, c, t bool) (autonomousbinding.Journal, error) { return records(s, c, t) })
	if e != nil {
		return e
	}
	defer binding.Close()
	if e = binding.Check(); e != nil {
		return e
	}
	identity := binding.Record()
	node, e := enrollmentstore.OpenWithJournal(identity.EnrollmentScope, true, func(s string, c, t bool) (enrollmentstore.Journal, error) { return records(s, c, t) })
	if e != nil {
		return e
	}
	defer node.Close()
	wantPhase := "idle"
	if confirm != nil {
		wantPhase = "registered"
	}
	if e = node.Check(); e != nil || node.State().Phase != wantPhase {
		return trustprofile.ErrTrust
	}
	pub, _ := hex.DecodeString(r.Specification().Root)
	s := r.Specification()
	activationScope, e := installrelease.Scope(r.Scope())
	stage = "signed_inventory"
	if e != nil {
		return e
	}
	activation, e := records(activationScope, false, true)
	if e != nil {
		return e
	}
	defer activation.Close()
	release, e := installrelease.OpenActive(activation, filepath.Join(root, "releases"), pub, s.Channel, runtime.GOOS, s.MinimumEpoch)
	if e != nil {
		return e
	}
	bootstrap, e := installrelease.OpenBootstrap(root, pub, s.Channel, runtime.GOOS, s.MinimumEpoch, true)
	if e != nil {
		return e
	}
	for path, count := range map[string]int{root: 2, filepath.Join(root, "releases"): 1, filepath.Join(root, "bootstrap"): 2, filepath.Join(root, "bootstrap", "sets"): 1} {
		entries, e := os.ReadDir(path)
		if e != nil || len(entries) != count {
			return trustprofile.ErrTrust
		}
	}
	if confirm != nil {
		stage = "online_node_retirement"
		profile, _, e := installation.Current()
		if e != nil {
			return e
		}
		if e = confirm(profile, identity, node.State()); e != nil {
			return e
		}
	}
	cleanup := func(scope string) error { return trustprofile.DeleteTestDesktopJournal(helper, scope) }
	// Exclusive trust + binding + idle enrollment leases prohibit an active
	// owner. No external kill, online revoke or automatic repair is attempted.
	stage = "enrollment_delete"
	if e = node.CleanupTestAfterJoinWith(cleanup); e != nil {
		return e
	}
	stage = "identity_delete"
	if e = clientruntime.DeleteNativeTestIdentity(vault, vaultDigest, identity.VaultScope, identity.PublicKey); e != nil {
		return e
	}
	stage = "binding_delete"
	if e = binding.Close(); e != nil {
		return e
	}
	if e = autonomousbinding.CleanupTestWith(r.Scope(), cleanup); e != nil {
		return e
	}
	stage = "activation_delete"
	if e = activation.Close(); e != nil {
		return e
	}
	if e = cleanup(activationScope); e != nil {
		return e
	}
	stage = "release_delete"
	if e = installrelease.CleanupVerifiedTestRelease(release.Directory, root, release.ManifestDigest, release.Manifest.Epoch, pub, s.Channel, runtime.GOOS, s.MinimumEpoch, true); e != nil {
		return e
	}
	stage = "bootstrap_delete"
	if e = installrelease.CleanupBootstrapTestState(root, []installrelease.VerifiedBootstrap{bootstrap}, pub, s.Channel, runtime.GOOS); e != nil {
		return e
	}
	stage = "trust_delete"
	if e = trust.Close(); e != nil {
		return e
	}
	if e = cleanup(r.Scope()); e != nil {
		return e
	}
	// Only empty known shells: unknown files prevent removal, never recurse.
	stage = "empty_shells"
	for _, p := range []string{filepath.Join(root, "bootstrap", "sets"), filepath.Join(root, "bootstrap"), filepath.Join(root, "releases"), root} {
		if e = os.Remove(p); e != nil {
			return e
		}
	}
	return nil
}

// FinishFiles is explicit recovery after the release directory was removed
// but its directory-sync receipt failed. It never retries identity deletion.
// Only absent binding/activation, empty releases and intact signed bootstrap
// are accepted. Any partial release inventory or live lease stays rejected.
func FinishFiles(r trustinstall.Release, root, helper string) error {
	if !r.Specification().Test {
		return trustprofile.ErrTrust
	}
	f, e := r.NativeFactory(helper)
	if e != nil {
		return e
	}
	j, e := f(false)
	if e != nil {
		return e
	}
	defer j.Close()
	if _, e = trustprofile.OpenInstallation(r.Anchor(), j, time.Now); e != nil {
		return e
	}
	states, e := r.NativeRecordState(helper)
	if e != nil {
		return e
	}
	binding, e := autonomousbinding.Scope(r.Scope())
	if e != nil {
		return e
	}
	activation, e := installrelease.Scope(r.Scope())
	if e != nil {
		return e
	}
	for _, scope := range []string{binding, activation} {
		s, e := states(scope, true)
		if e != nil || s != trustprofile.JournalAbsent {
			return trustprofile.ErrTrust
		}
	}
	files, e := os.ReadDir(filepath.Join(root, "releases"))
	if e != nil || len(files) != 0 {
		return trustprofile.ErrTrust
	}
	s := r.Specification()
	pub, _ := hex.DecodeString(s.Root)
	b, e := installrelease.OpenBootstrap(root, pub, s.Channel, runtime.GOOS, s.MinimumEpoch, true)
	if e != nil {
		return e
	}
	if e = installrelease.CleanupBootstrapTestState(root, []installrelease.VerifiedBootstrap{b}, pub, s.Channel, runtime.GOOS); e != nil {
		return e
	}
	if e = j.Close(); e != nil {
		return e
	}
	if e = trustprofile.DeleteTestDesktopJournal(helper, r.Scope()); e != nil {
		return e
	}
	for _, p := range []string{filepath.Join(root, "bootstrap", "sets"), filepath.Join(root, "bootstrap"), filepath.Join(root, "releases"), root} {
		if e = os.Remove(p); e != nil {
			return e
		}
	}
	return nil
}
