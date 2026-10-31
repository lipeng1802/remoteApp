package main

import (
	"errors"
	"io"
	"os"
	"path/filepath"
)

const maximumBuildInput = int64(100 << 20) // Same per-file ceiling as signed releases.
const maximumProfileInput = int64(64 << 10)
const offlineWorkBudget = int64(2 << 30)
const offlineBundleBudget = int64(512 << 20)
const maximumBudgetEntries = 10000

var errBuildInput = errors.New("test_build_input_rejected")
var errBuildBudget = errors.New("test_build_budget_exceeded_or_unreadable")

// Read only regular, non-link inputs, with a bounded allocation even if the
// file grows after Stat. No arbitrary input or OS error reaches diagnostics.
func readBuildInput(path string, limit int64) ([]byte, error) {
	before, err := os.Lstat(path)
	if limit < 1 || limit > maximumBuildInput || err != nil || !before.Mode().IsRegular() || before.Size() < 1 || before.Size() > limit {
		return nil, errBuildInput
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, errBuildInput
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(before, opened) || opened.Size() != before.Size() {
		return nil, errBuildInput
	}
	raw, err := io.ReadAll(io.LimitReader(f, limit+1))
	after, statErr := f.Stat()
	closeErr := f.Close()
	if err != nil || statErr != nil || closeErr != nil || int64(len(raw)) != before.Size() || after.Size() != before.Size() {
		return nil, errBuildInput
	}
	return raw, nil
}

// Counts logical bytes, refuses links/special entries/unreadable trees, and
// bounds traversal. This is a fail-closed observation, not a filesystem quota.
func checkTreeBudget(root string, limit int64, maxEntries int) (int64, error) {
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || limit < 0 || maxEntries < 1 {
		return 0, errBuildBudget
	}
	var total int64
	count := 1 // Includes the root, as did WalkDir.
	type directory struct {
		path string
		info os.FileInfo
	}
	pending := []directory{{root, info}}
	for len(pending) > 0 {
		current := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		err = func() error {
			before, e := os.Lstat(current.path)
			if e != nil || !before.IsDir() || before.Mode()&os.ModeSymlink != 0 || !os.SameFile(before, current.info) {
				return errBuildBudget
			}
			f, e := os.Open(current.path)
			if e != nil {
				return errBuildBudget
			}
			defer f.Close()
			opened, e := f.Stat()
			if e != nil || !os.SameFile(before, opened) {
				return errBuildBudget
			}
			for {
				entries, readErr := readBudgetEntries(f, maxEntries-count)
				if readErr != nil && readErr != io.EOF {
					return errBuildBudget
				}
				for _, entry := range entries {
					count++
					if count > maxEntries {
						return errBuildBudget
					}
					child, e := entry.Info()
					if e != nil || child.Mode()&os.ModeSymlink != 0 {
						return errBuildBudget
					}
					if child.IsDir() {
						pending = append(pending, directory{filepath.Join(current.path, entry.Name()), child})
						continue
					}
					if !child.Mode().IsRegular() || child.Size() < 0 || child.Size() > limit-total {
						return errBuildBudget
					}
					total += child.Size()
				}
				if readErr == io.EOF {
					break
				}
				if len(entries) == 0 {
					return errBuildBudget
				}
			}
			if f.Close() != nil {
				return errBuildBudget
			}
			return nil
		}()
		if err != nil {
			return 0, errBuildBudget
		}
	}
	return total, nil
}

// WalkDir reads a whole directory before its callback can enforce the count.
// Read at most a small batch, or one entry beyond the remaining budget to
// distinguish a full-but-complete tree from an over-budget tree.
func readBudgetEntries(reader interface {
	ReadDir(int) ([]os.DirEntry, error)
}, remaining int) ([]os.DirEntry, error) {
	if remaining < 0 {
		return nil, errBuildBudget
	}
	n := 64
	if remaining < 64 {
		n = remaining + 1
	}
	return reader.ReadDir(n)
}

func checkOfflineBuildBudget(dest string, before bool) error {
	limit := offlineWorkBudget
	if before {
		limit -= offlineBundleBudget
	}
	if _, err := checkTreeBudget(filepath.Dir(dest), limit, maximumBudgetEntries); err != nil {
		return err
	}
	if !before {
		_, err := checkTreeBudget(dest, offlineBundleBudget, maximumBudgetEntries)
		return err
	}
	return nil
}

func buildResourceReason(err error) string {
	for _, known := range []error{errBuildInput, errBuildBudget} {
		if errors.Is(err, known) {
			return known.Error()
		}
	}
	return ""
}
