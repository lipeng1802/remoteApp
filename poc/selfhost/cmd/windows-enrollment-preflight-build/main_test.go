package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestManifestHashesEveryRegularFile(t *testing.T) {
	dir := t.TempDir()
	if os.Mkdir(filepath.Join(dir, "registration"), 0700) != nil {
		t.Fatal("mkdir")
	}
	if write(filepath.Join(dir, "registration", "helper.exe"), []byte("fixture")) != nil {
		t.Fatal("write")
	}
	b, e := manifest(dir)
	if e != nil {
		t.Fatal(e)
	}
	var m map[string]string
	if json.Unmarshal(b, &m) != nil || len(m) != 1 || len(m["registration/helper.exe"]) != 64 {
		t.Fatal("manifest")
	}
	if write(filepath.Join(dir, "SHA256.json"), b) != nil {
		t.Fatal("write")
	}
	if _, e = manifest(dir); e == nil {
		t.Fatal("must refuse reusing completed bundle")
	}
}

func TestManifestRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	if os.Symlink("missing", filepath.Join(dir, "helper.exe")) != nil {
		t.Skip("symlinks unavailable")
	}
	if _, e := manifest(dir); e == nil {
		t.Fatal("link accepted")
	}
}
