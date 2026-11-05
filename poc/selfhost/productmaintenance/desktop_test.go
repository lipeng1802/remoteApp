package productmaintenance

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/autonomousbinding"
	"remoteapp.local/selfhost-poc/enrollmentstore"
	"remoteapp.local/selfhost-poc/installeddevice"
	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/privatepipe"
	"remoteapp.local/selfhost-poc/processowner"
	"remoteapp.local/selfhost-poc/trustinstall"
	"remoteapp.local/selfhost-poc/trustprofile"
)

func TestDesktopOwnerProcessFixture(t *testing.T) {
	if len(os.Args) < 3 || os.Args[len(os.Args)-3] != "desktop-owner-fixture" {
		return
	}
	scope, mode := os.Args[len(os.Args)-2], os.Args[len(os.Args)-1]
	r := privatepipe.NewReader(os.Stdin)
	enc := json.NewEncoder(os.Stdout)
	var open installeddevice.Request
	if r.Read(&open) != nil || !installeddevice.ValidAutonomousOpen(open) {
		os.Exit(2)
	}
	if mode == "duplicate" {
		os.Stdout.WriteString(`{"status":"ready","Permission":"desktop-control","Permission":"desktop-view"}` + "\n")
	} else if mode == "legacy" {
		enc.Encode(map[string]string{"status": "ready"})
	} else {
		enc.Encode(map[string]string{"status": "ready", "Permission": scope})
	}
	for {
		var req installeddevice.Request
		if r.Read(&req) != nil {
			os.Exit(3)
		}
		switch req.Op {
		case "stop":
			enc.Encode(map[string]string{"status": "closed"})
			os.Exit(0)
		case "device":
			d := invite.Result{Status: "registered", DeviceID: strings.Repeat("a", 64), DeviceCode: strings.Repeat("b", 12)}
			if req.Device.Action == "incoming" {
				d = invite.Result{Status: "pending", RequestID: strings.Repeat("c", 32), ControllerID: strings.Repeat("d", 64), Permission: invite.DesktopPermission(scope)}
			}
			if mode == "wrong-result" {
				d.Permission = "clipboard"
			}
			enc.Encode(map[string]any{"status": "device_result", "passed": true, "device": d})
		case "desktop_arm_stop":
			ack := map[string]any{"status": "desktop_stop_armed"}
			if mode == "mixed-stop" {
				ack["joined"] = true
			}
			enc.Encode(ack)
		case "provision":
			if mode == "slow" {
				time.Sleep(150 * time.Millisecond)
			}
			enc.Encode(map[string]any{"status": "network_watch", "passed": true, "active": true})
		default:
			os.Exit(4)
		}
	}
}

type desktopSigner struct {
	key    ed25519.PrivateKey
	closed atomic.Int32
}

func (s *desktopSigner) PublicKey() ed25519.PublicKey    { return s.key.Public().(ed25519.PublicKey) }
func (s *desktopSigner) Sign(raw []byte) ([]byte, error) { return ed25519.Sign(s.key, raw), nil }
func (s *desktopSigner) CloseJoined() error              { s.closed.Add(1); return nil }

type desktopBindingJournal struct {
	raw    []byte
	closed bool
}

func (j *desktopBindingJournal) Load() ([]byte, error) {
	if j.closed {
		return nil, ErrProduct
	}
	return append([]byte(nil), j.raw...), nil
}
func (j *desktopBindingJournal) Commit([]byte, []byte) error { return ErrProduct }
func (j *desktopBindingJournal) Close() error                { j.closed = true; return nil }

