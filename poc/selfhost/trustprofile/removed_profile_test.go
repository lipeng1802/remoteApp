package trustprofile

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestRemovedProfileComparisonCannotOpenOrResetAuthority(t *testing.T) {
	_, j, a, profile, other := installationFixture(t)
	s, e := OpenInstallation(a, &j, time.Now)
	if e != nil {
		t.Fatal(e)
	}
	if VerifyRemovedProfile(a, &j, profile, time.Now) == nil {
		t.Fatal("active install passed removed gate")
	}
	if e = s.BeginMaintenance(); e != nil {
		t.Fatal(e)
	}
	tx, e := OpenRemovalTransaction(a, &j, time.Now)
	if e != nil {
		t.Fatal(e)
	}
	if VerifyRemovedProfile(a, &j, profile, time.Now) == nil {
		t.Fatal("pending passed removed gate")
	}
	digest := strings.Repeat("a", 64)
	if tx.BindRemovalPlan(digest) != nil {
		t.Fatal("bind")
	}
	if VerifyRemovedProfile(a, &j, profile, time.Now) == nil {
		t.Fatal("deleting passed removed gate")
	}
	if tx.MarkFilesRemoved(digest) != nil {
		t.Fatal("complete")
	}
	before := append([]byte(nil), j.raw...)
	if VerifyRemovedProfile(a, &j, profile, time.Now) != nil {
		t.Fatal("exact native public profile rejected")
	}
	if VerifyRemovedProfile(a, &j, other, time.Now) == nil {
		t.Fatal("other signed profile admitted")
	}
	if VerifyRemovedProfile(a, &j, profile, func() time.Time { return time.Unix(profile.Claims.ExpiresAt, 0) }) == nil {
		t.Fatal("expired profile admitted")
	}
	if _, e = OpenInstallation(a, &j, time.Now); e != ErrMaintenance {
		t.Fatal("maintenance fence reopened", e)
	}
	if !bytes.Equal(before, j.raw) {
		t.Fatal("comparison rewrote trust")
	}
}
