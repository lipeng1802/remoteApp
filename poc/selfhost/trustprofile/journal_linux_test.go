//go:build linux

package trustprofile

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func rootJournalPath(t *testing.T) string {
	t.Helper()
	if os.Geteuid() != 0 {
		t.Skip("native protected journal requires Linux root")
	}
	d, e := os.MkdirTemp("/root", "remoteapp-trust-journal-test-")
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		if e := os.RemoveAll(d); e != nil {
			t.Error(e)
		}
	})
	return filepath.Join(d, "installation")
}

func journalChild(t *testing.T, path, mode string, a InstallationAnchor, old Profile, nonroot bool) {
	t.Helper()
	p, _ := json.Marshal(old)
	c := exec.Command(os.Args[0], "-test.run=^TestLinuxRootJournalChild$", "-test.v=true")
	c.Env = append(os.Environ(), "REMOTEAPP_JOURNAL_CHILD="+mode, "REMOTEAPP_JOURNAL_PATH="+path,
		"REMOTEAPP_JOURNAL_ROOT="+hex.EncodeToString(a.Root[:]), "REMOTEAPP_JOURNAL_OLD="+base64.StdEncoding.EncodeToString(p))
	if nonroot {
		c.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: 65534, Gid: 65534}}
	}
	b, e := c.CombinedOutput()
	if e != nil {
		t.Fatalf("child %s: %v %s", mode, e, b)
	}
}

func TestLinuxRootJournalChild(t *testing.T) {
	mode := os.Getenv("REMOTEAPP_JOURNAL_CHILD")
	if mode == "" {
		t.Skip("subprocess-only native entry")
	}
	j, e := OpenLinuxRootJournal(os.Getenv("REMOTEAPP_JOURNAL_PATH"))
	if mode == "denied" {
		if e == nil {
			j.Close()
			t.Fatal("unexpected ownership")
		}
		return
	}
	if e != nil {
		t.Fatal(e)
	}
	defer j.Close()
	b, _ := hex.DecodeString(os.Getenv("REMOTEAPP_JOURNAL_ROOT"))
	oldRaw, _ := base64.StdEncoding.DecodeString(os.Getenv("REMOTEAPP_JOURNAL_OLD"))
	var old Profile
	if json.Unmarshal(oldRaw, &old) != nil {
		t.Fatal("profile")
	}
	a := InstallationAnchor{Service: old.Claims.Service, MinimumEpoch: 1}
	copy(a.Root[:], b)
	s, e := OpenInstallation(a, j, time.Now)
	if e != nil {
		t.Fatal(e)
	}
	p, _, e := s.Current()
	if e != nil || p.Claims.Epoch != old.Claims.Epoch+1 {
		t.Fatal("durable floor not reopened")
	}
	if s.Accept(context.Background(), old, quiesceFunc(func(context.Context) error { t.Fatal("rollback cancelled sessions"); return nil })) == nil {
		t.Fatal("rollback accepted")
	}
}

func TestLinuxRootJournalNativeLifecycle(t *testing.T) {
	path := rootJournalPath(t)
	_, _, a, old, next := installationFixture(t)
	j, e := CreateLinuxRootJournal(path)
	if e != nil {
		t.Fatal(e)
	}
	defer j.Close()
	s, e := InitializeInstallation(a, old, j, time.Now)
	if e != nil {
		t.Fatal(e)
	}
	if other, e := OpenLinuxRootJournal(path); e == nil {
		other.Close()
		t.Fatal("second owner")
	}
	journalChild(t, path, "denied", a, old, false)
	// The uploaded test executable is public-readable; only root state is denied.
	journalChild(t, path, "denied", a, old, true)
	if s.Accept(context.Background(), next, quiesceFunc(func(context.Context) error { return nil })) != nil {
		t.Fatal("rotate")
	}
	if j.Close() != nil {
		t.Fatal("close")
	}
	if _, e := j.Load(); e == nil {
		t.Fatal("closed owner reads")
	}
	journalChild(t, path, "reopen", a, old, false)
	if other, e := CreateLinuxRootJournal(path); e == nil {
		other.Close()
		t.Fatal("installer reset")
	}
}

