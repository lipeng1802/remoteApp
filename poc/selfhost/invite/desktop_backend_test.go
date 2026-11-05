package invite

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type desktopAuditDriver struct {
	*memoryDriver
	effects atomic.Int64
}

func (d *desktopAuditDriver) Mint(c context.Context, r ProvisionRecord) (RegistrationCredential, error) {
	d.effects.Add(1)
	return d.memoryDriver.Mint(c, r)
}
func (d *desktopAuditDriver) Observe(c context.Context, r ProvisionRecord) (ProvisionObservation, error) {
	d.effects.Add(1)
	o, e := d.memoryDriver.Observe(c, r)
	// Distinct real-adapter-like observations for concurrent fixture pairs.
	i := strings.IndexByte("abcdef", r.NodeKey[8])
	if i >= 0 {
		o.IP = "100.120.0." + strconv.Itoa(i+1)
	}
	return o, e
}
func (d *desktopAuditDriver) Verify(c context.Context, r ProvisionRecord, o ProvisionObservation) bool {
	d.effects.Add(1)
	return d.memoryDriver.Verify(c, r, o)
}
func (d *desktopAuditDriver) Cleanup(c context.Context, r ProvisionRecord) error {
	d.effects.Add(1)
	return d.memoryDriver.Cleanup(c, r)
}
func (d *desktopAuditDriver) ReplaceRules(c context.Context, r []NetworkRule) error {
	d.effects.Add(1)
	return d.memoryDriver.ReplaceRules(c, r)
}

func desktopBackendFixture(t *testing.T, scope DesktopPermission) (*fixture, Grant, GrantPolicy, *ProvisionBackend, *desktopAuditDriver) {
	t.Helper()
	f, g, p, _, _ := policyFixture(t, scope)
	d := &desktopAuditDriver{memoryDriver: &memoryDriver{resources: map[string]ProvisionRecord{}}}
	b, e := NewDesktopProvisionBackend(f.s, d)
	if e != nil {
		t.Fatal(e)
	}
	b.VerifierSource = func(context.Context) (string, error) { return "nodekey:" + strings.Repeat("9", 64), nil }
	if e = b.Reconcile(context.Background()); e != nil {
		t.Fatal(e)
	}
	return f, g, p, b, d
}

