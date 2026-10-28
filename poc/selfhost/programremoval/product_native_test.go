package programremoval

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/autonomousbinding"
	"remoteapp.local/selfhost-poc/clientruntime"
	"remoteapp.local/selfhost-poc/enrollmentstore"
	"remoteapp.local/selfhost-poc/installrelease"
	"remoteapp.local/selfhost-poc/localmaintenance"
	"remoteapp.local/selfhost-poc/privatepipe"
	"remoteapp.local/selfhost-poc/processowner"
	"remoteapp.local/selfhost-poc/productmaintenance"
	"remoteapp.local/selfhost-poc/trustinstall"
	"remoteapp.local/selfhost-poc/trustprofile"
)

type productFixture struct {
	Version       int    `json:"version"`
	Release       string `json:"release"`
	Platform      string `json:"platform"`
	ProductSHA256 string `json:"product_sha256"`
}

func setupActualProduct(t *testing.T) (trustinstall.Release, trustinstall.Factory, autonomousbinding.JournalFactory, autonomousbinding.Record, string, processowner.Spec, string, string) {
	t.Helper()
	file := os.Getenv("REMOTEAPP_PRODUCT_MAINT_FIXTURE")
	if file == "" {
		t.Skip("actual compiled product fixture required")
	}
	raw, e := os.ReadFile(file)
	if e != nil {
		t.Fatal(e)
	}
	var fixture productFixture
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&fixture) != nil || fixture.Version != 1 || fixture.Platform != runtime.GOOS {
		t.Fatal("invalid TEST fixture")
	}
	r, e := trustinstall.DecodeCompiled(fixture.Release)
	if e != nil || !r.Specification().Test {
		t.Fatal("invalid TEST release")
	}
	helper, vault := os.Getenv("REMOTEAPP_JOURNAL_TEST_EXECUTABLE"), os.Getenv("REMOTEAPP_VAULT_TEST_EXECUTABLE")
	factory, e := r.NativeFactory(helper)
	if e != nil {
		t.Fatal(e)
	}
	offline, e := trustinstall.NewOfflineOwner(r, factory, time.Now)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = offline.Initialize(); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		if e := trustprofile.DeleteTestDesktopJournal(helper, r.Scope()); e != nil {
			t.Error(e)
		}
	})
	records, e := r.NativeRecordFactory(helper)
	if e != nil {
		t.Fatal(e)
	}
	bf := func(scope string, create, test bool) (autonomousbinding.Journal, error) {
		return records(scope, create, test)
	}
	ef := func(scope string, create, test bool) (enrollmentstore.Journal, error) {
		return records(scope, create, test)
	}
	vaultRaw, e := os.ReadFile(vault)
	if e != nil {
		t.Fatal(e)
	}
	hash := sha256.Sum256(vaultRaw)
	vaultDigest := hex.EncodeToString(hash[:])
	var nonce [16]byte
	rand.Read(nonce[:])
	scope := hex.EncodeToString(nonce[:])
	signer, e := clientruntime.CreateNativeIdentity(vault, vaultDigest, scope)
	if e != nil {
		t.Fatal(e)
	}
	public := hex.EncodeToString(signer.PublicKey())
	if e = signer.CloseJoined(); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		if e := clientruntime.DeleteNativeTestIdentity(vault, vaultDigest, scope, public); e != nil {
			t.Error(e)
		}
	})
	installer, e := autonomousbinding.BeginWithJournal(r.Scope(), true, bf)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		if e := autonomousbinding.CleanupTestWith(r.Scope(), func(s string) error { return trustprofile.DeleteTestDesktopJournal(helper, s) }); e != nil {
			t.Error(e)
		}
	})
	binding, e := installer.Commit(scope, public)
	if e != nil {
		installer.Close()
		t.Fatal(e)
	}
	record := binding.Record()
	if e = binding.Close(); e != nil {
		t.Fatal(e)
	}
	node, e := enrollmentstore.InitializeWithJournal(record.EnrollmentScope, true, ef)
	if e != nil {
		t.Fatal(e)
	}
	if e = node.Close(); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		node, e := enrollmentstore.OpenWithJournal(record.EnrollmentScope, true, ef)
		if e != nil {
			t.Error(e)
			return
		}
		if e = node.CleanupTestAfterJoinWith(func(s string) error { return trustprofile.DeleteTestDesktopJournal(helper, s) }); e != nil {
			t.Error(e)
		}
	})
	parent, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	root := filepath.Join(parent, "installed")
	if _, e = installrelease.PrepareProtectedLayout(root, true); e != nil {
		t.Fatal(e)
	}
	dir := filepath.Dir(file)
	payload := filepath.Join(dir, "payload")
	pub, _ := hex.DecodeString(r.Specification().Root)
	verified, e := installrelease.VerifyDirectory(payload, pub, r.Specification().Channel, runtime.GOOS, 1)
	if e != nil {
		t.Fatal(e)
	}
	stage := ".staging-" + strings.Repeat("1", 32)
	v, _, e := installrelease.ReconcileCandidate(payload, root, stage, verified.ManifestDigest, 2, pub, r.Specification().Channel, runtime.GOOS, 1, true)
	if e != nil {
		v, _, e = installrelease.ReconcileCandidate(payload, root, stage, verified.ManifestDigest, 2, pub, r.Specification().Channel, runtime.GOOS, 1, true)
	}
	if e != nil {
		t.Fatal(e)
	}
	v, e = installrelease.PromoteStaged(v.Directory, filepath.Join(root, "releases"), pub, r.Specification().Channel, runtime.GOOS, 1)
	if e != nil {
		t.Fatal(e)
	}
	bs := filepath.Join(dir, "bootstrap-payload")
	original, e := installrelease.VerifyBootstrapDirectory(bs, pub, r.Specification().Channel, runtime.GOOS, 1)
	if e != nil {
		t.Fatal(e)
	}
	b, _, e := installrelease.ReconcileBootstrapCandidate(bs, root, stage, original.ManifestDigest, 2, pub, r.Specification().Channel, runtime.GOOS, 1, true)
	if e != nil {
		b, _, e = installrelease.ReconcileBootstrapCandidate(bs, root, stage, original.ManifestDigest, 2, pub, r.Specification().Channel, runtime.GOOS, 1, true)
	}
	if e != nil {
		t.Fatal(e)
	}
	b, e = installrelease.PromoteBootstrapStaged(b.Directory, root, pub, r.Specification().Channel, runtime.GOOS, 1, true)
	if e != nil {
		t.Fatal(e)
	}
	if e = installrelease.ActivateBootstrap(root, strings.Repeat("2", 32), b, "install", pub, true); e != nil {
		opened, e2 := installrelease.OpenBootstrap(root, pub, r.Specification().Channel, runtime.GOOS, 1, true)
		if e2 != nil || opened.ManifestDigest != b.ManifestDigest {
			t.Fatal(e)
		}
	}
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	return r, factory, bf, record, root, processowner.Spec{Executable: filepath.Join(v.Directory, "installed-autonomous-product"+suffix), SHA256: fixture.ProductSHA256}, vault, vaultDigest
}

