package installrelease

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
)

const RemovalPlanName = ".program-removal.json"
const maximumRemovalPlan = 128 << 10

// ErrRemovalReceipt means writes/unlinks may already be visible. Stop this
// attempt; a fresh explicit invocation must validate the same bound checkpoint.
var ErrRemovalReceipt = errors.New("program_removal_receipt_uncertain")

// RemovalPlan is a checkpoint, not authority. Its exact digest must be committed
// in the native maintenance journal before allowing any absent program file.
// All file paths are reconstructed from signed fixed-layout manifests.
type RemovalPlan struct {
	Version     int             `json:"version"`
	Transaction string          `json:"transaction"`
	Epoch       uint64          `json:"epoch"`
	Layout      LayoutReceipt   `json:"layout"`
	Releases    [][]byte        `json:"releases"`
	Bootstraps  [][]byte        `json:"bootstraps"`
	Pointer     []byte          `json:"pointer"`
	Objects     []removalObject `json:"objects"`
}
type removalObject struct {
	Path      string `json:"path"`
	Directory bool   `json:"directory"`
	Volume    uint64 `json:"volume"`
	Index     uint64 `json:"index"`
	Size      int64  `json:"size"`
	SHA256    string `json:"sha256"`
}
type removalContent struct {
	directory bool
	size      int64
	digest    string
}

func content(raw []byte) removalContent {
	h := sha256.Sum256(raw)
	return removalContent{size: int64(len(raw)), digest: hex.EncodeToString(h[:])}
}

// removalInventory independently verifies every saved signature and derives the
// only permissible paths. A plan cannot add a path by editing its Objects list.
func removalInventory(p RemovalPlan, public []byte, channel, platform string) (map[string]removalContent, error) {
	if p.Version != 1 || !canonicalHex(p.Transaction, 16) || p.Epoch == 0 || len(public) != ed25519.PublicKeySize || len(p.Releases) < 1 || len(p.Releases) > 8 || len(p.Bootstraps) < 1 || len(p.Bootstraps) > 8 {
		return nil, ErrRelease
	}
	m := map[string]removalContent{".": {directory: true}, "releases": {directory: true}, "bootstrap": {directory: true}, "bootstrap/sets": {directory: true}}
	add := func(path string, c removalContent) bool {
		if _, ok := m[path]; ok {
			return false
		}
		m[path] = c
		return true
	}
	for _, raw := range p.Releases {
		s, d, err := Verify(raw, public, channel, platform, 1)
		if err != nil || s.Manifest.Epoch > p.Epoch {
			return nil, ErrRelease
		}
		name, _ := ReleaseDirectoryName(Verified{Manifest: s.Manifest, ManifestDigest: d})
		base := "releases/" + name
		if !add(base, removalContent{directory: true}) {
			return nil, ErrRelease
		}
		m[base+"/"+ManifestName] = content(raw)
		for _, f := range s.Manifest.Files {
			m[base+"/"+f.Name] = removalContent{size: f.Size, digest: f.SHA256}
		}
	}
	var pointer BootstrapPointer
	if json.Unmarshal(p.Pointer, &pointer) != nil || !validBootstrapPointer(pointer) || pointer.Channel != channel || pointer.Platform != platform || pointer.Epoch > p.Epoch {
		return nil, ErrRelease
	}
	canonical, _ := json.Marshal(pointer)
	if !bytes.Equal(canonical, p.Pointer) {
		return nil, ErrRelease
	}
	found := false
	for _, raw := range p.Bootstraps {
		var s SignedBootstrap
		if json.Unmarshal(raw, &s) != nil {
			return nil, ErrRelease
		}
		canonical, _ := json.Marshal(s)
		message, err := bootstrapMessage(s.Manifest)
		sig, e := hex.DecodeString(s.Signature)
		if !bytes.Equal(raw, canonical) || err != nil || e != nil || !canonicalHex(s.Signature, 64) || !ed25519.Verify(ed25519.PublicKey(public), message, sig) || s.Manifest.Channel != channel || s.Manifest.Platform != platform || s.Manifest.Epoch > p.Epoch {
			return nil, ErrRelease
		}
		d := content(raw).digest
		name, _ := BootstrapDirectoryName(VerifiedBootstrap{Manifest: s.Manifest, ManifestDigest: d})
		base := "bootstrap/sets/" + name
		if !add(base, removalContent{directory: true}) {
			return nil, ErrRelease
		}
		m[base+"/"+BootstrapManifestName] = content(raw)
		for _, f := range s.Manifest.Files {
			m[base+"/"+f.Name] = removalContent{size: f.Size, digest: f.SHA256}
		}
		if pointer.Directory == name && pointer.ManifestSHA256 == d && pointer.Epoch == s.Manifest.Epoch {
			found = true
		}
	}
	if !found {
		return nil, ErrRelease
	}
	m["bootstrap/"+BootstrapPointerName] = content(p.Pointer)
	return m, nil
}