func TestDesktopBackendHTTPSApprovalToScopedNetworkProofsAndRetirement(t *testing.T) {
	for _, scope := range []DesktopPermission{DesktopView, DesktopControl} {
		t.Run(string(scope), func(t *testing.T) {
			f, g, policy, b, d := desktopBackendFixture(t, scope)
			d.resources["unrelated"] = ProvisionRecord{ID: "unrelated"}
			server := httptest.NewTLSServer(NetworkHandler(b))
			defer server.Close()
			clients := make([]*NetworkClient, 0, 2)
			for _, key := range []ed25519.PrivateKey{f.owner, f.client} {
				c, e := NewNetworkClient(server.URL, server.Certificate().Raw, key)
				if e != nil {
					t.Fatal(e)
				}
				c.now = func() time.Time { return f.now }
				clients = append(clients, c)
				defer c.Close()
			}
			challenge := strings.Repeat("c", 64)
			clockStart := time.Now()
			clock, e := clients[0].Request(context.Background(), NetworkRequestBody{Action: "grant_clock", Grant: g, Challenge: challenge})
			if e != nil || clock.Proof == nil {
				t.Fatal("scoped preprovision calibration", e)
			}
			if _, e = AuthorityClock(f.s.PublicKey(), *clock.Proof, g.Claims.ID, challenge, clockStart); e != nil {
				t.Fatal(e)
			}
			var secrets []string
			for i, c := range clients {
				key, node := f.owner, "a"
				if i == 1 {
					key, node = f.client, "b"
				}
				r, e := c.Request(context.Background(), NetworkRequestBody{Action: "enroll", Grant: g, Intent: intent(f, g, key, node)})
				if e != nil || r.Credential == nil || r.Credential.Secret == "" || r.Ticket == "" {
					t.Fatal("scoped enroll", e)
				}
				secrets = append(secrets, r.Credential.Secret)
				permit, e := c.Request(context.Background(), NetworkRequestBody{Action: "verifier", Grant: g, Ticket: r.Ticket, Challenge: challenge})
				device := g.Claims.Target
				if i == 1 {
					device = g.Claims.Controller
				}
				if e != nil || permit.Verifier == nil || !VerifyVerifierPermitForPolicy(policy, f.s.PublicKey(), *permit.Verifier, g, device, r.Ticket, "nodekey:"+strings.Repeat(node, 64), challenge, f.now) {
					t.Fatal("scoped verifier", e)
				}
				if _, e = c.Request(context.Background(), NetworkRequestBody{Action: "bind", Grant: g, Ticket: r.Ticket}); e != nil {
					t.Fatal("scoped bind", e)
				}
			}
			if e = b.Reconcile(context.Background()); e != nil {
				t.Fatal(e)
			}
			state, e := clients[0].Request(context.Background(), NetworkRequestBody{Action: "state", Grant: g})
			if e != nil || state.State == nil || !state.State.Claims.Active || !VerifyNetworkState(f.s.PublicKey(), *state.State, f.now) {
				t.Fatal("scoped active state", e)
			}
			binding := state.State.Claims.Binding
			if binding.TargetIP != "100.120.0.1" || binding.ControllerIP != "100.120.0.2" || state.State.Claims.ExpiresAt-state.State.Claims.IssuedAt != int64(3*time.Second) {
				t.Fatal("wrong binding/lease")
			}
			cert, rsaKey := identityCert(t, f.now)
			publish := identityPublish(t, f, g, cert, rsaKey, f.owner)
			if _, e = clients[0].Request(context.Background(), publish.Body); e != nil {
				t.Fatal("scoped certificate possession", e)
			}
			tls, e := clients[1].Request(context.Background(), NetworkRequestBody{Action: "tls_identity", Grant: g, Challenge: challenge})
			if e != nil || tls.TLS == nil || !VerifyTLSIdentityForPolicy(policy, f.s.PublicKey(), *tls.TLS, g, binding, challenge, f.now) || VerifyTLSIdentity(f.s.PublicKey(), *tls.TLS, g, binding, challenge, f.now) {
				t.Fatal("scoped TLS identity", e)
			}
			id, e := clients[0].Request(context.Background(), NetworkRequestBody{Action: "session_identity", Grant: g, Challenge: challenge})
			if e != nil || id.Session == nil || !VerifySessionIdentityForPolicy(policy, f.s.PublicKey(), *id.Session, g, binding, challenge, f.now) {
				t.Fatal("scoped registered identity", e)
			}
			raw, _ := os.ReadFile(filepath.Join(f.root, "state.json"))
			for _, secret := range secrets {
				if bytes.Contains(raw, []byte(secret)) {
					t.Fatal("credential persisted")
				}
			}
			watch, e := clients[1].StartStateWatchForPolicy(context.Background(), policy, f.s.PublicKey(), g, binding, clients[1].now)
			if e != nil {
				t.Fatal(e)
			}
			defer watch.Stop()
			if _, e = clients[0].DeviceRequest(context.Background(), Body{Action: "revoke", GrantID: g.Claims.ID}); e != nil {
				t.Fatal(e)
			}
			select {
			case <-watch.Done():
			case <-time.After(2 * time.Second):
				t.Fatal("scoped revoke not observed")
			}
			watch.Stop()
			if e = b.Reconcile(context.Background()); e != nil {
				t.Fatal("scoped cleanup", e)
			}
			if d.minted != 2 || d.cleaned != 2 || len(d.rules) != 0 || len(d.resources) != 1 {
				t.Fatal("cleanup scope or mint count changed")
			}
			retired, e := clients[0].DeviceRequest(context.Background(), Body{Action: "node_retirement", NodeKey: binding.TargetNode, Challenge: challenge})
			if e != nil || retired.Retirement == nil || !VerifyNodeRetirement(f.s.PublicKey(), *retired.Retirement, g.Claims.Target, binding.TargetNode, binding.TargetIP, challenge, f.now) {
				t.Fatal("scoped precise retirement", e)
			}
			issuer := f.s.PublicKey()
			f.s.Close()
			f.s, e = OpenDesktopAuthorization(f.root, func() time.Time { return f.now })
			if e != nil {
				t.Fatal(e)
			}
			b, e = NewDesktopProvisionBackend(f.s, d)
			if e != nil || b.Reconcile(context.Background()) != nil {
				t.Fatal("scoped restart", e)
			}
			if !bytes.Equal(issuer, f.s.PublicKey()) || f.s.AuthorizedDesktop(g, g.Claims.Target, g.Claims.Controller, scope) || d.minted != 2 || len(d.resources) != 1 {
				t.Fatal("restart resurrected state or changed issuer")
			}
		})
	}
}