func startDesktopOwnerFixture(t *testing.T, scope invite.DesktopPermission, mode string, legacy bool) (*Controller, *desktopSigner, ed25519.PrivateKey, error) {
	t.Helper()
	now := time.Now().Truncate(time.Second)
	root, key, _ := ed25519.GenerateKey(rand.Reader)
	issuer, issuerKey, _ := ed25519.GenerateKey(rand.Reader)
	ca, caKey, _ := ed25519.GenerateKey(rand.Reader)
	cert := &x509.Certificate{SerialNumber: big.NewInt(1), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour)}
	der, e := x509.CreateCertificate(rand.Reader, cert, cert, ca, caKey)
	if e != nil {
		t.Fatal(e)
	}
	p, e := trustprofile.Sign(key, trustprofile.Claims{Version: 1, Service: "desktop-maintenance-test", Epoch: 1, Endpoint: "https://127.0.0.1:8444", Certificate: base64.StdEncoding.EncodeToString(der), Issuer: hex.EncodeToString(issuer), IssuedAt: now.Add(-time.Minute).Unix(), ExpiresAt: now.Add(20 * time.Minute).Unix()})
	if e != nil {
		t.Fatal(e)
	}
	// Unique channel per fixture; no native journal is opened by this test.
	var random [8]byte
	rand.Read(random[:])
	spec := trustinstall.Specification{Version: 1, Channel: "desktop-fixture-" + hex.EncodeToString(random[:]), Root: hex.EncodeToString(root), Service: p.Claims.Service, MinimumEpoch: 1, HelperDigest: strings.Repeat("1", 64), Bundled: p, Test: true}
	encoded, e := trustinstall.EncodeForTrustedBuild(spec, time.Now())
	if e != nil {
		t.Fatal(e)
	}
	release, e := trustinstall.DecodeCompiled(encoded)
	if e != nil {
		t.Fatal(e)
	}
	_, deviceKey, _ := ed25519.GenerateKey(rand.Reader)
	signer := &desktopSigner{key: deviceKey}
	record := autonomousbinding.Record{Version: 1, ReleaseScope: release.Scope(), VaultScope: strings.Repeat("1", 32), PublicKey: hex.EncodeToString(signer.PublicKey())}
	record.EnrollmentScope, e = enrollmentstore.Scope(record.ReleaseScope, record.VaultScope, record.PublicKey)
	if e != nil {
		t.Fatal(e)
	}
	raw, _ := json.Marshal(record)
	opened := false
	var inspection *desktopBindingJournal
	records := func(s string, create, test bool) (autonomousbinding.Journal, error) {
		want, _ := autonomousbinding.Scope(release.Scope())
		if s != want || create || !test {
			t.Error("binding replaced/created")
			return nil, ErrProduct
		}
		inspection = &desktopBindingJournal{raw: raw}
		return inspection, nil
	}
	exe, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	exe, e = filepath.EvalSymlinks(exe)
	if e != nil {
		t.Fatal(e)
	}
	bin, e := os.ReadFile(exe)
	if e != nil {
		t.Fatal(e)
	}
	hash := sha256.Sum256(bin)
	childSpec := processowner.Spec{Executable: exe, SHA256: hex.EncodeToString(hash[:]), Args: []string{"-test.run=^TestDesktopOwnerProcessFixture$", "desktop-owner-fixture", string(scope), mode}}
	open := func(r autonomousbinding.Record) (Signer, error) {
		if r != record || inspection == nil || !inspection.closed {
			t.Error("wrong identity or inspection lease not joined")
		}
		opened = true
		return signer, nil
	}
	trust := func(bool) (trustinstall.Journal, error) { return nil, ErrProduct }
	var c *Controller
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if legacy {
		c, e = StartTest(ctx, release, childSpec, trust, records, open)
	} else {
		c, e = StartDesktopTest(ctx, release, childSpec, trust, records, open, scope)
	}
	if !opened {
		t.Fatal("existing identity not inspected")
	}
	if c != nil {
		t.Cleanup(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			c.Release(ctx)
			select {
			case <-c.child.Done():
			default:
				t.Error("fixture not actually joined")
			}
		})
	}
	return c, signer, issuerKey, e
}

func desktopFixtureGrant(c *Controller, key ed25519.PrivateKey, scope invite.DesktopPermission) invite.Grant {
	id := sha256.Sum256(c.signer.PublicKey())
	now := time.Now().Unix()
	g := invite.Grant{Claims: invite.Claims{Version: 1, ID: strings.Repeat("a", 32), Target: hex.EncodeToString(id[:]), Controller: strings.Repeat("b", 64), Permission: string(scope), IssuedAt: now, ExpiresAt: now + 300}}
	raw, _ := json.Marshal(g.Claims)
	g.Signature = hex.EncodeToString(ed25519.Sign(key, append([]byte("remoteapp-invite-grant-v1\n"), raw...)))
	return g
}

