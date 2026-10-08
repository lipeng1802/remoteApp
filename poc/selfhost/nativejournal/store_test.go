package nativejournal

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

type memoryValues struct {
	items     map[string][]byte
	fail      string
	uncertain bool
}

func (k *memoryValues) Read(n string) ([]byte, error) {
	b, ok := k.items[n]
	if !ok {
		return nil, ErrMissing
	}
	return append([]byte(nil), b...), nil
}
func (k *memoryValues) Write(n string, b []byte) error {
	if k.fail == n && !k.uncertain {
		return ErrUnavailable
	}
	k.items[n] = append([]byte(nil), b...)
	if k.fail == n {
		return ErrUnavailable
	}
	return nil
}
func (k *memoryValues) Remove(n string) error {
	if k.fail == "delete" {
		return ErrUnavailable
	}
	delete(k.items, n)
	return nil
}
func (k *memoryValues) Empty() (bool, error) { return len(k.items) == 0, nil }
func (k *memoryValues) ClearTest() error     { return nil }
func (k *memoryValues) Close() error         { return nil }
func memoryStore() (*Store, *memoryValues) {
	k := &memoryValues{items: map[string][]byte{"manifest": append([]byte(nil), pending...)}}
	return &Store{kv: k, fresh: true, test: true}, k
}
func TestNativeJournalManifestSelectionAndMaximumRecord(t *testing.T) {
	s, k := memoryStore()
	old := bytes.Repeat([]byte("a"), 8191)
	if s.Commit("", old) != nil {
		t.Fatal("max record")
	}
	if len(k.items) != 5 {
		t.Fatal("four bounded chunks plus manifest")
	}
	reopened := &Store{kv: k}
	b, e := reopened.Load()
	if e != nil || !bytes.Equal(b, old) {
		t.Fatal("reopen")
	}
	next := bytes.Repeat([]byte("b"), 5000)
	if reopened.Commit(Digest(old), next) != nil {
		t.Fatal("rotate")
	}
	if len(k.items) != 4 {
		t.Fatal("exact old chunk reclamation")
	}
	if _, e := decodeManifest([]byte(`{"version":1,"digest":"` + Digest(next) + `","size":5000,"SIZE":5000}`)); e == nil {
		t.Fatal("duplicate")
	}
	if reopened.Commit("wrong", next) == nil {
		t.Fatal("CAS")
	}
	if _, e := reopened.Load(); e == nil {
		t.Fatal("closed after CAS")
	}
}
func TestNativeJournalCommitFaultsNeverSelectPartialRecord(t *testing.T) {
	for _, mode := range []string{"chunk", "manifest", "uncertain-manifest", "delete"} {
		t.Run(mode, func(t *testing.T) {
			s, k := memoryStore()
			old := []byte("old")
			next := bytes.Repeat([]byte("next"), 1500)
			if s.Commit("", old) != nil {
				t.Fatal("setup")
			}
			switch mode {
			case "chunk":
				k.fail = partName(Digest(next), 1)
			case "manifest":
				k.fail = "manifest"
			case "uncertain-manifest":
				k.fail = "manifest"
				k.uncertain = true
			case "delete":
				k.fail = "delete"
			}
			if s.Commit(Digest(old), next) == nil {
				t.Fatal("fault ignored")
			}
			if _, e := s.Load(); e == nil {
				t.Fatal("not latched")
			}
			k.fail = ""
			b, e := (&Store{kv: k}).Load()
			if e != nil {
				t.Fatal(e)
			}
			want := old
			if mode == "uncertain-manifest" || mode == "delete" {
				want = next
			}
			if !bytes.Equal(b, want) {
				t.Fatal("selected partial record")
			}
		})
	}
}
func TestNativeJournalCorruptionAndPendingRejectRecovery(t *testing.T) {
	s, k := memoryStore()
	if _, e := s.Load(); !errors.Is(e, ErrFresh) {
		t.Fatal("new only")
	}
	if _, e := (&Store{kv: k}).Load(); e == nil {
		t.Fatal("pending reset")
	}
	if s.Commit("", []byte("data")) != nil {
		t.Fatal("init")
	}
	for _, mode := range []string{"missing", "corrupt", "chunk"} {
		copyKV := &memoryValues{items: map[string][]byte{}}
		for n, b := range k.items {
			copyKV.items[n] = append([]byte(nil), b...)
		}
		broken := &Store{kv: copyKV, test: true}
		if broken.FaultTest(mode) != nil {
			t.Fatal("fault setup")
		}
		if _, e := broken.Load(); e == nil {
			t.Fatal(mode)
		}
	}
	for _, name := range []string{"data/" + strings.Repeat("0", 64) + "/4", "../manifest", "manifest/anything"} {
		if validName(name) {
			t.Fatal("unsafe name")
		}
	}
	if (&Store{kv: k}).FaultTest("missing") == nil {
		t.Fatal("production fault enabled")
	}
}
