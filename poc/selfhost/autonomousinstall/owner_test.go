package autonomousinstall

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"math/big"
	"strings"
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/trustinstall"
	"remoteapp.local/selfhost-poc/trustprofile"
)

func releases(t *testing.T, digest string) (trustinstall.Release, trustinstall.Release) {
	t.Helper()
	now := time.Now().Truncate(time.Second)
	pub, key, _ := ed25519.GenerateKey(rand.Reader)
	cp, ck, _ := ed25519.GenerateKey(rand.Reader)
	ca := &x509.Certificate{SerialNumber: big.NewInt(1), IsCA: true, BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageCertSign, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(24 * time.Hour)}
	der, err := x509.CreateCertificate(rand.Reader, ca, ca, cp, ck)
	if err != nil {
		t.Fatal(err)
	}
	var nonce [16]byte
	_, _ = rand.Read(nonce[:])
	s := trustinstall.Specification{Version: 1, Channel: "autonomous-install-test-" + hex.EncodeToString(nonce[:]),
		Root: hex.EncodeToString(pub), Service: "autonomous-install-test", MinimumEpoch: 1, HelperDigest: digest, Test: true}
	c := trustprofile.Claims{Version: 1, Service: s.Service, Epoch: 1, Endpoint: "https://127.0.0.1:18443",
		Certificate: base64.StdEncoding.EncodeToString(der), Issuer: hex.EncodeToString(cp),
		IssuedAt: now.Add(-time.Minute).Unix(), ExpiresAt: now.Add(time.Hour).Unix()}
	makeRelease := func(epoch uint64) trustinstall.Release {
		c.Epoch = epoch
		s.MinimumEpoch = epoch
		s.Bundled, err = trustprofile.Sign(key, c)
		if err != nil {
			t.Fatal(err)
		}
		encoded, err := trustinstall.EncodeForTrustedBuild(s, now)
		if err != nil {
			t.Fatal(err)
		}
		r, err := trustinstall.DecodeCompiled(encoded)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	return makeRelease(1), makeRelease(2)
}

type journal struct {
	raw                              []byte
	created, leased, fail, uncertain bool
}

func (j *journal) Load() ([]byte, error) {
	if j.raw == nil {
		return nil, trustprofile.ErrNotInstalled
	}
	return append([]byte(nil), j.raw...), nil
}
func (j *journal) Commit(old, next []byte) error {
	if j.fail || string(old) != string(j.raw) {
		return trustprofile.ErrTrust
	}
	j.raw = append([]byte(nil), next...)
	if j.uncertain {
		return trustprofile.ErrTrust
	}
	return nil
}
func (j *journal) Close() error { j.leased = false; return nil }
func (j *journal) open(create bool) (trustinstall.Journal, error) {
	if j.leased || (create && j.created) || (!create && !j.created) {
		return nil, trustprofile.ErrTrust
	}
	j.created = true
	j.leased = true
	return j, nil
}

type identity struct {
	key       ed25519.PublicKey
	closed    int
	failClose bool
}

func (i *identity) PublicKey() ed25519.PublicKey { return i.key }
func (i *identity) CloseJoined() error {
	i.closed++
	if i.failClose {
		return trustprofile.ErrTrust
	}
	return nil
}

type testNode struct {
	exists, leased, broken bool
	phase                  string
}

func (n *testNode) Check() error {
	if n.broken {
		return trustprofile.ErrTrust
	}
	return nil
}
func (n *testNode) Close() error { n.leased = false; return nil }
func (n *testNode) open(create bool) (node, error) {
	if n.leased || (create && n.exists) || (!create && !n.exists) {
		return nil, trustprofile.ErrTrust
	}
	if create {
		n.exists = true
		n.phase = "idle"
	}
	n.leased = true
	return n, nil
}
func fixture(t *testing.T) (*Owner, *Owner, *journal, *testNode, *identity) {
	t.Helper()
	a, b := releases(t, strings.Repeat("a", 64))
	pub, _, _ := ed25519.GenerateKey(rand.Reader)
	j := &journal{}
	n := &testNode{}
	i := &identity{key: pub}
	makeOwner := func(r trustinstall.Release) *Owner {
		o, err := New(r, j.open, func() (Identity, error) { return i, nil }, strings.Repeat("b", 32), hex.EncodeToString(pub), time.Now)
		if err != nil {
			t.Fatal(err)
		}
		o.node = n.open
		return o
	}
	return makeOwner(a), makeOwner(b), j, n, i
}

func TestInstallUpgradePreservesTerminalNodeAndRejectsRollback(t *testing.T) {
	a, b, j, n, i := fixture(t)
	ctx := context.Background()
	if a.Inspect(ctx) == nil || a.Upgrade(ctx) == nil || j.created {
		t.Fatal("missing store auto-created")
	}
	if a.Initialize(ctx) != nil || a.Inspect(ctx) != nil {
		t.Fatal("first install failed")
	}
	if a.Initialize(ctx) == nil {
		t.Fatal("duplicate initialized")
	}
	before := string(j.raw)
	n.leased = true
	if b.Upgrade(ctx) == nil || string(j.raw) != before {
		t.Fatal("live node bypassed")
	}
	n.leased = false
	n.phase = "registered"
	if b.Upgrade(ctx) != nil || b.Inspect(ctx) != nil || n.phase != "registered" {
		t.Fatal("upgrade reset node")
	}
	before = string(j.raw)
	if a.Upgrade(ctx) == nil || a.Inspect(ctx) == nil || string(j.raw) != before {
		t.Fatal("rollback accepted")
	}
	if b.Initialize(ctx) == nil || n.phase != "registered" {
		t.Fatal("reinstall reset")
	}
	if j.leased || n.leased || i.closed == 0 {
		t.Fatal("leases leaked")
	}
}

func TestPartialInitializationIsNotRepaired(t *testing.T) {
	a, _, j, n, _ := fixture(t)
	j.fail = true
	ctx := context.Background()
	if a.Initialize(ctx) == nil || !n.exists || j.raw != nil {
		t.Fatal("expected partial create")
	}
	j.fail = false
	if a.Initialize(ctx) == nil || a.Inspect(ctx) == nil || a.Upgrade(ctx) == nil || j.raw != nil {
		t.Fatal("partial install repaired")
	}
	if j.leased || n.leased {
		t.Fatal("lease leaked")
	}
}

func TestWrongIdentityAndBadNodeDoNotAdvanceTrust(t *testing.T) {
	a, b, j, n, i := fixture(t)
	ctx := context.Background()
	key := i.key
	i.key = make([]byte, 32)
	if a.Initialize(ctx) == nil || j.created || n.exists {
		t.Fatal("wrong identity initialized")
	}
	i.key = key
	if a.Initialize(ctx) != nil {
		t.Fatal("install")
	}
	before := string(j.raw)
	n.broken = true
	if b.Upgrade(ctx) == nil || string(j.raw) != before {
		t.Fatal("bad node advanced trust")
	}
	n.broken = false
	j.leased = true
	if b.Upgrade(ctx) == nil || n.leased {
		t.Fatal("trust lease bypassed")
	}
	j.leased = false
	i.failClose = true
	if a.Inspect(ctx) == nil {
		t.Fatal("signer join failure ignored")
	}
}

func TestUncertainUpgradeRequiresExplicitReopenAndKeepsFloor(t *testing.T) {
	a, b, j, n, _ := fixture(t)
	ctx := context.Background()
	if a.Initialize(ctx) != nil {
		t.Fatal("install")
	}
	n.phase = "attempted"
	j.uncertain = true
	if b.Upgrade(ctx) == nil {
		t.Fatal("unknown commit reported success")
	}
	j.uncertain = false
	if a.Inspect(ctx) == nil || a.Upgrade(ctx) == nil || b.Inspect(ctx) != nil || n.phase != "attempted" {
		t.Fatal("durable floor or node lost")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	before := string(j.raw)
	if b.Upgrade(cancelled) == nil || string(j.raw) != before {
		t.Fatal("cancelled mutation")
	}
}