func TestNativeCompiledSupervisorPrivateRoutingAndStop(t *testing.T) {
	goexe, module := os.Getenv("REMOTEAPP_REMOVAL_BUILD_GO"), os.Getenv("REMOTEAPP_REMOVAL_BUILD_MODULE")
	if goexe == "" || module == "" {
		t.Skip("explicit local compiler/module required")
	}
	r, factory, records, identity, _, product, _, digest := setupActualProduct(t)
	dir, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	for name, source := range map[string]string{"trust-journal": os.Getenv("REMOTEAPP_JOURNAL_TEST_EXECUTABLE"), "identity-vault": os.Getenv("REMOTEAPP_VAULT_TEST_EXECUTABLE")} {
		raw, e := os.ReadFile(source)
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(filepath.Join(dir, name+suffix), raw, 0500); e != nil {
			t.Fatal(e)
		}
	}
	exe := filepath.Join(dir, "supervisor"+suffix)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	if e = trustinstall.CompileProductSupervisorTest(ctx, goexe, module, runtime.GOOS, exe, r.Specification(), product.Executable, product.SHA256, digest, time.Now()); e != nil {
		t.Fatal(e)
	}
	if runtime.GOOS == "darwin" {
		if raw, e := exec.Command("/usr/bin/codesign", "--force", "--sign", "-", exe).CombinedOutput(); e != nil {
			t.Fatalf("codesign: %s %v", raw, e)
		}
	}
	raw, e := os.ReadFile(exe)
	if e != nil {
		t.Fatal(e)
	}
	sum := sha256.Sum256(raw)
	child, e := processowner.Start(processowner.Spec{Executable: exe, SHA256: hex.EncodeToString(sum[:])})
	if e != nil {
		t.Fatal(e)
	}
	defer child.Stop(context.Background())
	reply, e := child.Exchange(ctx, map[string]string{"op": "open"})
	if e != nil {
		t.Fatal(e)
	}
	var ready struct {
		Status  string `json:"status"`
		Address string `json:"maintenance_address"`
		Scope   string `json:"vault_scope"`
		Public  string `json:"public_key"`
	}
	if json.Unmarshal(reply, &ready) != nil || ready.Status != "ready" || ready.Scope != identity.VaultScope || ready.Public != identity.PublicKey {
		t.Fatal("supervisor identity not from native binding")
	}
	for _, request := range []map[string]string{{"op": "product_count"}, {"op": "maintenance_prepare"}, {"op": "device", "VaultScope": identity.VaultScope}} {
		reply, e = child.Exchange(ctx, request)
		if e != nil {
			t.Fatal(e)
		}
		var status struct {
			Status string `json:"status"`
		}
		if json.Unmarshal(reply, &status) != nil {
			t.Fatal("invalid reply")
		}
		want := "request_rejected"
		if request["op"] == "product_count" {
			want = "product_unavailable"
		}
		if status.Status != want {
			t.Fatalf("status %s, want %s", status.Status, want)
		}
	}
	if e = child.Stop(ctx); e != nil {
		t.Fatal("supervisor/product join", e)
	}
	owner, e := trustinstall.NewOfflineOwner(r, factory, time.Now)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = owner.Inspect(); e != nil {
		t.Fatal("ordinary stop changed trust", e)
	}
	binding, e := autonomousbinding.OpenWithJournal(r.Scope(), true, records)
	if e != nil {
		t.Fatal(e)
	}
	if binding.Record() != identity {
		t.Error("ordinary stop changed binding")
	}
	if e = binding.Close(); e != nil {
		t.Fatal(e)
	}
}

