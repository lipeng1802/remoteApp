package enrollmentstore

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

type memoryJournal struct {
	data                                      []byte
	fail, uncertain, mismatch, commitReadback bool
	closed                                    bool
	commits                                   int
}

func (j *memoryJournal) Load() ([]byte, error) {
	if j.closed || j.data == nil {
		return nil, ErrStorage
	}
	if j.mismatch {
		return []byte("wrong receipt"), nil
	}
	return append([]byte(nil), j.data...), nil
}
func (j *memoryJournal) Commit(expected, b []byte) error {
	if j.closed || !bytes.Equal(j.data, expected) {
		return ErrStorage
	}
	j.commits++
	if j.fail {
		return ErrStorage
	}
	j.data = append([]byte(nil), b...)
	if j.commitReadback {
		j.mismatch = true
	}
	if j.uncertain {
		return ErrStorage
	}
	return nil
}
func (j *memoryJournal) Close() error { j.closed = true; return nil }

type memoryDirectory struct{ empty, unsafe, closed bool }

func (d *memoryDirectory) Path() string { return "trusted-fixture" }
func (d *memoryDirectory) Check(empty bool) error {
	if d.unsafe || d.closed || (empty && !d.empty) {
		return ErrStorage
	}
	return nil
}
func (d *memoryDirectory) Close() error              { d.closed = true; return nil }
func (d *memoryDirectory) ClearTestMaterials() error { d.empty = true; return nil }
func fixture(t *testing.T) (*Store, *memoryJournal, *memoryDirectory) {
	t.Helper()
	j := &memoryJournal{}
	d := &memoryDirectory{empty: true}
	s, e := newStore(j, d, strings.Repeat("a", 32), true)
	if e != nil {
		t.Fatal(e)
	}
	return s, j, d
}
func reopen(t *testing.T, j *memoryJournal, d *memoryDirectory) *Store {
	t.Helper()
	j.closed = false
	d.closed = false
	s, e := newStore(j, d, strings.Repeat("a", 32), false)
	if e != nil {
		t.Fatal("reopen rejected valid evidence")
	}
	return s
}
func TestDurableAttemptAndRegisteredRemainTerminalAfterReopen(t *testing.T) {
	for _, registered := range []bool{false, true} {
		s, j, d := fixture(t)
		if s.Begin() != nil {
			t.Fatal("attempt not committed")
		}
		if registered {
			d.empty = false
			if s.Registered("nodekey:"+strings.Repeat("1", 64), "100.120.0.7") != nil {
				t.Fatal("result not committed")
			}
		}
		old := append([]byte(nil), j.data...)
		if s.Close() != nil {
			t.Fatal("close failed")
		}
		s = reopen(t, j, d)
		if s.Begin() == nil || !bytes.Equal(old, j.data) {
			t.Fatal("restart reset/retried prior outcome")
		}
		if s.Registered("nodekey:"+strings.Repeat("2", 64), "100.120.0.8") == nil && registered {
			t.Fatal("registered state overwritten")
		}
		s.Close()
	}
}
func TestCommitFailureAndUnknownReceiptCannotAuthorizeSideEffect(t *testing.T) {
	for _, fault := range []string{"write", "unknown", "readback"} {
		t.Run(fault, func(t *testing.T) {
			s, j, d := fixture(t)
			switch fault {
			case "write":
				j.fail = true
			case "unknown":
				j.uncertain = true
			case "readback":
				j.commitReadback = true
			}
			if s.Begin() == nil || !s.failed || s.Begin() == nil {
				t.Fatal("uncertain attempt authorized")
			}
			if fault == "unknown" {
				j.uncertain = false
				s.Close()
				s = reopen(t, j, d)
				if s.Begin() == nil {
					t.Fatal("committed unknown receipt replayed")
				}
			}
		})
	}
	s, j, d := fixture(t)
	s.Begin()
	j.uncertain = true
	if s.Registered("nodekey:"+strings.Repeat("1", 64), "100.120.0.7") == nil {
		t.Fatal("unknown completion passed")
	}
	j.uncertain = false
	s.Close()
	s = reopen(t, j, d)
	if s.State().Phase != "registered" || s.Begin() == nil {
		t.Fatal("unknown completion reset")
	}
}
func TestMissingCorruptOrAlteredMetadataAndUnexpectedFilesReject(t *testing.T) {
	for _, fault := range []string{"missing", "corrupt", "duplicate", "unknown", "trailing", "scope", "phase", "idle_files", "permissions"} {
		t.Run(fault, func(t *testing.T) {
			_, j, d := fixture(t)
			r := Record{Version: 1, Scope: strings.Repeat("a", 32), Phase: "idle"}
			switch fault {
			case "missing":
				j.data = nil
			case "corrupt":
				j.data = []byte("{}")
			case "duplicate":
				j.data = []byte(`{"version":1,"version":1}`)
			case "unknown":
				j.data = []byte(`{"version":1,"secret":"injected"}`)
			case "trailing":
				j.data = append(j.data, []byte("{}")...)
			case "scope":
				r.Scope = strings.Repeat("b", 32)
				j.data, _ = json.Marshal(r)
			case "phase":
				r.Phase = "cleaned"
				j.data, _ = json.Marshal(r)
			case "idle_files":
				d.empty = false
			case "permissions":
				d.unsafe = true
			}
			old := append([]byte(nil), j.data...)
			if _, e := newStore(j, d, strings.Repeat("a", 32), false); e == nil || !bytes.Equal(old, j.data) {
				t.Fatal("bad state repaired or accepted")
			}
		})
	}
	s, j, _ := fixture(t)
	j.data = append(j.data, ' ')
	if s.Check() == nil || !s.failed {
		t.Fatal("live mutation ignored")
	}
}
func TestStableScopeAndInvalidCompletionCannotAdvance(t *testing.T) {
	a, e := Scope(strings.Repeat("a", 32), strings.Repeat("b", 32), strings.Repeat("c", 64))
	if e != nil {
		t.Fatal(e)
	}
	b, _ := Scope(strings.Repeat("a", 32), strings.Repeat("b", 32), strings.Repeat("c", 64))
	if a != b {
		t.Fatal("unstable scope")
	}
	c, _ := Scope(strings.Repeat("a", 32), strings.Repeat("b", 32), strings.Repeat("d", 64))
	if a == c {
		t.Fatal("shared identity scope")
	}
	if _, e := Scope("injected-path", strings.Repeat("b", 32), strings.Repeat("c", 64)); e == nil {
		t.Fatal("untrusted scope accepted")
	}
	s, j, _ := fixture(t)
	old := append([]byte(nil), j.data...)
	if s.Registered("nodekey:"+strings.Repeat("1", 64), "100.120.0.7") == nil || !bytes.Equal(old, j.data) {
		t.Fatal("completion before attempt")
	}
	s.Begin()
	old = append([]byte(nil), j.data...)
	for _, v := range [][2]string{{"bad", "100.120.0.7"}, {"nodekey:" + strings.Repeat("0", 64), "100.120.0.7"}, {"nodekey:" + strings.Repeat("1", 64), "100.64.0.1"}, {"nodekey:" + strings.Repeat("1", 64), "100.120.0.255"}} {
		if s.Registered(v[0], v[1]) == nil || !bytes.Equal(old, j.data) {
			t.Fatal("bad completion committed")
		}
	}
	if s.CleanupTestAfterJoin() == nil {
		t.Fatal("production cleanup exposed")
	}
}
