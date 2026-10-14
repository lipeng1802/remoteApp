package autonomousvalidation

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"runtime"
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/devicevalidation"
	"remoteapp.local/selfhost-poc/nativejournal"
	"remoteapp.local/selfhost-poc/trustinstall"
	"remoteapp.local/selfhost-poc/trustprofile"
)

func TestNativeProfileCleanupReadOnlyEmptyAndRecreate(t *testing.T) {
	exe := os.Getenv("REMOTEAPP_JOURNAL_TEST_EXECUTABLE")
	if exe == "" {
		t.Skip("native journal helper required; no simulated OS substitute")
	}
	helper, e := os.ReadFile(exe)
	if e != nil {
		t.Fatal("read pinned helper")
	}
	now := time.Now()
	clock := func() time.Time { return now }
	spec, _, e := devicevalidation.NewMaterial(helper, now)
	if e != nil {
		t.Fatal("public TEST profile")
	}
	encoded, e := trustinstall.EncodeForTrustedBuild(spec, now)
	if e != nil {
		t.Fatal("encode TEST profile")
	}
	release, e := trustinstall.DecodeCompiled(encoded)
	if e != nil {
		t.Fatal("decode TEST profile")
	}
	var raw [16]byte
	if _, e := rand.Read(raw[:]); e != nil {
		t.Fatal("random scope")
	}
	scope := hex.EncodeToString(raw[:])
	empty := func() bool {
		t.Helper()
		runtime.LockOSThread()
		ok, e := nativejournal.TestEmpty(scope)
		runtime.UnlockOSThread()
		if e != nil {
			t.Fatal("native TEST attributes unavailable; preserve evidence")
		}
		return ok
	}
	if !empty() {
		t.Fatal("fresh random namespace not empty")
	}
	for round := 0; round < 2; round++ {
		j, e := trustprofile.NewTestDesktopJournal(exe, scope, true)
		if e != nil {
			t.Fatal("fresh helper create rejected; preserve evidence")
		}
		installed, e := trustprofile.InitializeInstallation(release.Anchor(), release.Specification().Bundled, j, clock)
		if e != nil {
			j.Close()
			t.Fatal("native commit; preserve evidence")
		}
		if _, _, e = installed.Current(); e != nil || j.Close() != nil {
			t.Fatal("native commit/join; preserve evidence")
		}
		if empty() {
			t.Fatal("committed TEST namespace wrongly empty")
		}
		// Empty inspection must not change the committed native record.
		j, e = trustprofile.NewTestDesktopJournal(exe, scope, false)
		if e != nil {
			t.Fatal("native reopen; preserve evidence")
		}
		installed, e = trustprofile.OpenInstallation(release.Anchor(), j, clock)
		if e != nil {
			j.Close()
			t.Fatal("native reopen; preserve evidence")
		}
		if _, _, e = installed.Current(); e != nil || j.Close() != nil {
			t.Fatal("read-only inspection mutated record")
		}
		if cleanupProfile(exe, scope) != nil || !empty() {
			t.Fatal("exact joined cleanup not empty; preserve evidence")
		}
	}
}
