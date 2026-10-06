package invite

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type fixture struct {
	s                    *Store
	root                 string
	now                  time.Time
	owner, client, other ed25519.PrivateKey
	target, controller   Result
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{root: filepath.Join(t.TempDir(), "private"), now: time.Unix(1800000000, 0)}
	var err error
	f.s, err = Open(f.root, func() time.Time { return f.now })
	if err != nil {
		t.Fatal(err)
	}
	_, f.owner, _ = ed25519.GenerateKey(rand.Reader)
	_, f.client, _ = ed25519.GenerateKey(rand.Reader)
	_, f.other, _ = ed25519.GenerateKey(rand.Reader)
	f.target = f.exec(t, f.owner, Body{Action: "register"})
	f.controller = f.exec(t, f.client, Body{Action: "register"})
	f.exec(t, f.other, Body{Action: "register"})
	t.Cleanup(func() { f.s.Close() })
	return f
}
func (f *fixture) envelope(key ed25519.PrivateKey, b Body) Envelope {
	b.Nonce, _ = randomHex(16)
	b.IssuedAt = f.now.Unix()
	return Sign(key, b)
}
func (f *fixture) exec(t *testing.T, key ed25519.PrivateKey, b Body) Result {
	t.Helper()
	r, err := f.s.Execute(f.envelope(key, b))
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func (f *fixture) invitation(t *testing.T) Result { return f.exec(t, f.owner, Body{Action: "invite"}) }
func (f *fixture) claim(t *testing.T, inv Result) Result {
	return f.exec(t, f.client, Body{Action: "claim", TargetCode: f.target.DeviceCode, InviteID: inv.InviteID, Code: inv.Code})
}
func (f *fixture) granted(t *testing.T) (Result, Result) {
	inv := f.invitation(t)
	req := f.claim(t, inv)
	r := f.exec(t, f.owner, Body{Action: "approve", RequestID: req.RequestID, Allow: true})
	return req, r
}
func (f *fixture) restart(t *testing.T) {
	t.Helper()
	if f.s.Close() != nil {
		t.Fatal("close_failed")
	}
	var err error
	f.s, err = Open(f.root, func() time.Time { return f.now })
	if err != nil {
		t.Fatal(err)
	}
}

func TestExplicitApprovalAndBinding(t *testing.T) {
	f := newFixture(t)
	inv := f.invitation(t)
	req := f.claim(t, inv)
	pending := f.exec(t, f.client, Body{Action: "status", RequestID: req.RequestID})
	if pending.Status != "pending" || pending.Grant != nil {
		t.Fatal("claim granted without approval")
	}
	if _, err := f.s.Execute(f.envelope(f.other, Body{Action: "approve", RequestID: req.RequestID, Allow: true})); err != ErrDenied {
		t.Fatal("third device approved")
	}
	approved := f.exec(t, f.owner, Body{Action: "approve", RequestID: req.RequestID, Allow: true})
	g := *approved.Grant
	pub := f.s.PublicKey()
	if !VerifyGrant(pub, g, f.target.DeviceID, f.controller.DeviceID, f.now) || !f.s.Authorized(g, f.target.DeviceID, f.controller.DeviceID) {
		t.Fatal("valid grant rejected")
	}
	if VerifyGrant(pub, g, f.controller.DeviceID, f.target.DeviceID, f.now) || f.s.Authorized(g, f.target.DeviceID, deviceID(f.other.Public().(ed25519.PublicKey))) {
		t.Fatal("device binding bypassed")
	}
	g.Claims.Permission = "input"
	if VerifyGrant(pub, g, f.target.DeviceID, f.controller.DeviceID, f.now) {
		t.Fatal("permission escalation accepted")
	}
	if _, err := f.s.Execute(f.envelope(f.other, Body{Action: "status", RequestID: req.RequestID})); err != ErrDenied {
		t.Fatal("third device read grant")
	}
}

func TestPersistenceAndRevocation(t *testing.T) {
	f := newFixture(t)
	req, approved := f.granted(t)
	g := *approved.Grant
	pub := f.s.PublicKey()
	f.restart(t)
	target := f.exec(t, f.owner, Body{Action: "register"})
	if target != f.target || !VerifyGrant(pub, g, f.target.DeviceID, f.controller.DeviceID, f.now) || !f.s.Authorized(g, f.target.DeviceID, f.controller.DeviceID) {
		t.Fatal("restart changed identity/grant")
	}
	if _, err := f.s.Execute(f.envelope(f.other, Body{Action: "revoke", GrantID: g.Claims.ID})); err != ErrDenied {
		t.Fatal("foreign device revoked")
	}
	f.exec(t, f.owner, Body{Action: "revoke", GrantID: g.Claims.ID})
	if f.s.Authorized(g, f.target.DeviceID, f.controller.DeviceID) {
		t.Fatal("revoked grant active")
	}
	// Offline signature verification cannot know revocation; callers need live authorization.
	if !VerifyGrant(pub, g, f.target.DeviceID, f.controller.DeviceID, f.now) {
		t.Fatal("test lost offline distinction")
	}
	f.restart(t)
	status := f.exec(t, f.client, Body{Action: "status", RequestID: req.RequestID})
	if status.Status != "revoked" || status.Grant != nil || f.s.Authorized(g, f.target.DeviceID, f.controller.DeviceID) {
		t.Fatal("revocation resurrected")
	}
	if _, err := f.s.Execute(f.envelope(f.owner, Body{Action: "approve", RequestID: req.RequestID, Allow: true})); err != ErrDenied {
		t.Fatal("old request reapproved")
	}
}

func TestDenialExpiryAndOneUse(t *testing.T) {
	f := newFixture(t)
	inv := f.invitation(t)
	req := f.claim(t, inv)
	if _, err := f.s.Execute(f.envelope(f.other, Body{Action: "claim", TargetCode: f.target.DeviceCode, InviteID: inv.InviteID, Code: inv.Code})); err != ErrDenied {
		t.Fatal("invite reused")
	}
	denied := f.exec(t, f.owner, Body{Action: "approve", RequestID: req.RequestID, Allow: false})
	if denied.Status != "denied" || denied.Grant != nil {
		t.Fatal("denial granted")
	}
	f.restart(t)
	if f.exec(t, f.client, Body{Action: "status", RequestID: req.RequestID}).Status != "denied" {
		t.Fatal("denial forgotten")
	}
	fresh := f.invitation(t)
	f.now = f.now.Add(120 * time.Second)
	if _, err := f.s.Execute(f.envelope(f.client, Body{Action: "claim", TargetCode: f.target.DeviceCode, InviteID: fresh.InviteID, Code: fresh.Code})); err != ErrDenied {
		t.Fatal("expired invite accepted")
	}
	_, grant := f.granted(t)
	f.now = f.now.Add(300 * time.Second)
	if f.s.Authorized(*grant.Grant, f.target.DeviceID, f.controller.DeviceID) {
		t.Fatal("expired grant accepted")
	}
}

func TestSignedRequestReplayAndTamper(t *testing.T) {
	f := newFixture(t)
	e := f.envelope(f.owner, Body{Action: "invite"})
	tampered := e
	tampered.Body.Action = "register"
	if _, err := f.s.Execute(tampered); err != ErrDenied {
		t.Fatal("modified action accepted")
	}
	if _, err := f.s.Execute(e); err != nil {
		t.Fatal(err)
	}
	f.restart(t)
	if _, err := f.s.Execute(e); err != ErrDenied {
		t.Fatal("request replay accepted after restart")
	}
	stale := f.envelope(f.owner, Body{Action: "invite"})
	f.now = f.now.Add(31 * time.Second)
	if _, err := f.s.Execute(stale); err != ErrDenied {
		t.Fatal("stale signed request accepted")
	}
	f.now = f.now.Add(-60 * time.Second)
	if _, err := f.s.Execute(f.envelope(f.owner, Body{Action: "invite"})); err != ErrDenied {
		t.Fatal("clock rollback accepted")
	}
}

func TestGuessRateLimitSurvivesRestart(t *testing.T) {
	f := newFixture(t)
	inv := f.invitation(t)
	bad := "00000000"
	if inv.Code == bad {
		bad = "11111111"
	}
	for i := 0; i < 5; i++ {
		if _, err := f.s.Execute(f.envelope(f.client, Body{Action: "claim", TargetCode: f.target.DeviceCode, InviteID: inv.InviteID, Code: bad})); err != ErrDenied {
			t.Fatal("wrong code accepted")
		}
	}
	f.restart(t)
	if _, err := f.s.Execute(f.envelope(f.client, Body{Action: "claim", TargetCode: f.target.DeviceCode, InviteID: inv.InviteID, Code: inv.Code})); err != ErrDenied {
		t.Fatal("device budget forgotten")
	}
	if _, err := f.s.Execute(f.envelope(f.other, Body{Action: "claim", TargetCode: f.target.DeviceCode, InviteID: inv.InviteID, Code: inv.Code})); err != ErrDenied {
		t.Fatal("invite budget bypassed by other device")
	}
	f.now = f.now.Add(60 * time.Second)
	f.claim(t, inv)
}

func TestConcurrentClaimOnlyOneWins(t *testing.T) {
	f := newFixture(t)
	inv := f.invitation(t)
	var wg sync.WaitGroup
	accepted := make(chan bool, 2)
	for _, key := range []ed25519.PrivateKey{f.client, f.other} {
		e := f.envelope(key, Body{Action: "claim", TargetCode: f.target.DeviceCode, InviteID: inv.InviteID, Code: inv.Code})
		wg.Add(1)
		go func() { defer wg.Done(); _, err := f.s.Execute(e); accepted <- err == nil }()
	}
	wg.Wait()
	close(accepted)
	count := 0
	for ok := range accepted {
		if ok {
			count++
		}
	}
	if count != 1 {
		t.Fatal("concurrent invite consumed multiple times")
	}
}

func TestPrivateStorageAndFailClosed(t *testing.T) {
	f := newFixture(t)
	inv := f.invitation(t)
	raw, err := os.ReadFile(filepath.Join(f.root, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), `"`+inv.Code+`"`) {
		t.Fatal("plaintext invite code persisted")
	}
	info, _ := os.Stat(filepath.Join(f.root, "state.json"))
	if info.Mode().Perm() != 0600 {
		t.Fatal("state permission")
	}
	if _, err := Open(f.root, func() time.Time { return f.now }); err != ErrStorage {
		t.Fatal("second writer accepted")
	}
	if err := f.s.Close(); err != nil {
		t.Fatal(err)
	}
	// This test deliberately corrupts only its own fixture snapshot.
	if err := os.WriteFile(filepath.Join(f.root, "state.json"), []byte(`{"Version":1}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(f.root, func() time.Time { return f.now }); err != ErrStorage {
		t.Fatal("corrupt state regenerated")
	}
}

func TestDeviceCodeOnlyAndSupersededInvite(t *testing.T) {
	f := newFixture(t)
	old := f.invitation(t)
	oldRequest := f.claim(t, old)
	fresh := f.invitation(t)
	if f.exec(t, f.client, Body{Action: "status", RequestID: oldRequest.RequestID}).Status != "denied" {
		t.Fatal("superseded request still pending")
	}
	if _, err := f.s.Execute(f.envelope(f.owner, Body{Action: "approve", RequestID: oldRequest.RequestID, Allow: true})); err != ErrDenied {
		t.Fatal("superseded request approved")
	}
	request := f.exec(t, f.client, Body{Action: "claim", TargetCode: f.target.DeviceCode, Code: fresh.Code})
	if request.Status != "pending" {
		t.Fatal("code-only claim failed")
	}
}

func TestSingleActiveGrantPerTarget(t *testing.T) {
	f := newFixture(t)
	_, approved := f.granted(t)
	inv := f.invitation(t)
	req := f.exec(t, f.other, Body{Action: "claim", TargetCode: f.target.DeviceCode, Code: inv.Code})
	if _, err := f.s.Execute(f.envelope(f.owner, Body{Action: "approve", RequestID: req.RequestID, Allow: true})); err != ErrDenied {
		t.Fatal("second active grant issued")
	}
	f.exec(t, f.owner, Body{Action: "revoke", GrantID: approved.Grant.Claims.ID})
	f.exec(t, f.owner, Body{Action: "approve", RequestID: req.RequestID, Allow: true})
}

func TestResultDoesNotMutateStoredGrant(t *testing.T) {
	f := newFixture(t)
	req, r := f.granted(t)
	original := *r.Grant
	r.Grant.Claims.Permission = "input"
	persisted := f.exec(t, f.client, Body{Action: "status", RequestID: req.RequestID})
	if *persisted.Grant != original {
		t.Fatal("caller mutated backend state")
	}
}

func TestBodyBoundsAndMissingIdentity(t *testing.T) {
	f := newFixture(t)
	_, unknown, _ := ed25519.GenerateKey(rand.Reader)
	if _, err := f.s.Execute(f.envelope(unknown, Body{Action: "invite"})); err != ErrDenied {
		t.Fatal("unregistered device invited")
	}
	if _, err := f.s.Execute(f.envelope(f.owner, Body{Action: strings.Repeat("x", 5000)})); err != ErrDenied {
		t.Fatal("oversized body accepted")
	}
	e := f.envelope(f.owner, Body{Action: "invite"})
	e.Signature = "00"
	if _, err := f.s.Execute(e); err != ErrDenied {
		t.Fatal("bad signature accepted")
	}
	if bytes, _ := json.Marshal(Result{}); strings.Contains(string(bytes), "PrivateKey") {
		t.Fatal("private field in result")
	}
}

func TestCommitFailureDisablesAuthorization(t *testing.T) {
	f := newFixture(t)
	_, approved := f.granted(t)
	path := filepath.Join(f.root, "state.json")
	if os.Rename(path, filepath.Join(f.root, "fixture-backup.json")) != nil || os.Mkdir(path, 0700) != nil {
		t.Fatal("fault setup failed")
	}
	if _, err := f.s.Execute(f.envelope(f.owner, Body{Action: "revoke", GrantID: approved.Grant.Claims.ID})); err != ErrStorage {
		t.Fatal("failed commit reported success")
	}
	if f.s.Authorized(*approved.Grant, f.target.DeviceID, f.controller.DeviceID) {
		t.Fatal("storage failure left live authorization")
	}
	if _, err := f.s.Execute(f.envelope(f.owner, Body{Action: "register"})); err != ErrStorage {
		t.Fatal("broken store continued")
	}
}

func TestMissingSnapshotDoesNotRotateIssuer(t *testing.T) {
	f := newFixture(t)
	if f.s.Close() != nil || os.Remove(filepath.Join(f.root, "state.json")) != nil {
		t.Fatal("fault setup failed")
	}
	if _, err := Open(f.root, func() time.Time { return f.now }); err != ErrStorage {
		t.Fatal("missing identity silently reset")
	}
}