func TestNativeRemovalActualProductExitHandoff(t *testing.T) {
	for _, mode := range []string{"complete", "abort-finish"} {
		t.Run(mode, func(t *testing.T) {
			r, factory, records, identity, root, spec, vault, vaultDigest := setupActualProduct(t)
			ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
			defer cancel()
			controller, e := productmaintenance.StartTest(ctx, r, spec, factory, records, func(record autonomousbinding.Record) (productmaintenance.Signer, error) {
				if record != identity {
					return nil, productmaintenance.ErrProduct
				}
				s, e := clientruntime.NativeSigner(vault, vaultDigest, record.VaultScope, record.PublicKey)()
				if e != nil {
					return nil, e
				}
				return s.(*privatepipe.Signer), nil
			})
			if controller != nil {
				defer controller.Release(context.Background())
			}
			if e != nil {
				t.Fatal("actual product start", e)
			}
			if controller.Identity() != identity {
				t.Fatal("identity not from native binding")
			}
			if competing, e := factory(false); e == nil {
				competing.Close()
				t.Fatal("product did not hold native trust")
			}
			if b, e := autonomousbinding.OpenWithJournal(r.Scope(), true, records); e == nil {
				b.Close()
				t.Fatal("product did not hold binding")
			}
			l, e := net.Listen("tcp4", "127.0.0.1:0")
			if e != nil {
				t.Fatal(e)
			}
			endpoint, e := localmaintenance.NewTest(r, controller, controller.Signer(), l)
			if e != nil {
				l.Close()
				t.Fatal(e)
			}
			type serverReply struct {
				state trustprofile.MaintenanceState
				err   error
			}
			done := make(chan serverReply, 1)
			go func() { s, e := endpoint.Serve(ctx); done <- serverReply{s, e}; close(done) }()
			defer func() {
				endpoint.Close()
				select {
				case <-done:
				case <-time.After(3 * time.Second):
					t.Error("authentication endpoint did not join")
				}
			}()
			encoded, e := trustinstall.EncodeForTrustedBuild(r.Specification(), time.Now())
			if e != nil {
				t.Fatal(e)
			}
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestAuthenticatedRemovalClientFixture$")
			cmd.Env = append(os.Environ(), "REMOTEAPP_AUTH_MAINT_RELEASE="+encoded, "REMOTEAPP_AUTH_MAINT_PUBLIC="+identity.PublicKey, "REMOTEAPP_AUTH_MAINT_VAULT_SCOPE="+identity.VaultScope, "REMOTEAPP_AUTH_MAINT_ADDRESS="+l.Addr().String(), "REMOTEAPP_AUTH_MAINT_ROOT="+root, "REMOTEAPP_AUTH_MAINT_MODE="+mode)
			out, e := cmd.CombinedOutput()
			var reply authenticatedClientReply
			if e != nil || json.Unmarshal(out, &reply) != nil {
				t.Fatalf("independent client %v %s", e, out)
			}
			result := <-done
			if mode == "complete" {
				if result.err != nil || reply.Status != "files_removed" || reply.Transaction != result.state.Transaction {
					t.Fatal(reply, result.err)
				}
				select {
				case <-controller.Exited():
				default:
					t.Fatal("files removed before actual product exit")
				}
				if _, e = os.Lstat(spec.Executable); !os.IsNotExist(e) {
					t.Fatal("actual product executable retained", e)
				}
				if e = controller.Release(context.Background()); e != nil {
					t.Fatal(e)
				}
			} else {
				if result.err == nil || reply.Status != "rejected" {
					t.Fatal("aborted confirmation accepted")
				}
				select {
				case <-controller.Exited():
					t.Fatal("interruption prematurely exited product")
				default:
				}
				if _, e = os.Stat(spec.Executable); e != nil {
					t.Fatal("interruption deleted executable")
				}
				if competing, e := factory(false); e == nil {
					competing.Close()
					t.Fatal("interruption released live lease")
				}
				// Explicit failed teardown joins actual nonzero product exit. It
				// cannot become authenticated success or authorize deletion.
				if e = controller.Release(context.Background()); e == nil {
					t.Fatal("aborted product teardown reported success")
				}
			}
			b, e := autonomousbinding.OpenWithJournal(r.Scope(), true, records)
			if e != nil {
				t.Fatal("binding lease not released after exit", e)
			}
			if b.Check() != nil || b.Record() != identity {
				t.Fatal("binding changed")
			}
			b.Close()
			j, e := factory(false)
			if e != nil {
				t.Fatal(e)
			}
			state, e := trustprofile.ReadMaintenance(r.Anchor(), j, time.Now)
			j.Close()
			want := "files_removed"
			if mode == "abort-finish" {
				want = "pending"
			}
			if e != nil || state.State != want || state.Epoch != 2 || state.Transaction != result.state.Transaction {
				t.Fatal(state, e)
			}
			t.Log("CHECK actual compiled installed-autonomous-product + native binding selected identity + trusted prepare + signer/binding/reader/zero-exit gate before deletion; interrupted finish retains product and explicit failed join PASS")
		})
	}
}