func TestDesktopOwnerRequiresMatchingReadinessAndIndependentRevocationBeforeBusiness(t *testing.T) {
	for _, scope := range []invite.DesktopPermission{invite.DesktopView, invite.DesktopControl} {
		c, signer, key, e := startDesktopOwnerFixture(t, scope, "normal", false)
		if e != nil {
			t.Fatal(e)
		}
		register := installeddevice.Request{Op: "device", Device: invite.Body{Action: "register"}}
		if _, e = c.Forward(context.Background(), register); e == nil || c.broken {
			t.Fatal("business before independent channel / rejection poisoned owner")
		}
		if c.ConfigureRevocation(context.Background(), 0) != nil || c.ConfigureRevocation(context.Background(), 0) == nil {
			t.Fatal("channel setup not one-shot")
		}
		if _, e = c.Count(context.Background()); e == nil || c.broken {
			t.Fatal("PONG query reached/poisoned desktop child")
		}
		if _, e = c.Forward(context.Background(), register); e != nil {
			t.Fatal("scoped business rejected")
		}
		other := invite.DesktopView
		if scope == other {
			other = invite.DesktopControl
		}
		if _, e = c.Forward(context.Background(), installeddevice.Request{Op: "device", Device: invite.Body{Action: "invite", Permission: other}}); e == nil || c.broken {
			t.Fatal("scope switch admitted/poisoned")
		}
		wrong := desktopFixtureGrant(c, key, other)
		if c.revocation.ObserveContext(context.Background(), strings.Repeat("c", 32), wrong) == nil {
			t.Fatal("configured revocation adopted peer scope")
		}
		g := desktopFixtureGrant(c, key, scope)
		if c.revocation.ObserveContext(context.Background(), strings.Repeat("c", 32), g) != nil {
			t.Fatal("configured revocation lost local scope")
		}
		if signer.closed.Load() != 0 {
			t.Fatal("signer released before child")
		}
		if c.Release(context.Background()) != nil || signer.closed.Load() != 1 {
			t.Fatal("normal child/signer join")
		}
	}
	for _, mode := range []string{"legacy", "duplicate"} {
		c, signer, _, e := startDesktopOwnerFixture(t, invite.DesktopView, mode, false)
		if e == nil || c == nil || !c.broken || signer.closed.Load() != 0 {
			t.Fatal("mismatched readiness accepted/owner lost")
		}
	}
	if c, _, _, e := startDesktopOwnerFixture(t, invite.DesktopView, "normal", true); e == nil || c == nil {
		t.Fatal("fixed owner adopted scoped child")
	}
}

