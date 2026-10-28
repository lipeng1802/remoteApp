package installrelease

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func removalFixture(t *testing.T) (string, []byte, RemovalPlan) {
	t.Helper()
	public, key, _ := ed25519.GenerateKey(rand.Reader)
	root := filepath.Join(t.TempDir(), "install")
	if _, err := PrepareProtectedLayout(root, true); err != nil {
		t.Fatal("layout", err)
	}
	var boot VerifiedBootstrap
	for epoch := uint64(1); epoch <= 2; epoch++ {
		stage := makeStage(t, filepath.Join(root, "releases"), "removal-test", runtime.GOOS, epoch, key, byte(epoch))
		if _, err := PromoteStaged(stage, filepath.Join(root, "releases"), public, "removal-test", runtime.GOOS, 1); err != nil {
			t.Fatal("promote", err)
		}
		dir := filepath.Join(root, "bootstrap", "sets", ".stage")
		if err := ensureDirectory(dir, true); err != nil {
			t.Fatal("bootstrap directory", err)
		}
		for _, name := range mustBootstrapExpected(runtime.GOOS) {
			if err := os.WriteFile(filepath.Join(dir, name), []byte{byte(epoch)}, 0500); err != nil {
				t.Fatal(err)
			}
		}
		m, err := BuildBootstrapManifest(dir, "removal-test", runtime.GOOS, epoch)
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := SignBootstrapForBuild(key, m)
		if err = os.WriteFile(filepath.Join(dir, BootstrapManifestName), raw, 0400); err != nil {
			t.Fatal(err)
		}
		boot, err = VerifyBootstrapDirectory(dir, public, "removal-test", runtime.GOOS, 1)
		if err != nil {
			t.Fatal(err)
		}
		name, _ := BootstrapDirectoryName(boot)
		if err = os.Rename(dir, filepath.Join(root, "bootstrap", "sets", name)); err != nil {
			t.Fatal(err)
		}
		boot.Directory = filepath.Join(root, "bootstrap", "sets", name)
		boot.DirectoryName = name
	}
	if err := ActivateBootstrap(root, strings.Repeat("1", 32), boot, "install", public, true); err != nil {
		if opened, e := OpenBootstrap(root, public, "removal-test", runtime.GOOS, 1, true); e != nil || opened.ManifestDigest != boot.ManifestDigest {
			t.Fatal("bootstrap activation", err)
		}
	}
	p, err := CaptureRemovalTest(root, public, "removal-test", runtime.GOOS, strings.Repeat("a", 32), 2, true)
	if err != nil {
		t.Fatal("capture", err)
	}
	return root, public, p
}

func TestRemovalPlanPartialDeletionResumeAndReappearance(t *testing.T) {
	root, public, p := removalFixture(t)
	if err := PersistRemovalTest(root, p, true); err != nil && err != ErrRemovalReceipt {
		t.Fatal(err)
	}
	_, digest, _ := EncodeRemovalPlan(p)
	if _, _, err := LoadRemovalTest(root, strings.Repeat("0", 64), true); err == nil {
		t.Fatal("wrong bound digest")
	}
	done, err := RemoveProgramBatchTest(context.Background(), root, p, public, "removal-test", runtime.GOOS, 1, true)
	if (err != nil && err != ErrRemovalReceipt) || done {
		t.Fatal("partial removal", done, err)
	}
	if ValidateRemovalTest(root, p, public, "removal-test", runtime.GOOS, false, false, true) == nil {
		t.Fatal("missing before bind accepted")
	}
	reopened, _, err := LoadRemovalTest(root, digest, true)
	if err != nil {
		t.Fatal(err)
	}
	done, err = RemoveProgramBatchTest(context.Background(), root, reopened, public, "removal-test", runtime.GOOS, 256, true)
	if err == ErrRemovalReceipt {
		done, err = RemoveProgramBatchTest(context.Background(), root, reopened, public, "removal-test", runtime.GOOS, 256, true)
	}
	if err != nil || !done {
		t.Fatal("resume", done, err)
	}
	if ValidateRemovalTest(root, p, public, "removal-test", runtime.GOOS, true, true, true) != nil {
		t.Fatal("completion")
	}
	if err = os.WriteFile(filepath.Join(root, "bootstrap", BootstrapPointerName), p.Pointer, 0600); err != nil {
		t.Fatal(err)
	}
	if ValidateRemovalTest(root, p, public, "removal-test", runtime.GOOS, true, true, true) == nil {
		t.Fatal("reappeared program accepted")
	}
}

