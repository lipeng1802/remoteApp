package invite

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func desktopFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{root: filepath.Join(t.TempDir(), "desktop-private"), now: time.Unix(1800000000, 0)}
	var err error
	f.s, err = OpenDesktopAuthorization(f.root, func() time.Time { return f.now })
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

func scopedClaim(t *testing.T, f *fixture, p DesktopPermission) Result {
	t.Helper()
	i := f.exec(t, f.owner, Body{Action: "invite", Permission: p})
	return f.exec(t, f.client, Body{Action: "claim", Permission: p, TargetCode: f.target.DeviceCode, Code: i.Code})
}

func TestDesktopApprovalScopesPersistenceAndRevocation(t *testing.T) {
	for _, scope := range []DesktopPermission{DesktopView, DesktopControl} {
		t.Run(string(scope), func(t *testing.T) {
			f := desktopFixture(t)
			otherScope := DesktopControl
			if scope == DesktopControl {
				otherScope = DesktopView
			}
			i := f.exec(t, f.owner, Body{Action: "invite", Permission: scope})
			if i.Permission != scope {
				t.Fatal("invitation scope missing")
			}
			for _, wrong := range []DesktopPermission{"", otherScope} {
				if _, err := f.s.Execute(f.envelope(f.client, Body{Action: "claim", Permission: wrong, TargetCode: f.target.DeviceCode, Code: i.Code})); err != ErrDenied {
					t.Fatal("controller claimed a different/implicit scope")
				}
			}
			r := f.exec(t, f.client, Body{Action: "claim", Permission: scope, TargetCode: f.target.DeviceCode, Code: i.Code})
			pending := f.exec(t, f.owner, Body{Action: "incoming"})
			if pending.Status != "pending" || pending.RequestID != r.RequestID || pending.ControllerID != f.controller.DeviceID || pending.Permission != scope || pending.Grant != nil {
				t.Fatal("pending consent context lost or premature grant")
			}
			for _, key := range []ed25519.PrivateKey{f.client, f.other} {
				if _, err := f.s.Execute(f.envelope(key, Body{Action: "approve", RequestID: r.RequestID, Allow: true, Permission: scope})); err != ErrDenied {
					t.Fatal("controller/third device approved desktop access")
				}
			}
			for _, wrong := range []DesktopPermission{"", otherScope} {
				if _, err := f.s.Execute(f.envelope(f.owner, Body{Action: "approve", RequestID: r.RequestID, Allow: true, Permission: wrong})); err != ErrDenied {
					t.Fatal("ambiguous or substituted approval scope accepted")
				}
			}
			approved := f.exec(t, f.owner, Body{Action: "approve", RequestID: r.RequestID, Allow: true, Permission: scope})
			g := *approved.Grant
			pub := f.s.PublicKey()
			if approved.Permission != scope || g.Claims.Permission != string(scope) || g.Claims.ExpiresAt-g.Claims.IssuedAt != 300 ||
				!VerifyDesktopGrant(pub, g, f.target.DeviceID, f.controller.DeviceID, scope, f.now) ||
				!f.s.AuthorizedDesktop(g, f.target.DeviceID, f.controller.DeviceID, scope) {
				t.Fatal("explicit scope was not signed/authorized")
			}
			if VerifyGrant(pub, g, f.target.DeviceID, f.controller.DeviceID, f.now) || f.s.Authorized(g, f.target.DeviceID, f.controller.DeviceID) ||
				VerifyDesktopGrant(pub, g, f.target.DeviceID, f.controller.DeviceID, otherScope, f.now) ||
				f.s.AuthorizedDesktop(g, f.target.DeviceID, f.controller.DeviceID, otherScope) ||
				VerifyDesktopGrant(pub, g, f.controller.DeviceID, f.target.DeviceID, scope, f.now) ||
				VerifyDesktopGrant(pub, g, f.target.DeviceID, f.controller.DeviceID, scope, f.now.Add(-time.Second)) {
				t.Fatal("scope, role or issue time bypass")
			}
			tampered := g
			tampered.Claims.Permission = string(otherScope)
			if VerifyDesktopGrant(pub, tampered, f.target.DeviceID, f.controller.DeviceID, otherScope, f.now) {
				t.Fatal("permission not bound to issuer signature")
			}
			if err := f.s.Close(); err != nil {
				t.Fatal(err)
			}
			var err error
			f.s, err = OpenDesktopAuthorization(f.root, func() time.Time { return f.now })
			if err != nil {
				t.Fatal(err)
			}
			status := f.exec(t, f.client, Body{Action: "status", RequestID: r.RequestID})
			if status.Permission != scope || status.Grant == nil || *status.Grant != g || !bytes.Equal(pub, f.s.PublicKey()) ||
				!f.s.AuthorizedDesktop(g, f.target.DeviceID, f.controller.DeviceID, scope) {
				t.Fatal("restart changed scope, issuer or grant")
			}
			// Existing network enrollment cannot turn a new device grant into
			// the old fixed TLS path, even with a valid device-signed intent.
			n, _ := randomHex(16)
			_, err = f.s.PlanProvision(g, SignProvision(f.owner, ProvisionBody{GrantID: g.Claims.ID, Nonce: n,
				IssuedAt: f.now.Unix(), NodeKey: "nodekey:" + strings.Repeat("a", 64)}))
			if err != ErrDenied || len(f.s.ProvisionRecords()) != 0 {
				t.Fatal("desktop grant entered legacy network provisioning")
			}
			binding := NetworkBinding{TargetNode: "nodekey:" + strings.Repeat("a", 64), ControllerNode: "nodekey:" + strings.Repeat("b", 64), TargetIP: "100.120.0.42", ControllerIP: "100.120.0.93"}
			if _, err = f.s.NetworkState(g, binding); err != ErrDenied {
				t.Fatal("desktop grant received a legacy network state")
			}
			f.exec(t, f.client, Body{Action: "revoke", GrantID: g.Claims.ID})
			status = f.exec(t, f.owner, Body{Action: "status", RequestID: r.RequestID})
			if status.Status != "revoked" || status.Grant != nil || f.s.AuthorizedDesktop(g, f.target.DeviceID, f.controller.DeviceID, scope) {
				t.Fatal("revocation ignored")
			}
			if !VerifyDesktopGrant(pub, g, f.target.DeviceID, f.controller.DeviceID, scope, f.now) {
				t.Fatal("offline signature confused with revocation")
			}
			f.now = f.now.Add(300 * time.Second)
			if VerifyDesktopGrant(pub, g, f.target.DeviceID, f.controller.DeviceID, scope, f.now) {
				t.Fatal("original grant expiry relaxed")
			}
		})
	}
}