func TestDesktopOwnerMalformedReplyAndUnknownProvisionRetainSameSigner(t *testing.T) {
	c, signer, _, e := startDesktopOwnerFixture(t, invite.DesktopControl, "wrong-result", false)
	if e != nil {
		t.Fatal(e)
	}
	if c.ConfigureRevocation(context.Background(), 0) != nil {
		t.Fatal("channel setup")
	}
	if _, e = c.Forward(context.Background(), installeddevice.Request{Op: "device", Device: invite.Body{Action: "incoming"}}); e == nil || !c.broken || signer.closed.Load() != 0 || c.revocation == nil {
		t.Fatal("mixed scope reply released/rearmed owner")
	}
	late, signer, key, e := startDesktopOwnerFixture(t, invite.DesktopView, "slow", false)
	if e != nil {
		t.Fatal(e)
	}
	if late.ConfigureRevocation(context.Background(), 0) != nil {
		t.Fatal("channel setup")
	}
	g := desktopFixtureGrant(late, key, invite.DesktopView)
	if late.revocation.ObserveContext(context.Background(), strings.Repeat("c", 32), g) != nil {
		t.Fatal("grant observation")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, e = late.Forward(ctx, installeddevice.Request{Op: "provision", Network: invite.NetworkRequestBody{Grant: g}}); e == nil || !late.provisioned || !late.broken || signer.closed.Load() != 0 || late.revocation == nil {
		t.Fatal("uncertain provision lost pinned owner")
	}
	if late.revocation.Pin(g) == nil {
		t.Fatal("uncertain pin rearmed")
	}
	if late.Release(context.Background()) == nil || signer.closed.Load() != 1 {
		t.Fatal("late reply pretended zero success / signer not joined")
	}
}

func TestDesktopApprovedCancellationPinsWithoutProvision(t *testing.T) {
	c, signer, key, err := startDesktopOwnerFixture(t, invite.DesktopControl, "normal", false)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	revoke := &revokeFixture{unknown: true}
	c.revocation = revoke
	g := desktopFixtureGrant(c, key, invite.DesktopControl)
	request := strings.Repeat("c", 32)
	progress := installeddevice.DesktopProgress{Status: "desktop_status", Phase: "approved", RequestID: request, Grant: &g}
	if err := c.observeDesktopProgress(ctx, progress); err != nil {
		t.Fatal(err)
	}
	progress.RequestID = strings.Repeat("d", 32)
	if c.observeDesktopProgress(ctx, progress) == nil {
		t.Fatal("desktop request switched after the first observation")
	}
	if _, err := c.Forward(ctx, installeddevice.Request{Op: "revoke_session"}); err == nil ||
		revoke.pins != 1 || !c.desktopPreprovisionPinned || c.provisioned || signer.closed.Load() != 0 {
		t.Fatal("unknown pre-provision revoke was treated as joined or lost")
	}
	if _, err := c.Forward(ctx, installeddevice.Request{Op: "provision", Network: invite.NetworkRequestBody{Grant: g}}); err == nil {
		t.Fatal("revocation attempt allowed provision")
	}
	if c.AllowRetainedStop(ctx) == nil {
		t.Fatal("unknown revocation allowed stop")
	}
	revoke.unknown = false
	if _, err := c.Forward(ctx, installeddevice.Request{Op: "revoke_session"}); err != nil || revoke.pins != 1 {
		t.Fatal("explicit recovery repinned or lost its channel")
	}
	if _, err := c.Forward(ctx, installeddevice.Request{Op: "preprovision_terminal"}); err != nil || c.AllowRetainedStop(ctx) != nil {
		t.Fatal("independent terminal verification failed")
	}
}

func TestDesktopQuiesceNeedsExactStopArmAndActualExit(t *testing.T) {
	for _, mode := range []string{"normal", "mixed-stop"} {
		t.Run(mode, func(t *testing.T) {
			c, signer, _, err := startDesktopOwnerFixture(t, invite.DesktopControl, mode, false)
			if err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			c.revocation = &revokeFixture{}
			arm := installeddevice.Request{Op: "desktop_arm_stop"}
			quiesce := installeddevice.Request{Op: "quiesce_session"}
			if _, err := c.Forward(ctx, arm); err == nil || c.broken {
				t.Fatal("unprovisioned desktop armed")
			}
			c.provisioned = true
			if _, err := c.Forward(ctx, quiesce); err == nil || c.quiesced || c.broken {
				t.Fatal("unarmed desktop quiesced")
			}
			_, err = c.Forward(ctx, arm)
			if mode == "mixed-stop" {
				if err == nil || !c.broken || c.desktopStopArmed {
					t.Fatal("mixed reply accepted")
				}
				return
			}
			if err != nil || !c.desktopStopArmed || c.quiesced {
				t.Fatal("arm treated as join or lost")
			}
			if _, err := c.Forward(ctx, arm); err == nil || c.broken {
				t.Fatal("stop replay admitted")
			}
			if _, err := c.Forward(ctx, quiesce); err != nil {
				t.Fatal("armed child could not join")
			}
			select {
			case <-c.child.Done():
			default:
				t.Fatal("quiesce before exit")
			}
			if signer.closed.Load() != 0 {
				t.Fatal("independent revocation signer released")
			}
		})
	}
}