func TestDesktopBackendModeMismatchHasNoDriverOrFenceEffects(t *testing.T) {
	f, g, _, b, d := desktopBackendFixture(t, DesktopControl)
	before := d.effects.Load()
	legacy := &ProvisionBackend{Store: f.s, Driver: d, ready: true, VerifierSource: b.VerifierSource}
	legacy.VerifierSource = func(context.Context) (string, error) {
		d.effects.Add(100)
		return "nodekey:" + strings.Repeat("9", 64), nil
	}
	if _, c, e := legacy.Enroll(context.Background(), g, intent(f, g, f.owner, "a")); e != ErrDenied || c.Secret != "" {
		t.Fatal("legacy wrapper enrolled desktop")
	}
	if legacy.Bind(context.Background(), strings.Repeat("a", 32)) != ErrDenied || legacy.Reconcile(context.Background()) != ErrDenied {
		t.Fatal("legacy wrapper used desktop driver")
	}
	if _, e := legacy.GrantClockProof(g, strings.Repeat("b", 64)); e != ErrDenied {
		t.Fatal("legacy clock scope widened")
	}
	if _, e := legacy.NetworkState(g); e != ErrDenied {
		t.Fatal("legacy state scope widened")
	}
	if _, e := legacy.VerifierPermit(context.Background(), g, g.Claims.Target, strings.Repeat("a", 32), strings.Repeat("b", 64)); e != ErrDenied {
		t.Fatal("legacy verifier source reached")
	}
	nonces := len(f.s.state.Replay)
	if _, e := legacy.ExecuteNetworkRequest(context.Background(), apiEnvelope(f, f.owner, g, "state")); e != ErrDenied || len(f.s.state.Replay) != nonces {
		t.Fatal("wrong-mode wrapper consumed another backend's request")
	}
	if _, e := legacy.StartMaintenance(context.Background(), time.Second, time.Second); e != ErrDenied {
		t.Fatal("legacy maintenance adopted desktop root")
	}
	legacy.Fence()
	if d.effects.Load() != before || !f.s.provisionReady {
		t.Fatal("wrong-mode wrapper altered shared projection/readiness")
	}
	if _, e := b.GrantClockProof(g, strings.Repeat("c", 64)); e != nil {
		t.Fatal("valid backend was fenced", e)
	}
	old := newFixture(t)
	raw, _ := os.ReadFile(filepath.Join(old.root, "state.json"))
	if candidate, e := NewDesktopProvisionBackend(old.s, d); e != ErrDenied || candidate != nil {
		t.Fatal("constructor upgraded fixed store")
	}
	after, _ := os.ReadFile(filepath.Join(old.root, "state.json"))
	if !bytes.Equal(raw, after) || d.effects.Load() != before {
		t.Fatal("mode rejection changed store or driver")
	}
	other := desktopFixture(t)
	b.Store = other.s
	if b.Reconcile(context.Background()) != ErrDenied || d.effects.Load() != before {
		t.Fatal("public Store rebinding transferred opt-in")
	}
}

func TestDesktopBackendUsesPersistedScopeNotJustIssuerSignature(t *testing.T) {
	f, g, _, b, d := desktopBackendFixture(t, DesktopControl)
	before := d.effects.Load()
	rawKey, _ := hex.DecodeString(f.s.state.PrivateKey)
	for _, scope := range []string{string(DesktopView), permission} {
		foreign := g
		foreign.Claims.Permission = scope
		foreign.Signature = hex.EncodeToString(ed25519.Sign(ed25519.PrivateKey(rawKey), grantBytes(foreign.Claims)))
		req := apiEnvelope(f, f.owner, foreign, "grant_clock")
		req.Body.Challenge = strings.Repeat("a", 64)
		req = SignNetworkRequest(f.owner, req.Body)
		if _, e := b.ExecuteNetworkRequest(context.Background(), req); e != ErrDenied {
			t.Fatal("unrecorded issuer-signed scope accepted")
		}
		if _, e := b.GrantClockProof(foreign, strings.Repeat("b", 64)); e != ErrDenied {
			t.Fatal("direct grant clock bypassed persisted scope")
		}
		if _, c, e := b.Enroll(context.Background(), foreign, intent(f, foreign, f.owner, "a")); e != ErrDenied || c.Secret != "" {
			t.Fatal("unrecorded scope minted credentials")
		}
	}
	if d.effects.Load() != before || len(f.s.ProvisionRecords()) != 0 {
		t.Fatal("scope substitution reached driver or durable intent")
	}
}

func TestDesktopBackendUnknownMintIsCleanedWithoutRemintAfterRestart(t *testing.T) {
	f, g, _, b, d := desktopBackendFixture(t, DesktopView)
	d.failMint = true
	ticket, c, e := b.Enroll(context.Background(), g, intent(f, g, f.owner, "a"))
	if e == nil || ticket == "" || c.Secret != "" || d.minted != 1 {
		t.Fatal("unknown mint exposed secret or lost intent")
	}
	d.failCleanup = true
	if b.Reconcile(context.Background()) == nil || d.minted != 1 || f.s.provisionReady {
		t.Fatal("unknown cleanup authorized or reminted")
	}
	f.s.Close()
	f.s, e = OpenDesktopAuthorization(f.root, func() time.Time { return f.now })
	if e != nil {
		t.Fatal(e)
	}
	b, e = NewDesktopProvisionBackend(f.s, d)
	if e != nil {
		t.Fatal(e)
	}
	d.failCleanup = false
	if b.Reconcile(context.Background()) != nil || d.minted != 1 || len(d.resources) != 0 {
		t.Fatal("restart retried mint or missed exact cleanup")
	}
	rows := f.s.ProvisionRecords()
	if len(rows) != 1 || rows[0].ID != ticket || rows[0].Status != "cleaned" {
		t.Fatal("unknown intent lost its final receipt")
	}
}

