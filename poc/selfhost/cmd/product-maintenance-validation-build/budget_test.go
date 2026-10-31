package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

type budgetReadProbe struct{ requested int }

func (p *budgetReadProbe) ReadDir(n int) ([]os.DirEntry, error) { p.requested = n; return nil, io.EOF }

func TestBudgetDirectoryReadIsBoundedBeforeEnumeration(t *testing.T) {
	for _, c := range []struct{ remaining, want int }{{0, 1}, {1, 2}, {63, 64}, {64, 64}, {10000, 64}} {
		p := new(budgetReadProbe)
		if _, err := readBudgetEntries(p, c.remaining); err != io.EOF || p.requested != c.want {
			t.Fatal(c, p.requested, err)
		}
	}
	p := new(budgetReadProbe)
	if _, err := readBudgetEntries(p, -1); err != errBuildBudget || p.requested != 0 {
		t.Fatal("invalid budget reached reader")
	}
}

func TestBudgetMultipleBatchesAndFailureReleaseDirectory(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 129; i++ {
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("f%03d", i)), []byte("x"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if n, err := checkTreeBudget(dir, 129, 130); err != nil || n != 129 {
		t.Fatal("multi-batch count", n, err)
	}
	if _, err := checkTreeBudget(dir, 129, 129); err != errBuildBudget {
		t.Fatal("one excess entry accepted")
	}
	if _, err := checkTreeBudget(dir, 128, 130); err != errBuildBudget {
		t.Fatal("one excess byte accepted")
	}
	if err := os.Rename(dir, dir+"-released"); err != nil {
		t.Fatal("failed scan leaked handle", err)
	}
	if err := os.Rename(dir+"-released", dir); err != nil {
		t.Fatal(err)
	}
}

func TestBuildInputBoundsAndInvalidFiles(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "input")
	if err := os.WriteFile(file, []byte("12345678"), 0600); err != nil {
		t.Fatal(err)
	}
	b, err := readBuildInput(file, 8)
	if err != nil || string(b) != "12345678" {
		t.Fatal(err)
	}
	for _, limit := range []int64{0, 7, maximumBuildInput + 1} {
		if _, err = readBuildInput(file, limit); !errors.Is(err, errBuildInput) {
			t.Fatal("limit accepted")
		}
	}
	for _, p := range []string{dir, filepath.Join(dir, "missing")} {
		if _, err = readBuildInput(p, 8); !errors.Is(err, errBuildInput) {
			t.Fatal("bad input accepted")
		}
	}
	if err = os.WriteFile(file, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = readBuildInput(file, 8); !errors.Is(err, errBuildInput) {
		t.Fatal("empty input accepted")
	}
}

func TestBuildTreeBudgetCountsNestedFilesAndStopsAtLimit(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "file"), []byte("12345678"), 0600); err != nil {
		t.Fatal(err)
	}
	n, err := checkTreeBudget(dir, 8, 3)
	if err != nil || n != 8 {
		t.Fatal(n, err)
	}
	for _, v := range []struct {
		bytes   int64
		entries int
	}{{7, 3}, {8, 2}, {-1, 3}, {8, 0}} {
		if _, err = checkTreeBudget(dir, v.bytes, v.entries); !errors.Is(err, errBuildBudget) {
			t.Fatal("budget accepted", v)
		}
	}
	b, err := os.ReadFile(filepath.Join(sub, "file"))
	if err != nil || string(b) != "12345678" {
		t.Fatal("budget check changed file")
	}
}

func TestBuildResourceDiagnosticsHideRawFailures(t *testing.T) {
	if buildResourceReason(errors.New("PRIVATE")) != "" {
		t.Fatal("raw error accepted")
	}
	if _, err := checkTreeBudget(filepath.Join(t.TempDir(), "missing"), 8, 3); buildResourceReason(err) != errBuildBudget.Error() {
		t.Fatal("missing tree not rejected")
	}
}

func TestBuildInputsAndBudgetRefuseSymlinks(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "source")
	if err := os.WriteFile(file, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(file, link); err != nil {
		t.Skip("native symlink creation unavailable")
	}
	if _, err := readBuildInput(link, 8); !errors.Is(err, errBuildInput) {
		t.Fatal("input symlink accepted")
	}
	if _, err := checkTreeBudget(dir, 8, 10); !errors.Is(err, errBuildBudget) {
		t.Fatal("tree symlink accepted")
	}
}

func TestBadInputRejectedBeforeBundleCreation(t *testing.T) {
	previous := os.Args
	t.Cleanup(func() { os.Args = previous })
	dir := t.TempDir()
	dest := filepath.Join(dir, "bundle")
	missing := filepath.Join(dir, "go-not-run")
	os.Args = []string{"builder", missing, dir, runtime.GOOS, dest, dir, dir, dir, "--offline-install-root", filepath.Join(dir, "install")}
	if err := run(); !errors.Is(err, errBuildInput) {
		t.Fatal("input rejection not reached", err)
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatal("invalid input left bundle")
	}
}
