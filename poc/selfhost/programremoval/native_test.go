package programremoval

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/clientowner"
	"remoteapp.local/selfhost-poc/clientruntime"
	"remoteapp.local/selfhost-poc/installrelease"
	"remoteapp.local/selfhost-poc/privatepipe"
	"remoteapp.local/selfhost-poc/trustinstall"
	"remoteapp.local/selfhost-poc/trustprofile"
)

func nativeFixture(t *testing.T) (trustinstall.Release, trustinstall.Factory, string, []byte, installrelease.Verified) {
	t.Helper()
	helper := os.Getenv("REMOTEAPP_JOURNAL_TEST_EXECUTABLE")
	if helper == "" {
		t.Skip("native journal helper required")
	}
	raw, err := os.ReadFile(helper)
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(raw)
	public, key, _ := ed25519.GenerateKey(rand.Reader)
	cp, ck, _ := ed25519.GenerateKey(rand.Reader)
	issuer, _, _ := ed25519.GenerateKey(rand.Reader)
	now := time.Now()
	ca := &x509.Certificate{SerialNumber: big.NewInt(1), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(24 * time.Hour)}
	der, err := x509.CreateCertificate(rand.Reader, ca, ca, cp, ck)
	if err != nil {
		t.Fatal(err)
	}
	claims := trustprofile.Claims{Version: 1, Service: "removal-test", Epoch: 1, Endpoint: "https://removal.example:8444", Certificate: base64.StdEncoding.EncodeToString(der), Issuer: hex.EncodeToString(issuer), IssuedAt: now.Add(-time.Minute).Unix(), ExpiresAt: now.Add(time.Hour).Unix()}
	p, _ := trustprofile.Sign(key, claims)
	var random [8]byte
	rand.Read(random[:])
	spec := trustinstall.Specification{Version: 1, Channel: "removal-" + hex.EncodeToString(random[:]), Root: hex.EncodeToString(public), Service: claims.Service, MinimumEpoch: 1, HelperDigest: hex.EncodeToString(h[:]), Bundled: p, Test: true}
	encoded, err := trustinstall.EncodeForTrustedBuild(spec, now)
	if err != nil {
		t.Fatal(err)
	}
	r, err := trustinstall.DecodeCompiled(encoded)
	if err != nil {
		t.Fatal(err)
	}
	factory, err := r.NativeFactory(helper)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if e := trustprofile.DeleteTestDesktopJournal(helper, r.Scope()); e != nil {
			t.Error(e)
		}
	})
	offline, _ := trustinstall.NewOfflineOwner(r, factory, time.Now)
	if _, err = offline.Initialize(); err != nil {
		t.Fatal(err)
	}
	claims.Epoch = 2
	next, _ := trustprofile.Sign(key, claims)
	if _, err = offline.Upgrade(context.Background(), next); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "install")
	if _, err = installrelease.PrepareProtectedLayout(root, true); err != nil {
		t.Fatal(err)
	}
	var latest installrelease.Verified
	var boot installrelease.VerifiedBootstrap
	for epoch := uint64(1); epoch <= 2; epoch++ {
		source := t.TempDir()
		files, _ := installrelease.ExpectedFiles(runtime.GOOS)
		for _, name := range files {
			if err = os.WriteFile(filepath.Join(source, name), []byte{byte(epoch), 3}, 0500); err != nil {
				t.Fatal(err)
			}
		}
		m, e := installrelease.BuildManifest(source, spec.Channel, runtime.GOOS, epoch)
		if e != nil {
			t.Fatal(e)
		}
		raw, _ := installrelease.SignForBuild(key, m)
		if err = os.WriteFile(filepath.Join(source, installrelease.ManifestName), raw, 0400); err != nil {
			t.Fatal(err)
		}
		nonce := strings.Repeat("1", 32)
		if epoch == 2 {
			nonce = strings.Repeat("2", 32)
		}
		verifiedSource, e := installrelease.VerifyDirectory(source, public, spec.Channel, runtime.GOOS, 1)
		if e != nil {
			t.Fatal(e)
		}
		latest, _, err = installrelease.ReconcileCandidate(source, root, ".staging-"+nonce, verifiedSource.ManifestDigest, epoch, public, spec.Channel, runtime.GOOS, 1, true)
		if err != nil {
			latest, _, err = installrelease.ReconcileCandidate(source, root, ".staging-"+nonce, verifiedSource.ManifestDigest, epoch, public, spec.Channel, runtime.GOOS, 1, true)
		}
		if err != nil {
			t.Fatal("import", err)
		}
		latest, err = installrelease.PromoteStaged(latest.Directory, filepath.Join(root, "releases"), public, spec.Channel, runtime.GOOS, 1)
		if err != nil {
			t.Fatal(err)
		}
		bs := t.TempDir()
		bfiles, _ := installrelease.BootstrapExpectedFiles(runtime.GOOS)
		for _, name := range bfiles {
			os.WriteFile(filepath.Join(bs, name), []byte{byte(epoch), 4}, 0500)
		}
		bm, e := installrelease.BuildBootstrapManifest(bs, spec.Channel, runtime.GOOS, epoch)
		if e != nil {
			t.Fatal(e)
		}
		raw, _ = installrelease.SignBootstrapForBuild(key, bm)
		os.WriteFile(filepath.Join(bs, installrelease.BootstrapManifestName), raw, 0400)
		verifiedBoot, e := installrelease.VerifyBootstrapDirectory(bs, public, spec.Channel, runtime.GOOS, 1)
		if e != nil {
			t.Fatal(e)
		}
		boot, _, err = installrelease.ReconcileBootstrapCandidate(bs, root, ".staging-"+nonce, verifiedBoot.ManifestDigest, epoch, public, spec.Channel, runtime.GOOS, 1, true)
		if err != nil {
			boot, _, err = installrelease.ReconcileBootstrapCandidate(bs, root, ".staging-"+nonce, verifiedBoot.ManifestDigest, epoch, public, spec.Channel, runtime.GOOS, 1, true)
		}
		if err != nil {
			t.Fatal(err)
		}
		boot, err = installrelease.PromoteBootstrapStaged(boot.Directory, root, public, spec.Channel, runtime.GOOS, 1, true)
		if err != nil {
			t.Fatal(err)
		}
	}
	if err = installrelease.ActivateBootstrap(root, strings.Repeat("3", 32), boot, "install", public, true); err != nil {
		opened, e := installrelease.OpenBootstrap(root, public, spec.Channel, runtime.GOOS, 1, true)
		if e != nil || opened.ManifestDigest != boot.ManifestDigest {
			t.Fatal("bootstrap receipt", err)
		}
	}
	o, err := clientowner.Open(r, factory, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	if err = o.MaintainDurable(context.Background(), func(context.Context, uint64) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if err = o.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	return r, factory, root, public, latest
}

// Actual process crash after a real unlink, deliberately bypassing journal
// Close. Only public random TEST fixture metadata is passed via environment.
func TestRemovalCrashFixture(t *testing.T) {
	encoded := os.Getenv("REMOTEAPP_REMOVAL_CRASH_RELEASE")
	if encoded == "" {
		return
	}
	r, err := trustinstall.DecodeCompiled(encoded)
	if err != nil || !r.Specification().Test {
		t.Fatal("invalid TEST release")
	}
	factory, err := r.NativeFactory(os.Getenv("REMOTEAPP_JOURNAL_TEST_EXECUTABLE"))
	if err != nil {
		t.Fatal(err)
	}
	j, err := factory(false)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := trustprofile.OpenRemovalTransaction(r.Anchor(), j, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	s, _ := tx.State()
	root := os.Getenv("REMOTEAPP_REMOVAL_CRASH_ROOT")
	public, _ := hex.DecodeString(r.Specification().Root)
	p, digest, err := installrelease.LoadRemovalTest(root, "", true)
	if err != nil || p.Transaction != s.Transaction || p.Epoch != s.Epoch || installrelease.ValidateRemovalTest(root, p, public, r.Specification().Channel, runtime.GOOS, false, false, true) != nil {
		t.Fatal("fresh pre-bind checkpoint", err)
	}
	if err = tx.BindRemovalPlan(digest); err != nil {
		t.Fatal(err)
	}
	done, err := installrelease.RemoveProgramBatchTest(context.Background(), root, p, public, r.Specification().Channel, runtime.GOOS, 1, true)
	if (err != nil && err != installrelease.ErrRemovalReceipt) || done {
		t.Fatal("partial delete", err)
	}
	os.Exit(78)
}

func TestNativeRemovalCrashResumeRetainsIdentityAndFloor(t *testing.T) {
	r, factory, root, public, latest := nativeFixture(t)
	vault := os.Getenv("REMOTEAPP_VAULT_TEST_EXECUTABLE")
	if vault == "" {
		t.Fatal("native vault helper required")
	}
	raw, err := os.ReadFile(vault)
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(raw)
	digest := hex.EncodeToString(h[:])
	scope := r.Scope()
	signer, err := clientruntime.CreateNativeIdentity(vault, digest, scope)
	if err != nil {
		t.Fatal(err)
	}
	pub := hex.EncodeToString(signer.PublicKey())
	if err = signer.CloseJoined(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if e := clientruntime.DeleteNativeTestIdentity(vault, digest, scope, pub); e != nil {
			t.Error(e)
		}
	})
	activationScope, _ := installrelease.Scope(r.Scope())
	records, err := r.NativeRecordFactory(os.Getenv("REMOTEAPP_JOURNAL_TEST_EXECUTABLE"))
	if err != nil {
		t.Fatal(err)
	}
	activation, err := records(activationScope, true, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = installrelease.ActivateFirst(activation, latest); err != nil {
		t.Fatal(err)
	}
	originalActivation, err := activation.Load()
	if err != nil {
		t.Fatal(err)
	}
	activation.Close()
	t.Cleanup(func() {
		if e := trustprofile.DeleteTestDesktopJournal(os.Getenv("REMOTEAPP_JOURNAL_TEST_EXECUTABLE"), activationScope); e != nil {
			t.Error(e)
		}
	})
	j, err := factory(false)
	if err != nil {
		t.Fatal(err)
	}
	originalTrust, err := j.Load()
	if err != nil {
		t.Fatal(err)
	}
	before, err := trustprofile.ReadMaintenance(r.Anchor(), j, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	j.Close()
	prepared, e := installrelease.CaptureRemovalTest(root, public, r.Specification().Channel, runtime.GOOS, before.Transaction, 2, true)
	if e != nil {
		t.Fatal(e)
	}
	if e = installrelease.PersistRemovalTest(root, prepared, true); e != nil && e != installrelease.ErrRemovalReceipt {
		t.Fatal(e)
	}
	encoded, _ := trustinstall.EncodeForTrustedBuild(r.Specification(), time.Now())
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestRemovalCrashFixture$")
	cmd.Env = append(os.Environ(), "REMOTEAPP_REMOVAL_CRASH_RELEASE="+encoded, "REMOTEAPP_REMOVAL_CRASH_ROOT="+root)
	out, err := cmd.CombinedOutput()
	var exited *exec.ExitError
	if !errors.As(err, &exited) || exited.ExitCode() != 78 {
		t.Fatalf("crash fixture %v %s", err, out)
	}
	// Wait only for the abandoned helper to observe inherited pipe EOF and
	// release its actual native lease; never force-release or replace it.
	deadline := time.Now().Add(5 * time.Second)
	for {
		j, err = factory(false)
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("crash lease not released", err)
		}
		time.Sleep(30 * time.Millisecond)
	}
	middle, err := trustprofile.ReadMaintenance(r.Anchor(), j, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	j.Close()
	if middle.State != "deleting" || middle.Epoch != 2 || middle.Transaction != before.Transaction || middle.PlanSHA256 == "" {
		t.Fatal(middle)
	}
	if _, err = clientowner.Open(r, factory, time.Now); err == nil {
		t.Fatal("active during partial deletion")
	}
	state, err := runFreshAfterReceipt(t, r, factory, root)
	if err != nil {
		t.Fatal(err)
	}
	if state.State != "files_removed" || state.Epoch != 2 || state.Transaction != before.Transaction || state.PlanSHA256 != middle.PlanSHA256 {
		t.Fatal(state)
	}
	if again, e := RunTest(context.Background(), r, factory, root); e != nil || again != state {
		t.Fatal("completion not idempotent", e)
	}
	p, _, err := installrelease.LoadRemovalTest(root, state.PlanSHA256, true)
	if err != nil {
		t.Fatal(err)
	}
	if err = installrelease.ValidateRemovalTest(root, p, public, r.Specification().Channel, runtime.GOOS, true, true, true); err != nil {
		t.Fatal(err)
	}
	j, err = factory(false)
	if err != nil {
		t.Fatal(err)
	}
	finalTrust, err := j.Load()
	if err != nil {
		t.Fatal(err)
	}
	j.Close()
	// Remove only the changing maintenance member and compare all authority
	// fields byte-for-byte through canonical raw JSON.
	if !bytes.Equal(withoutMaintenance(t, originalTrust), withoutMaintenance(t, finalTrust)) {
		t.Fatal("profile/floor/anchor altered")
	}
	offline, _ := trustinstall.NewOfflineOwner(r, factory, time.Now)
	if _, err = offline.Initialize(); err == nil {
		t.Fatal("reinitialized after removal")
	}
	if _, err = offline.Inspect(); err == nil {
		t.Fatal("active after removal")
	}
	activation, err = records(activationScope, false, true)
	if err != nil {
		t.Fatal(err)
	}
	finalActivation, err := activation.Load()
	activation.Close()
	if err != nil || !bytes.Equal(originalActivation, finalActivation) {
		t.Fatal("activation erased")
	}
	reopened, err := clientruntime.NativeSigner(vault, digest, scope, pub)()
	if err != nil {
		t.Fatal(err)
	}
	message := []byte("remoteapp-invite-request-v1\n" + strings.Repeat("a", 64))
	sig, err := reopened.Sign(message)
	if err != nil || !ed25519.Verify(reopened.PublicKey(), message, sig) {
		t.Fatal("identity no longer signs", err)
	}
	if hex.EncodeToString(reopened.PublicKey()) != pub {
		t.Fatal("identity replaced")
	}
	if err = reopened.(*privatepipe.Signer).CloseJoined(); err != nil {
		t.Fatal(err)
	}
	t.Log("CHECK real unlink + process crash + fresh same-plan resume + files_removed idempotence + unchanged native identity/activation/profile/epoch2; product startup denied PASS")
}

func withoutMaintenance(t *testing.T, raw []byte) []byte {
	t.Helper()
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil {
		t.Fatal("invalid journal")
	}
	delete(fields, "maintenance")
	out, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// Optional local build validation: the portable Windows package runs native
// transaction/deletion tests without requiring a compiler on the target.
func TestCompiledRemovalEntry(t *testing.T) {
	goexe, module := os.Getenv("REMOTEAPP_REMOVAL_BUILD_GO"), os.Getenv("REMOTEAPP_REMOVAL_BUILD_MODULE")
	if goexe == "" || module == "" {
		t.Skip("explicit local compiler/module required")
	}
	r, factory, root, _, _ := nativeFixture(t)
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	helper := os.Getenv("REMOTEAPP_JOURNAL_TEST_EXECUTABLE")
	raw, err := os.ReadFile(helper)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "trust-journal"+suffix), raw, 0500); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(dir, "program-removal-test"+suffix)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	if err = trustinstall.CompileProgramRemovalTest(ctx, goexe, module, runtime.GOOS, exe, r.Specification(), root, time.Now()); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "darwin" {
		if out, e := exec.Command("/usr/bin/codesign", "--force", "--sign", "-", exe).CombinedOutput(); e != nil {
			t.Fatalf("ad-hoc TEST signing %v %s", e, out)
		}
	}
	if out, e := exec.CommandContext(ctx, exe, "--root", root).CombinedOutput(); e == nil || !bytes.Contains(out, []byte(`"status":"failed"`)) {
		t.Fatalf("runtime override accepted %v %s", e, out)
	}
	j, err := factory(false)
	if err != nil {
		t.Fatal(err)
	}
	s, err := trustprofile.ReadMaintenance(r.Anchor(), j, time.Now)
	j.Close()
	if err != nil || s.State != "pending" {
		t.Fatal("argument failure mutated native state", s, err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		out, e := exec.CommandContext(ctx, exe).CombinedOutput()
		if e != nil || !bytes.Contains(out, []byte(`"status":"files_removed","epoch":2`)) {
			t.Fatalf("compiled removal %v %s", e, out)
		}
	}
	t.Log("CHECK fixed-root compiled TEST entry + pinned external native helper + runtime override denied + removal + idempotent reopen PASS")
}

func TestNativeRemovalPendingRecoveryRejectsSubstitution(t *testing.T) {
	for _, mode := range []string{"fresh", "saved-pending", "foreign-transaction", "corrupt-checkpoint", "missing-before-bind"} {
		t.Run(mode, func(t *testing.T) {
			r, factory, root, public, _ := nativeFixture(t)
			j, e := factory(false)
			if e != nil {
				t.Fatal(e)
			}
			state, e := trustprofile.ReadMaintenance(r.Anchor(), j, time.Now)
			j.Close()
			if e != nil {
				t.Fatal(e)
			}
			p, e := installrelease.CaptureRemovalTest(root, public, r.Specification().Channel, runtime.GOOS, state.Transaction, 2, true)
			if e != nil {
				t.Fatal(e)
			}
			if mode == "foreign-transaction" {
				p.Transaction = strings.Repeat("f", 32)
			}
			if mode != "fresh" {
				if e = installrelease.PersistRemovalTest(root, p, true); e != nil && e != installrelease.ErrRemovalReceipt {
					t.Fatal(e)
				}
			}
			pointer := filepath.Join(root, "bootstrap", installrelease.BootstrapPointerName)
			if mode == "corrupt-checkpoint" {
				os.WriteFile(filepath.Join(root, installrelease.RemovalPlanName), []byte("{}"), 0600)
			}
			if mode == "missing-before-bind" {
				os.Remove(pointer)
			}
			result, e := runFreshAfterReceipt(t, r, factory, root)
			if mode == "fresh" || mode == "saved-pending" {
				if e != nil || result.State != "files_removed" || result.Transaction != state.Transaction {
					t.Fatal(result, e)
				}
			} else {
				if e == nil {
					t.Fatal("substitution accepted")
				}
				j, e = factory(false)
				if e != nil {
					t.Fatal(e)
				}
				unchanged, e := trustprofile.ReadMaintenance(r.Anchor(), j, time.Now)
				j.Close()
				if e != nil || unchanged != state {
					t.Fatal("pending mutated", unchanged, e)
				}
				if mode != "missing-before-bind" {
					if _, e = os.Stat(pointer); e != nil {
						t.Fatal("deleted before validation")
					}
				}
			}
		})
	}
}

// Each retry is a fresh owner and only follows an explicit filesystem receipt
// uncertainty. Native errors, invalid plans and arbitrary failures never retry.
func runFreshAfterReceipt(t *testing.T, r trustinstall.Release, f trustinstall.Factory, root string) (trustprofile.MaintenanceState, error) {
	t.Helper()
	for attempt := 0; attempt < 3; attempt++ {
		s, e := RunTest(context.Background(), r, f, root)
		if e != installrelease.ErrRemovalReceipt {
			return s, e
		}
		t.Log("CHECK uncertain filesystem receipt: attempt stopped; explicit fresh owner validates original checkpoint before continuation")
	}
	return trustprofile.MaintenanceState{}, installrelease.ErrRemovalReceipt
}

func TestNativeRemovalBoundCheckpointCannotBeReplaced(t *testing.T) {
	for _, mode := range []string{"corrupt", "missing"} {
		t.Run(mode, func(t *testing.T) {
			r, factory, root, public, _ := nativeFixture(t)
			j, e := factory(false)
			if e != nil {
				t.Fatal(e)
			}
			tx, e := trustprofile.OpenRemovalTransaction(r.Anchor(), j, time.Now)
			if e != nil {
				t.Fatal(e)
			}
			state, _ := tx.State()
			p, e := installrelease.CaptureRemovalTest(root, public, r.Specification().Channel, runtime.GOOS, state.Transaction, 2, true)
			if e != nil {
				t.Fatal(e)
			}
			if e = installrelease.PersistRemovalTest(root, p, true); e != nil && e != installrelease.ErrRemovalReceipt {
				t.Fatal(e)
			}
			_, digest, _ := installrelease.EncodeRemovalPlan(p)
			if e = tx.BindRemovalPlan(digest); e != nil {
				t.Fatal(e)
			}
			before, _ := tx.State()
			j.Close()
			path := filepath.Join(root, installrelease.RemovalPlanName)
			if mode == "corrupt" {
				raw, _, _ := installrelease.EncodeRemovalPlan(p)
				raw[len(raw)-3] ^= 1
				if e = os.WriteFile(path, raw, 0600); e != nil {
					t.Fatal(e)
				}
			} else {
				if e = os.Remove(path); e != nil {
					t.Fatal(e)
				}
			}
			if _, e = RunTest(context.Background(), r, factory, root); e == nil {
				t.Fatal("bound checkpoint replaced or recaptured")
			}
			j, e = factory(false)
			if e != nil {
				t.Fatal(e)
			}
			after, e := trustprofile.ReadMaintenance(r.Anchor(), j, time.Now)
			j.Close()
			if e != nil || after != before {
				t.Fatal("changed native transaction", e)
			}
			if _, e = os.Stat(filepath.Join(root, "bootstrap", installrelease.BootstrapPointerName)); e != nil {
				t.Fatal("deleted after checkpoint failure")
			}
		})
	}
}