func TestDesktopBackendProjectionKeepsOtherScopesAndFixedGrants(t *testing.T) {
	f, g, _, b, d := desktopBackendFixture(t, DesktopView)
	bindPair(t, f, g, b)
	_, peer, _ := ed25519.GenerateKey(rand.Reader)
	f.exec(t, peer, Body{Action: "register"})
	inv := f.exec(t, f.other, Body{Action: "invite", Permission: DesktopControl})
	third := f.exec(t, f.other, Body{Action: "register"})
	req := f.exec(t, peer, Body{Action: "claim", TargetCode: third.DeviceCode, Code: inv.Code, Permission: DesktopControl})
	control := *f.exec(t, f.other, Body{Action: "approve", RequestID: req.RequestID, Allow: true, Permission: DesktopControl}).Grant
	a := enroll(t, f, control, b, f.other, "c")
	c := enroll(t, f, control, b, peer, "d")
	if b.Bind(context.Background(), a) != nil || b.Bind(context.Background(), c) != nil || b.Reconcile(context.Background()) != nil {
		t.Fatal("second scope binding")
	}
	_, owner3, _ := ed25519.GenerateKey(rand.Reader)
	_, peer3, _ := ed25519.GenerateKey(rand.Reader)
	target3 := f.exec(t, owner3, Body{Action: "register"})
	f.exec(t, peer3, Body{Action: "register"})
	inv = f.exec(t, owner3, Body{Action: "invite"})
	req = f.exec(t, peer3, Body{Action: "claim", TargetCode: target3.DeviceCode, Code: inv.Code})
	fixed := *f.exec(t, owner3, Body{Action: "approve", RequestID: req.RequestID, Allow: true}).Grant
	a = enroll(t, f, fixed, b, owner3, "e")
	c = enroll(t, f, fixed, b, peer3, "f")
	if b.Bind(context.Background(), a) != nil || b.Bind(context.Background(), c) != nil || b.Reconcile(context.Background()) != nil || len(d.rules) != 3 {
		t.Fatal("mixed scope projection dropped a grant")
	}
	issuer := f.s.PublicKey()
	f.s.Close()
	var err error
	f.s, err = OpenDesktopAuthorization(f.root, func() time.Time { return f.now })
	if err != nil {
		t.Fatal(err)
	}
	b, err = NewDesktopProvisionBackend(f.s, d)
	if err != nil || b.Reconcile(context.Background()) != nil || !bytes.Equal(issuer, f.s.PublicKey()) || len(d.rules) != 3 || d.minted != 6 || d.cleaned != 0 {
		t.Fatal("restart changed issuer or lost an approved scope", err)
	}
	f.exec(t, f.owner, Body{Action: "revoke", GrantID: g.Claims.ID})
	if b.Reconcile(context.Background()) != nil || len(d.rules) != 2 || d.cleaned != 2 || len(d.resources) != 4 {
		t.Fatal("one scope revocation removed unrelated scopes")
	}
	for _, remaining := range []Grant{control, fixed} {
		n, e := b.NetworkState(remaining)
		if e != nil || !n.Claims.Active {
			t.Fatal("other scope lost authorization", e)
		}
	}
}

func TestDesktopBackendUnknownScopedRecordCannotBeAdoptedOnOpen(t *testing.T) {
	f, g, _, b, d := desktopBackendFixture(t, DesktopControl)
	enroll(t, f, g, b, f.owner, "a")
	f.s.Close()
	path := filepath.Join(f.root, "state.json")
	raw, _ := os.ReadFile(path)
	var st snapshot
	json.Unmarshal(raw, &st)
	for id, row := range st.Provisions {
		row.Grant.Claims.Permission = string(DesktopView)
		st.Provisions[id] = row
	}
	raw, _ = json.Marshal(st)
	os.WriteFile(path, raw, 0600)
	before := d.effects.Load()
	if reopened, e := OpenDesktopAuthorization(f.root, func() time.Time { return f.now }); e != ErrStorage || reopened != nil {
		t.Fatal("floating scoped record adopted")
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(raw, after) || d.effects.Load() != before {
		t.Fatal("unknown scoped record rewritten or cleaned")
	}
}