// CaptureRemovalTest requires a fully intact signed installation. It cannot
// recover missing files before the plan digest is bound in native storage.
func CaptureRemovalTest(root string, public []byte, channel, platform, transaction string, epoch uint64, test bool) (RemovalPlan, error) {
	if !test {
		return RemovalPlan{}, ErrRelease
	}
	layout, err := layoutReceipt(root, true)
	if err != nil {
		return RemovalPlan{}, err
	}
	p := RemovalPlan{Version: 1, Transaction: transaction, Epoch: epoch, Layout: layout}
	for _, group := range []struct {
		path, manifest string
		target         *[][]byte
	}{{"releases", ManifestName, &p.Releases}, {"bootstrap/sets", BootstrapManifestName, &p.Bootstraps}} {
		entries, e := os.ReadDir(filepath.Join(root, filepath.FromSlash(group.path)))
		if e != nil || len(entries) < 1 || len(entries) > 8 {
			return RemovalPlan{}, ErrRelease
		}
		for _, entry := range entries {
			raw, e := readRegular(filepath.Join(root, filepath.FromSlash(group.path), entry.Name(), group.manifest), maximumManifest)
			if e != nil {
				return RemovalPlan{}, ErrRelease
			}
			*group.target = append(*group.target, raw)
		}
	}
	p.Pointer, err = readRegular(filepath.Join(root, "bootstrap", BootstrapPointerName), 2048)
	if err != nil {
		return RemovalPlan{}, err
	}
	inventory, err := removalInventory(p, public, channel, platform)
	if err != nil {
		return RemovalPlan{}, err
	}
	paths := make([]string, 0, len(inventory))
	for path := range inventory {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		c := inventory[path]
		f, id, e := openChecked(filepath.Join(root, filepath.FromSlash(path)), c.directory, true)
		if e != nil {
			return RemovalPlan{}, ErrRelease
		}
		f.Close()
		p.Objects = append(p.Objects, removalObject{path, c.directory, id.volume, id.index, c.size, c.digest})
	}
	if err = ValidateRemovalTest(root, p, public, channel, platform, false, false, true); err != nil {
		return RemovalPlan{}, err
	}
	return p, nil
}

func EncodeRemovalPlan(p RemovalPlan) ([]byte, string, error) {
	if p.Version != 1 || !canonicalHex(p.Transaction, 16) || p.Epoch == 0 || len(p.Releases) < 1 || len(p.Releases) > 8 || len(p.Bootstraps) < 1 || len(p.Bootstraps) > 8 || len(p.Objects) < 1 || len(p.Objects) > 256 {
		return nil, "", ErrRelease
	}
	raw, err := json.Marshal(p)
	if err != nil || len(raw) > maximumRemovalPlan {
		return nil, "", ErrRelease
	}
	return raw, content(raw).digest, nil
}

// PersistRemovalTest writes once. An uncertain write never authorizes removal;
// a fresh owner may re-read the complete checkpoint while still pending.
func PersistRemovalTest(root string, p RemovalPlan, test bool) error {
	if !test || ValidateLayoutReceipt(p.Layout, root, true) != nil {
		return ErrRelease
	}
	raw, _, err := EncodeRemovalPlan(p)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(root, RemovalPlanName), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return ErrRelease
	}
	n, e := f.Write(raw)
	syncErr, closeErr := f.Sync(), f.Close()
	if e != nil || n != len(raw) || syncErr != nil || closeErr != nil {
		return ErrRelease
	}
	return syncRemovalDirectory(root)
}

func LoadRemovalTest(root, expectedDigest string, test bool) (RemovalPlan, string, error) {
	if !test {
		return RemovalPlan{}, "", ErrRelease
	}
	raw, err := readRegular(filepath.Join(root, RemovalPlanName), maximumRemovalPlan)
	if err != nil {
		return RemovalPlan{}, "", err
	}
	var p RemovalPlan
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&p) != nil {
		return RemovalPlan{}, "", ErrRelease
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return RemovalPlan{}, "", ErrRelease
	}
	canonical, digest, err := EncodeRemovalPlan(p)
	if err != nil || !bytes.Equal(raw, canonical) || (expectedDigest != "" && digest != expectedDigest) {
		return RemovalPlan{}, "", ErrRelease
	}
	return p, digest, nil
}

