package autonomousbinding_test

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"remoteapp.local/selfhost-poc/autonomousbinding"
	"remoteapp.local/selfhost-poc/trustprofile"
	"strings"
	"testing"
)

func TestNativeIdentityIntentCASAndReadOnlyReopen(t *testing.T) {
	exe := os.Getenv("REMOTEAPP_JOURNAL_TEST_EXECUTABLE")
	if exe == "" {
		t.Skip("explicit native helper required")
	}
	auditDir := os.Getenv("REMOTEAPP_INTENT_TEST_AUDIT_DIR")
	if auditDir == "" || !filepath.IsAbs(auditDir) {
		t.Fatal("absolute isolated audit directory required before native side effects")
	}
	var entropy [16]byte
	if _, e := rand.Read(entropy[:]); e != nil {
		t.Fatal(e)
	}
	release := hex.EncodeToString(entropy[:])
	scope, _ := autonomousbinding.Scope(release)
	auditScope(t, auditDir, release, scope)
	t.Cleanup(func() {
		if e := trustprofile.DeleteTestDesktopJournal(exe, scope); e != nil {
			t.Error("exact TEST journal cleanup failed", e)
		}
	})
	factory := func(s string, create, test bool) (autonomousbinding.Journal, error) {
		j, e := trustprofile.NewRecordDesktopJournal(exe, s, create, test)
		if e != nil {
			t.Log("native journal open failure", e)
		}
		return j, e
	}
	i, e := autonomousbinding.BeginWithJournal(release, true, factory)
	if e != nil {
		t.Fatal(e)
	}
	if e = i.ReserveIdentity(strings.Repeat("2", 32)); e != nil {
		i.Close()
		t.Fatal(e)
	}
	if e = i.Close(); e != nil {
		t.Fatal(e)
	}
	found, e := autonomousbinding.CheckCreationIntentWithJournal(release, true, factory)
	if e != nil || !found {
		t.Fatal("native intent reopen", e)
	}
	// No recovery/create/delete of an OS identity is authorized by the intent.
	if s, e := autonomousbinding.OpenWithJournal(release, true, factory); e == nil {
		s.Close()
		t.Fatal("intent became runtime binding")
	}
	if _, e = rand.Read(entropy[:]); e != nil {
		t.Fatal(e)
	}
	finalRelease := hex.EncodeToString(entropy[:])
	finalScope, _ := autonomousbinding.Scope(finalRelease)
	auditScope(t, auditDir, finalRelease, finalScope)
	t.Cleanup(func() {
		if e := trustprofile.DeleteTestDesktopJournal(exe, finalScope); e != nil {
			t.Error(e)
		}
	})
	i, e = autonomousbinding.BeginWithJournal(finalRelease, true, factory)
	if e != nil {
		t.Fatal(e)
	}
	if e = i.ReserveIdentity(strings.Repeat("4", 32)); e != nil {
		i.Close()
		t.Fatal(e)
	}
	bound, e := i.Commit(strings.Repeat("4", 32), strings.Repeat("5", 64))
	if e != nil {
		i.Close()
		t.Fatal(e)
	}
	if e = bound.Close(); e != nil {
		t.Fatal(e)
	}
	found, e = autonomousbinding.CheckCreationIntentWithJournal(finalRelease, true, factory)
	if e != nil || found {
		t.Fatal("native final CAS reopen", e)
	}

}

// Public TEST namespace metadata, synchronously retained before any native write.
func auditScope(t *testing.T, dir, release, scope string) {
	t.Helper()
	f, e := os.OpenFile(filepath.Join(dir, scope+".json"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		t.Fatal(e)
	}
	raw, _ := json.Marshal(map[string]any{"version": 1, "test": true, "release_scope": release, "journal_scope": scope, "phase": "native_outcome_unknown"})
	_, e = f.Write(raw)
	if e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e != nil || ce != nil {
		t.Fatal("audit write failed")
	}
	d, e := os.Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	e = d.Sync()
	ce = d.Close()
	if e != nil || ce != nil {
		t.Fatal("audit directory sync failed")
	}
}