func TestRemovalPlanRejectsChangedTreeBeforeDeleting(t *testing.T) {
	for _, mode := range []string{"extra", "content", "replacement", "hardlink", "symlink", "plan-path", "signature", "root-replacement", "missing", "duplicate-object", "future-epoch"} {
		t.Run(mode, func(t *testing.T) {
			root, public, p := removalFixture(t)
			var target string
			for _, o := range p.Objects {
				if !o.Directory && strings.HasPrefix(o.Path, "releases/") {
					target = filepath.Join(root, filepath.FromSlash(o.Path))
					break
				}
			}
			original, _ := os.ReadFile(target)
			switch mode {
			case "extra":
				os.WriteFile(filepath.Join(root, "user-data.txt"), []byte("keep"), 0600)
			case "content":
				os.Chmod(target, 0600)
				os.WriteFile(target, []byte("changed"), 0600)
			case "replacement":
				os.Rename(target, target+".original")
				os.WriteFile(target, original, 0600)
			case "hardlink":
				if err := os.Link(target, filepath.Join(t.TempDir(), "link")); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				if runtime.GOOS == "windows" {
					// Junctions exercise the reparse-point boundary without
					// requiring SeCreateSymbolicLinkPrivilege/developer mode.
					dir := filepath.Dir(target)
					backup := filepath.Join(t.TempDir(), "original")
					if err := os.Rename(dir, backup); err != nil {
						t.Fatal(err)
					}
					if out, err := exec.Command("cmd.exe", "/d", "/c", "mklink", "/J", dir, backup).CombinedOutput(); err != nil {
						t.Fatalf("junction fixture %v %s", err, out)
					}
				} else {
					if err := os.Rename(target, target+".original"); err != nil {
						t.Fatal(err)
					}
					if err := os.Symlink(target+".original", target); err != nil {
						t.Fatal(err)
					}
				}
			case "plan-path":
				p.Objects[0].Path = "../outside"
			case "signature":
				p.Releases[0][len(p.Releases[0])-5] ^= 1
			case "root-replacement":
				os.Rename(root, root+".old")
				defer os.RemoveAll(root + ".old")
				PrepareProtectedLayout(root, true)
			case "missing":
				os.Remove(target)
			case "duplicate-object":
				p.Objects[1] = p.Objects[0]
			case "future-epoch":
				p.Epoch = 1
			}
			if mode == "missing" {
				if ValidateRemovalTest(root, p, public, "removal-test", runtime.GOOS, false, false, true) == nil {
					t.Fatal("missing pre-bind file accepted")
				}
			} else if _, err := RemoveProgramBatchTest(context.Background(), root, p, public, "removal-test", runtime.GOOS, 256, true); err == nil {
				t.Fatal("changed tree accepted")
			}
			if mode != "root-replacement" {
				if _, err := os.Stat(filepath.Join(root, "bootstrap", BootstrapPointerName)); err != nil {
					t.Fatal("deleted before complete validation")
				}
			}
		})
	}
}

func TestRemovalPlanProductionDisabledAndCorruptCheckpoint(t *testing.T) {
	root, public, p := removalFixture(t)
	if _, err := CaptureRemovalTest(root, public, "removal-test", runtime.GOOS, p.Transaction, 2, false); err == nil {
		t.Fatal("production capture")
	}
	if PersistRemovalTest(root, p, false) == nil {
		t.Fatal("production persist")
	}
	if _, err := RemoveProgramBatchTest(context.Background(), root, p, public, "removal-test", runtime.GOOS, 256, false); err == nil {
		t.Fatal("production deletion")
	}
	if err := PersistRemovalTest(root, p, true); err != nil && err != ErrRemovalReceipt {
		t.Fatal(err)
	}
	if err := PersistRemovalTest(root, p, true); err == nil {
		t.Fatal("checkpoint replaced")
	}
	if err := os.WriteFile(filepath.Join(root, RemovalPlanName), []byte(`{"version":1}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := LoadRemovalTest(root, "", true); err == nil {
		t.Fatal("corrupt plan accepted")
	}
}