// ValidateRemovalTest checks the entire remaining tree before any deletion.
// Directory identities persist across recovery; symlinks, hardlinks, extra
// files, changed content and replaced objects fail closed. Power-loss durability
// and hostile same-user path/unlink races are outside this TEST-only API.
func ValidateRemovalTest(root string, p RemovalPlan, public []byte, channel, platform string, allowMissing, allAbsent, test bool) error {
	if !test || ValidateLayoutReceipt(p.Layout, root, true) != nil {
		return ErrRelease
	}
	inventory, err := removalInventory(p, public, channel, platform)
	if err != nil || len(p.Objects) != len(inventory) {
		return ErrRelease
	}
	seen := map[string]bool{}
	for _, o := range p.Objects {
		c, ok := inventory[o.Path]
		if !ok || seen[o.Path] || o.Directory != c.directory || o.Size != c.size || o.SHA256 != c.digest {
			return ErrRelease
		}
		seen[o.Path] = true
		path := filepath.Join(root, filepath.FromSlash(o.Path))
		_, e := os.Lstat(path)
		if os.IsNotExist(e) && !o.Directory && allowMissing {
			continue
		}
		if e != nil || (!o.Directory && allAbsent) {
			return ErrRelease
		}
		f, id, e := openChecked(path, o.Directory, true)
		if e != nil {
			return ErrRelease
		}
		if id.volume != o.Volume || id.index != o.Index {
			f.Close()
			return ErrRelease
		}
		if !o.Directory {
			h := sha256.New()
			n, e := io.Copy(h, io.LimitReader(f, maximumFile+1))
			if e != nil || n != o.Size || hex.EncodeToString(h.Sum(nil)) != o.SHA256 {
				f.Close()
				return ErrRelease
			}
		}
		if f.Close() != nil {
			return ErrRelease
		}
	}
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, e error) error {
		if e != nil {
			return ErrRelease
		}
		rel, e := filepath.Rel(root, path)
		if e != nil {
			return ErrRelease
		}
		rel = filepath.ToSlash(rel)
		if rel == RemovalPlanName {
			loaded, _, e := LoadRemovalTest(root, "", true)
			if e != nil || !reflect.DeepEqual(loaded, p) {
				return ErrRelease
			}
			return nil
		}
		if _, ok := inventory[rel]; !ok {
			return ErrRelease
		}
		return nil
	})
}

// RemoveProgramBatchTest never recurses or removes a directory. The caller must
// hold the exclusive native lease and prove digest-bound state == deleting.
// Keeping directory shells and the checkpoint permits exact crash recovery.
func RemoveProgramBatchTest(ctx context.Context, root string, p RemovalPlan, public []byte, channel, platform string, limit int, test bool) (bool, error) {
	if ctx == nil || ctx.Err() != nil || limit < 1 || !test || ValidateRemovalTest(root, p, public, channel, platform, true, false, true) != nil {
		return false, ErrRelease
	}
	removed := 0
	handles, err := openRemovalHandles(root, p)
	if err != nil {
		return false, err
	}
	defer handles.close()
	dirty := map[string]bool{}
	for _, o := range p.Objects {
		if o.Directory {
			continue
		}
		// Verify the tree before opening a Windows DELETE handle: Go's
		// ordinary read opens do not share DELETE access. Once acquired,
		// validate/delete through that exact handle without reopening it.
		if ctx.Err() != nil || ValidateRemovalTest(root, p, public, channel, platform, true, false, true) != nil {
			return false, ErrRelease
		}
		file, e := handles.openFile(o)
		if os.IsNotExist(e) {
			continue
		}
		if e != nil {
			return false, ErrRelease
		}
		if removed == limit {
			file.Close()
			for dir := range dirty {
				if syncRemovalDirectory(dir) != nil {
					return false, ErrRemovalReceipt
				}
			}
			return false, nil
		}
		removeErr, closeErr := handles.remove(file, o), file.Close()
		if removeErr != nil || closeErr != nil {
			return false, ErrRelease
		}
		removed++
		dirty[filepath.Dir(filepath.Join(root, filepath.FromSlash(o.Path)))] = true
	}
	for dir := range dirty {
		if syncRemovalDirectory(dir) != nil {
			return false, ErrRemovalReceipt
		}
	}
	return true, ValidateRemovalTest(root, p, public, channel, platform, true, true, true)
}

func syncRemovalDirectory(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return ErrRelease
	}
	e, c := f.Sync(), f.Close()
	if e != nil || c != nil {
		return ErrRemovalReceipt
	}
	return nil
}
