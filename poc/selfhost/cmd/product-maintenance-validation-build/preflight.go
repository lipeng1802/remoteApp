package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Output and installation must be separate trees. In addition to lexical
// overlap, compare existing parent objects so aliases cannot create the same
// fresh root. This is an observation, not protection from concurrent changes.
func checkOfflineOutputPath(dest, root string) error {
	if runtime.GOOS == "windows" {
		for _, path := range []string{dest, root} {
			// Clean/Rel do not model Win32's trimming of trailing dots/spaces.
			// Refuse these spellings, including in ancestor components.
			for _, part := range strings.FieldsFunc(path, func(r rune) bool { return r == '\\' || r == '/' }) {
				if strings.TrimRight(part, " .") != part {
					return errBuildInput
				}
			}
		}
	}
	for _, pair := range [][2]string{{dest, root}, {root, dest}} {
		rel, err := filepath.Rel(pair[0], pair[1])
		if err != nil {
			// Different volumes cannot lexically overlap.
			continue
		}
		if rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return errBuildInput
		}
	}
	destParent, err := os.Stat(filepath.Dir(dest))
	if err != nil || !destParent.IsDir() {
		return errBuildInput
	}
	rootParent, err := os.Stat(filepath.Dir(root))
	if err != nil || !rootParent.IsDir() {
		return errBuildInput
	}
	sameName := filepath.Base(dest) == filepath.Base(root)
	if runtime.GOOS == "windows" {
		sameName = strings.EqualFold(filepath.Base(dest), filepath.Base(root))
	}
	if sameName && os.SameFile(destParent, rootParent) {
		return errBuildInput
	}
	return nil
}
