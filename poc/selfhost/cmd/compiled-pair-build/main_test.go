package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPairBuildInputRejectsOverrideAndMultipleDocuments(t *testing.T) {
	for _, raw := range []string{`{"version":1,"root":"peer-root"}`, `{"version":1,"issuer":"a","issuer":"b"}`, `{"version":1} {"version":1}`, `{"version":1,"private_key":"secret"}`} {
		file := filepath.Join(t.TempDir(), "public.json")
		if e := os.WriteFile(file, []byte(raw), 0600); e != nil {
			t.Fatal(e)
		}
		if _, e := publicInput(file); e == nil {
			t.Fatal("untrusted extension accepted")
		}
	}
}

func TestPairBuildInputAcceptsOneDocumentTrailingNewline(t *testing.T) {
	file := filepath.Join(t.TempDir(), "public.json")
	if e := os.WriteFile(file, []byte("{\"version\":1}\n"), 0600); e != nil {
		t.Fatal(e)
	}
	if value, e := publicInput(file); e != nil || value.Version != 1 {
		t.Fatal("canonical text-file input rejected")
	}
}
