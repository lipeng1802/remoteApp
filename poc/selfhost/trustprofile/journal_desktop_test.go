package trustprofile

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
	"time"
)

func nativeJournalExecutable(t *testing.T) string {
	t.Helper()
	exe := os.Getenv("REMOTEAPP_JOURNAL_TEST_EXECUTABLE")
	if exe == "" {
		t.Skip("native journal executable required; no simulated OS substitute")
	}
	return exe
}
func nativeJournalScope(t *testing.T, exe string) string {
	t.Helper()
	var b [16]byte
	if _, e := rand.Read(b[:]); e != nil {
		t.Fatal(e)
	}
	scope := hex.EncodeToString(b[:])
	t.Cleanup(func() {
		if e := DeleteTestDesktopJournal(exe, scope); e != nil {
			t.Error("exact native test credential cleanup", e)
		}
	})
	return scope
}
func TestDesktopJournalNativeRestartFloorAndExclusiveOwner(t *testing.T) {
	exe := nativeJournalExecutable(t)
	scope := nativeJournalScope(t, exe)
	_, _, a, old, next := installationFixture(t)
	j, e := NewTestDesktopJournal(exe, scope, true)
	if e != nil {
		t.Fatal("native create", e)
	}
	defer j.Close()
	s, e := InitializeInstallation(a, old, j, time.Now)
	if e != nil {
		t.Fatal("initialize", e)
	}
	if second, e := NewTestDesktopJournal(exe, scope, false); e == nil {
		second.Close()
		t.Fatal("second helper owner")
	}
	if second, e := NewTestDesktopJournal(exe, scope, true); e == nil {
		second.Close()
		t.Fatal("create overwrites existing owner")
	}
	joined := false
	if e := s.Accept(context.Background(), next, quiesceFunc(func(context.Context) error { joined = true; return nil })); e != nil || !joined {
		t.Fatal("rotate", e)
	}
	if e := j.Close(); e != nil {
		t.Fatal("join helper", e)
	}
	if _, e := j.Load(); e == nil {
		t.Fatal("closed owner usable")
	}
	if reset, e := NewTestDesktopJournal(exe, scope, true); e == nil {
		reset.Close()
		t.Fatal("existing committed native state overwritten after owner exit")
	}
	reopened, e := NewTestDesktopJournal(exe, scope, false)
	if e != nil {
		t.Fatal("new native helper reopen", e)
	}
	defer reopened.Close()
	s, e = OpenInstallation(a, reopened, time.Now)
	if e != nil {
		t.Fatal(e)
	}
	p, _, e := s.Current()
	if e != nil || p != next {
		t.Fatal("durable floor")
	}
	if s.Accept(context.Background(), old, quiesceFunc(func(context.Context) error { t.Fatal("rollback joins"); return nil })) == nil {
		t.Fatal("downgrade")
	}
	if second, e := NewTestDesktopJournal(exe, scope, true); e == nil {
		second.Close()
		t.Fatal("reset")
	}
	wrong := a
	wrong.Root[0] ^= 1
	if _, e := OpenInstallation(wrong, reopened, time.Now); e == nil {
		t.Fatal("wrong installed root")
	}
}
func TestDesktopJournalNativeMissingCorruptAndPartialRejectReset(t *testing.T) {
	exe := nativeJournalExecutable(t)
	for _, mode := range []string{"missing", "corrupt", "chunk", "changedchunk"} {
		t.Run(mode, func(t *testing.T) {
			scope := nativeJournalScope(t, exe)
			_, _, a, old, next := installationFixture(t)
			j, e := NewTestDesktopJournal(exe, scope, true)
			if e != nil {
				t.Fatal(e)
			}
			defer j.Close()
			s, e := InitializeInstallation(a, old, j, time.Now)
			if e != nil {
				t.Fatal(e)
			}
			if j.FaultTest(mode) != nil {
				t.Fatal("native fault setup")
			}
			if _, _, e := s.Current(); e == nil {
				t.Fatal("fault not rejected")
			}
			if s.Accept(context.Background(), next, quiesceFunc(func(context.Context) error { return nil })) == nil {
				t.Fatal("fault recovers automatically")
			}
			j.Close()
			if reopened, e := NewTestDesktopJournal(exe, scope, false); e == nil {
				reopened.Close()
				t.Fatal("corrupt reopened")
			}
			if reset, e := NewTestDesktopJournal(exe, scope, true); e == nil {
				reset.Close()
				t.Fatal("state reset")
			}
		})
	}
}

func TestDesktopJournalNativeMaximumRecord(t *testing.T) {
	exe := nativeJournalExecutable(t)
	scope := nativeJournalScope(t, exe)
	_, _, a, old, _ := installationFixture(t)
	id, e := anchorID(a)
	if e != nil {
		t.Fatal(e)
	}
	b, e := json.Marshal(installationRecord{Version: 1, Anchor: id, Epoch: old.Claims.Epoch, AcceptedAt: time.Now().Unix(), Profile: old})
	if e != nil || len(b) > 8191 {
		t.Fatal("record")
	}
	b = append(b, bytes.Repeat([]byte(" "), 8191-len(b))...)
	j, e := NewTestDesktopJournal(exe, scope, true)
	if e != nil {
		t.Fatal(e)
	}
	defer j.Close()
	if j.Commit(nil, b) != nil {
		t.Fatal("8191 byte native commit")
	}
	if j.Close() != nil {
		t.Fatal("native stop")
	}
	next, e := NewTestDesktopJournal(exe, scope, false)
	if e != nil {
		t.Fatal("maximum reopen", e)
	}
	defer next.Close()
	got, e := next.Load()
	if e != nil || !bytes.Equal(got, b) {
		t.Fatal("maximum readback")
	}
	if _, e := OpenInstallation(a, next, time.Now); e != nil {
		t.Fatal("verified maximum record", e)
	}
}
func TestDesktopJournalNativeInterruptedInitializationAndCASFailure(t *testing.T) {
	exe := nativeJournalExecutable(t)
	t.Run("pending", func(t *testing.T) {
		scope := nativeJournalScope(t, exe)
		j, e := NewTestDesktopJournal(exe, scope, true)
		if e != nil {
			t.Fatal(e)
		}
		if j.Close() != nil {
			t.Fatal("stop pending owner")
		}
		for _, create := range []bool{false, true} {
			if next, e := NewTestDesktopJournal(exe, scope, create); e == nil {
				next.Close()
				t.Fatal("incomplete reset")
			}
		}
	})
	t.Run("CAS", func(t *testing.T) {
		scope := nativeJournalScope(t, exe)
		_, _, a, old, _ := installationFixture(t)
		j, e := NewTestDesktopJournal(exe, scope, true)
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
		if j.Commit([]byte("wrong previous record"), raw) == nil {
			t.Fatal("CAS")
		}
		if _, e := j.Load(); e == nil {
			t.Fatal("CAS owner usable")
		}
		j.Close()
		next, e := NewTestDesktopJournal(exe, scope, false)
		if e != nil {
			t.Fatal("old native record lost", e)
		}
		defer next.Close()
		if _, e := OpenInstallation(a, next, time.Now); e != nil {
			t.Fatal(e)
		}
	})
}
