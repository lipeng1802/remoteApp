package installrelease

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type removedActivationJournal struct{ raw []byte }

func (j removedActivationJournal) Load() ([]byte, error)       { return append([]byte(nil), j.raw...), nil }
func (j removedActivationJournal) Commit([]byte, []byte) error { return ErrRelease }
func (j removedActivationJournal) Close() error                { return nil }
func removedFixture(t *testing.T) (string, []byte, RemovalPlan, removedActivationJournal) {
	t.Helper()
	root, public, p := removalFixture(t)
	if e := PersistRemovalTest(root, p, true); e != nil {
		t.Fatal(e)
	}
	var active Activation
	for _, raw := range p.Releases {
		signed, d, e := Verify(raw, public, "removal-test", runtime.GOOS, 1)
		if e != nil {
			t.Fatal(e)
		}
		if signed.Manifest.Epoch == 2 {
			v := Verified{Manifest: signed.Manifest, ManifestDigest: d}
			v.DirectoryName, _ = ReleaseDirectoryName(v)
			active, e = activationFor(v)
			if e != nil {
				t.Fatal(e)
			}
		}
	}
	encoded, e := encodeActivation(active)
	if e != nil {
		t.Fatal(e)
	}
	done, e := RemoveProgramBatchTest(context.Background(), root, p, public, "removal-test", runtime.GOOS, 256, true)
	if e != nil || !done {
		t.Fatal("program deletion", e)
	}
	return root, public, p, removedActivationJournal{encoded}
}
func TestRemovedRetirementRejectsForeignActivationUnknownAndReappearedFiles(t *testing.T) {
	for _, mode := range []string{"foreign_activation", "unknown", "reappeared", "wrong_plan", "production"} {
		t.Run(mode, func(t *testing.T) {
			root, public, p, j := removedFixture(t)
			test := true
			switch mode {
			case "foreign_activation":
				a, _ := decodeActivation(j.raw)
				a.ManifestSHA256 = strings.Repeat("b", 64)
				a.Directory = formatDirectory(a.Epoch, a.ManifestSHA256)
				j.raw, _ = encodeActivation(a)
			case "unknown":
				if e := os.WriteFile(filepath.Join(root, "unknown"), []byte("preserve"), 0600); e != nil {
					t.Fatal(e)
				}
			case "reappeared":
				for _, o := range p.Objects {
					if !o.Directory {
						if e := os.WriteFile(filepath.Join(root, filepath.FromSlash(o.Path)), []byte("reappeared"), 0600); e != nil {
							t.Fatal(e)
						}
						break
					}
				}
			case "wrong_plan":
				p.Transaction = strings.Repeat("c", 32)
			case "production":
				test = false
			}
			if ValidateRemovedActivationTest(root, p, j, public, "removal-test", runtime.GOOS, 1, test) == nil {
				t.Fatal("bad removed activation admitted")
			}
			// An unsafe directory cannot be swept even with a superficially valid plan.
			if mode != "foreign_activation" && CleanupRemovedTestShells(root, p, public, "removal-test", runtime.GOOS, test) == nil {
				t.Fatal("unsafe shell cleanup")
			}
			if _, e := os.Stat(root); e != nil {
				t.Fatal("rejected root was removed")
			}
		})
	}
}
func TestRemovedRetirementCleansOnlyAttestedEmptyShells(t *testing.T) {
	root, public, p, j := removedFixture(t)
	if e := ValidateRemovedActivationTest(root, p, j, public, "removal-test", runtime.GOOS, 1, true); e != nil {
		t.Fatal(e)
	}
	if e := CleanupRemovedTestShells(root, p, public, "removal-test", runtime.GOOS, true); e != nil {
		t.Fatal(e)
	}
	if _, e := os.Lstat(root); !os.IsNotExist(e) {
		t.Fatal("known empty shells remained")
	}
	if e := CleanupRemovedTestShells(root, p, public, "removal-test", runtime.GOOS, true); e == nil {
		t.Fatal("retired root implicitly reused")
	}
}
