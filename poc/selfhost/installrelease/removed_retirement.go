package installrelease

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ValidateRemovedActivationTest binds the native activation to a signed release
// in the digest-bound removal checkpoint, without treating missing files as an
// intact install. Caller must own exclusive native leases and have checked the
// exact files_removed transaction/epoch/plan digest before calling this.
func ValidateRemovedActivationTest(root string, p RemovalPlan, j Journal, public []byte, channel, platform string, floor uint64, test bool) error {
	if !test || j == nil || floor == 0 || p.Epoch < floor || ValidateRemovalTest(root, p, public, channel, platform, true, true, true) != nil {
		return ErrRelease
	}
	raw, e := j.Load()
	if e != nil {
		return ErrRelease
	}
	active, e := decodeActivation(raw)
	if e != nil || active.Channel != channel || active.Platform != platform || active.Epoch < floor || active.Epoch > p.Epoch {
		return ErrRelease
	}
	found := false
	for _, manifest := range p.Releases {
		signed, digest, e := Verify(manifest, public, channel, platform, floor)
		if e != nil {
			continue
		}
		v := Verified{Manifest: signed.Manifest, ManifestDigest: digest}
		v.DirectoryName, _ = ReleaseDirectoryName(v)
		want, e := activationFor(v)
		if e == nil && want == active {
			found = true
		}
	}
	after, e := j.Load()
	if !found || e != nil || !bytes.Equal(raw, after) {
		return ErrRelease
	}
	return nil
}

// CleanupRemovedTestShells runs only after the independent online TEST
// retirement proof and exact native record cleanup. It never deletes a program
// file: every signed program file must ALREADY be absent. Unknown/reappeared
// objects fail before the first rmdir; each removed object is an empty known
// directory. Uncertain filesystem receipts require explicit diagnosis, no retry.
func CleanupRemovedTestShells(root string, p RemovalPlan, public []byte, channel, platform string, test bool) error {
	if !test || ValidateRemovalTest(root, p, public, channel, platform, true, true, true) != nil {
		return ErrRelease
	}
	dirs := []string{}
	for _, o := range p.Objects {
		if o.Directory && o.Path != "." {
			dirs = append(dirs, o.Path)
		}
	}
	sort.Slice(dirs, func(i, j int) bool {
		if strings.Count(dirs[i], "/") == strings.Count(dirs[j], "/") {
			return dirs[i] > dirs[j]
		}
		return strings.Count(dirs[i], "/") > strings.Count(dirs[j], "/")
	})
	for _, relative := range dirs {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if e := os.Remove(path); e != nil {
			return ErrRelease
		}
		if syncRemovalDirectory(filepath.Dir(path)) != nil {
			return ErrRemovalReceipt
		}
	}
	// Keep the checkpoint through all subdirectory removals. It is data checked
	// against the caller's native-bound digest, never an arbitrary cleanup path.
	loaded, digest, e := LoadRemovalTest(root, "", true)
	_, expected, e2 := EncodeRemovalPlan(p)
	if e != nil || e2 != nil || digest != expected || loaded.Transaction != p.Transaction {
		return ErrRelease
	}
	if os.Remove(filepath.Join(root, RemovalPlanName)) != nil {
		return ErrRelease
	}
	if syncRemovalDirectory(root) != nil {
		return ErrRemovalReceipt
	}
	if os.Remove(root) != nil {
		return ErrRelease
	}
	return syncRemovalDirectory(filepath.Dir(root))
}