func TestDesktopDefaultModeAndStoreModeAreNotAnUpgrade(t *testing.T) {
	for _, desktop := range []bool{false, true} {
		f := newFixture(t)
		if desktop {
			f = desktopFixture(t)
		}
		if !desktop {
			if _, err := f.s.Execute(f.envelope(f.owner, Body{Action: "invite", Permission: DesktopControl})); err != ErrDenied {
				t.Fatal("default authority issued a desktop invitation")
			}
		}
		// The old fixed path remains fixed even in the explicit desktop store.
		_, result := f.granted(t)
		if result.Permission != "" || !VerifyGrant(f.s.PublicKey(), *result.Grant, f.target.DeviceID, f.controller.DeviceID, f.now) ||
			VerifyDesktopGrant(f.s.PublicKey(), *result.Grant, f.target.DeviceID, f.controller.DeviceID, DesktopView, f.now) {
			t.Fatal("fixed payload was reinterpreted as desktop")
		}
		f.s.Close()
		before, _ := os.ReadFile(filepath.Join(f.root, "state.json"))
		var wrong *Store
		var err error
		if desktop {
			wrong, err = Open(f.root, func() time.Time { return f.now })
		} else {
			wrong, err = OpenDesktopAuthorization(f.root, func() time.Time { return f.now })
		}
		if wrong != nil || err != ErrStorage {
			t.Fatal("store mode changed silently")
		}
		after, _ := os.ReadFile(filepath.Join(f.root, "state.json"))
		if !bytes.Equal(before, after) {
			t.Fatal("failed mode open rewrote existing state")
		}
	}
}

