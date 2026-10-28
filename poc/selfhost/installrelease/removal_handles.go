package installrelease

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// removalHandles pins every signed directory for one batch. Windows denies
// rename/delete sharing; Unix child lookup and unlink use the pinned dirfd.
// This is not authority: signed inventory/native transaction checks precede it.
type removalHandles struct {
	root string
	dirs map[string]*os.File
}

func openRemovalHandles(root string, p RemovalPlan) (*removalHandles, error) {
	h := &removalHandles{root: root, dirs: map[string]*os.File{}}
	var dirs []removalObject
	for _, o := range p.Objects {
		if o.Directory {
			dirs = append(dirs, o)
		}
	}
	sort.Slice(dirs, func(i, j int) bool {
		a, b := strings.Count(dirs[i].Path, "/"), strings.Count(dirs[j].Path, "/")
		if dirs[i].Path == "." {
			return true
		}
		if dirs[j].Path == "." {
			return false
		}
		if a != b {
			return a < b
		}
		return dirs[i].Path < dirs[j].Path
	})
	for _, o := range dirs {
		parent := h.dirs[path.Dir(o.Path)]
		f, err := platformRemovalOpen(root, o.Path, parent, true)
		if err != nil {
			h.close()
			return nil, ErrRelease
		}
		if err = checkRemovalHandle(f, o); err != nil {
			f.Close()
			h.close()
			return nil, err
		}
		h.dirs[o.Path] = f
	}
	return h, nil
}
func (h *removalHandles) close() {
	for _, f := range h.dirs {
		f.Close()
	}
}

func checkRemovalHandle(f *os.File, o removalObject) error {
	st, err := f.Stat()
	if err != nil || st.IsDir() != o.Directory || (!o.Directory && !st.Mode().IsRegular()) {
		return ErrRelease
	}
	id, err := platformIdentity("", f, st, o.Directory, true)
	if err != nil || id.volume != o.Volume || id.index != o.Index || (!o.Directory && id.links != 1) {
		return ErrRelease
	}
	if !o.Directory {
		if _, err = f.Seek(0, io.SeekStart); err != nil {
			return ErrRelease
		}
		hash := sha256.New()
		n, err := io.Copy(hash, io.LimitReader(f, maximumFile+1))
		if err != nil || n != o.Size || hex.EncodeToString(hash.Sum(nil)) != o.SHA256 {
			return ErrRelease
		}
	}
	return nil
}

// openFile does not follow the final link. The result remains open through
// validation and deletion; Windows disallows any concurrent write/rename handle.
func (h *removalHandles) openFile(o removalObject) (*os.File, error) {
	parent := h.dirs[path.Dir(o.Path)]
	if parent == nil || o.Directory {
		return nil, ErrRelease
	}
	f, err := platformRemovalOpen(h.root, o.Path, parent, false)
	if err != nil {
		return nil, err
	}
	if err = checkRemovalHandle(f, o); err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}

func (h *removalHandles) remove(f *os.File, o removalObject) error {
	if checkRemovalHandle(f, o) != nil {
		return ErrRelease
	}
	return platformRemovalDelete(h.dirs[path.Dir(o.Path)], f, filepath.Base(filepath.FromSlash(o.Path)))
}