func TestLinuxRootJournalNativeRejectsResetAndUnsafeState(t *testing.T) {
	for _, mode := range []string{"incomplete", "missing-record", "missing-lock", "corrupt", "symlink", "permissions", "parent-permissions", "hardlink"} {
		t.Run(mode, func(t *testing.T) {
			path := rootJournalPath(t)
			j, e := CreateLinuxRootJournal(path)
			if e != nil {
				t.Fatal(e)
			}
			if mode != "incomplete" {
				_, _, a, old, _ := installationFixture(t)
				if _, e := InitializeInstallation(a, old, j, time.Now); e != nil {
					t.Fatal(e)
				}
			}
			j.Close()
			record := filepath.Join(path, "record")
			must := func(e error) {
				t.Helper()
				if e != nil {
					t.Fatal(e)
				}
			}
			switch mode {
			case "missing-record":
				must(os.Remove(record))
			case "missing-lock":
				must(os.Remove(filepath.Join(path, "owner.lock")))
			case "corrupt":
				must(os.WriteFile(record, []byte("{}"), 0600))
			case "symlink":
				must(os.Remove(record))
				must(os.Symlink("owner.lock", record))
			case "permissions":
				must(os.Chmod(record, 0644))
			case "parent-permissions":
				must(os.Chmod(filepath.Dir(path), 0777))
			case "hardlink":
				must(os.Link(record, filepath.Join(path, "alias")))
			}
			if other, e := OpenLinuxRootJournal(path); e == nil {
				other.Close()
				t.Fatal("unsafe recovery")
			}
			if other, e := CreateLinuxRootJournal(path); e == nil {
				other.Close()
				t.Fatal("unsafe reinitialize")
			}
		})
	}
}

func TestLinuxRootJournalNativeCASFailureLatches(t *testing.T) {
	path := rootJournalPath(t)
	_, _, a, old, _ := installationFixture(t)
	j, e := CreateLinuxRootJournal(path)
	if e != nil {
		t.Fatal(e)
	}
	defer j.Close()
	if _, e := InitializeInstallation(a, old, j, time.Now); e != nil {
		t.Fatal(e)
	}
	raw, e := j.Load()
	if e != nil {
		t.Fatal(e)
	}
	if j.Commit([]byte("wrong expected state"), raw) == nil {
		t.Fatal("CAS")
	}
	if _, e := j.Load(); e == nil {
		t.Fatal("failed owner usable")
	}
	j.Close()
	reopened, e := OpenLinuxRootJournal(path)
	if e != nil {
		t.Fatal(e)
	}
	defer reopened.Close()
	if _, e := OpenInstallation(a, reopened, time.Now); e != nil {
		t.Fatal("durable old state lost")
	}
}

func TestLinuxRootJournalNativeLiveFaultAndUnsafePath(t *testing.T) {
	for _, mode := range []string{"missing", "permissions", "directory-permissions", "missing-live-lock", "symlink-directory", "writable-ancestor"} {
		t.Run(mode, func(t *testing.T) {
			path := rootJournalPath(t)
			_, _, a, old, next := installationFixture(t)
			j, e := CreateLinuxRootJournal(path)
			if e != nil {
				t.Fatal(e)
			}
			defer j.Close()
			s, e := InitializeInstallation(a, old, j, time.Now)
			if e != nil {
				t.Fatal(e)
			}
			switch mode {
			case "symlink-directory":
				alias := filepath.Join(filepath.Dir(path), "link")
				if os.Symlink(path, alias) != nil {
					t.Fatal("symlink")
				}
				if other, e := OpenLinuxRootJournal(alias); e == nil {
					other.Close()
					t.Fatal("symlink traversed")
				}
				return
			case "writable-ancestor":
				if other, e := CreateLinuxRootJournal(filepath.Join("/tmp", filepath.Base(filepath.Dir(path)))); e == nil {
					other.Close()
					t.Fatal("untrusted ancestor")
				}
				return
			case "missing":
				if os.Remove(filepath.Join(path, "record")) != nil {
					t.Fatal("remove")
				}
			case "missing-live-lock":
				if os.Remove(filepath.Join(path, "owner.lock")) != nil {
					t.Fatal("remove lock")
				}
			case "directory-permissions":
				if os.Chmod(path, 0777) != nil {
					t.Fatal("chmod directory")
				}
			case "permissions":
				if os.Chmod(filepath.Join(path, "record"), 0644) != nil {
					t.Fatal("chmod")
				}
			}
			if s.Accept(context.Background(), next, quiesceFunc(func(context.Context) error { t.Fatal("fault should deny before join"); return nil })) == nil {
				t.Fatal("fault accepted")
			}
			if _, _, e := s.Current(); e == nil {
				t.Fatal("fault did not close")
			}
		})
	}
}