func TestDesktopSignedRequestScopeAndDenial(t *testing.T) {
	f := desktopFixture(t)
	bad := f.envelope(f.owner, Body{Action: "invite", Permission: DesktopView})
	bad.Body.Permission = DesktopControl
	if _, err := f.s.Execute(bad); err != ErrDenied {
		t.Fatal("request signature did not bind scope")
	}
	for _, b := range []Body{
		{Action: "invite", Permission: "clipboard"}, {Action: "invite", Permission: "desktop-control "},
		{Action: "incoming", Permission: DesktopView}, {Action: "register", Permission: DesktopControl},
		{Action: "invite", Permission: DesktopControl, Allow: true},
		{Action: "invite", Permission: DesktopView, RequestID: strings.Repeat("a", 32)},
	} {
		if _, err := f.s.Execute(f.envelope(f.owner, b)); err != ErrDenied {
			t.Fatal("unknown or misplaced scope accepted")
		}
		// Denied authenticated attempts intentionally consume durable budgets.
		// Isolate each shape check from the unchanged invitation rotation limit.
		f.now = f.now.Add(61 * time.Second)
	}
	r := scopedClaim(t, f, DesktopControl)
	if _, err := f.s.Execute(f.envelope(f.owner, Body{Action: "approve", RequestID: r.RequestID, GrantID: strings.Repeat("b", 32)})); err != ErrDenied {
		t.Fatal("desktop decline accepted unrelated selectors")
	}
	// Declining access need not assert any positive scope.
	refused := f.exec(t, f.owner, Body{Action: "approve", RequestID: r.RequestID})
	if refused.Status != "denied" || refused.Grant != nil || refused.Permission != DesktopControl {
		t.Fatal("decline granted access or lost the original context")
	}
	if _, err := f.s.Execute(f.envelope(f.owner, Body{Action: "approve", RequestID: r.RequestID, Permission: DesktopControl, Allow: true})); err != ErrDenied {
		t.Fatal("declined desktop request resurrected")
	}
}

func TestDesktopSnapshotScopeMismatchIsRejected(t *testing.T) {
	f := desktopFixture(t)
	r := scopedClaim(t, f, DesktopControl)
	f.exec(t, f.owner, Body{Action: "approve", RequestID: r.RequestID, Permission: DesktopControl, Allow: true})
	f.s.Close()
	path := filepath.Join(f.root, "state.json")
	raw, _ := os.ReadFile(path)
	var state snapshot
	if json.Unmarshal(raw, &state) != nil {
		t.Fatal("snapshot decode failed")
	}
	req := state.Requests[r.RequestID]
	req.Permission = DesktopView
	state.Requests[r.RequestID] = req
	raw, _ = json.Marshal(state)
	if os.WriteFile(path, raw, 0600) != nil {
		t.Fatal("fixture snapshot write failed")
	}
	if reopened, err := OpenDesktopAuthorization(f.root, func() time.Time { return f.now }); err != ErrStorage || reopened != nil {
		t.Fatal("snapshot permission mismatched its signed grant")
	}
}

func TestDesktopLegacyWireBytesUnchanged(t *testing.T) {
	const legacy = "remoteapp-invite-request-v1\n" +
		`{"action":"invite","nonce":"","issued_at":0,"target_code":"","invite_id":"","code":"","request_id":"","grant_id":"","allow":false}`
	if string(requestBytes(Body{Action: "invite"})) != legacy {
		t.Fatal("empty desktop scope changed legacy signed bytes")
	}
	raw, err := json.Marshal(Result{Status: "pending"})
	if err != nil || string(raw) != `{"status":"pending"}` {
		t.Fatal("legacy result acquired an implicit desktop scope")
	}
}
