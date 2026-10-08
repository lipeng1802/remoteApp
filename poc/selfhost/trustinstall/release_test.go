package trustinstall

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/trustprofile"
)

func releaseFixture(t *testing.T) (Specification, trustprofile.Profile, ed25519.PrivateKey, time.Time) {
	t.Helper()
	now := time.Now().Truncate(time.Second)
	pub, key, _ := ed25519.GenerateKey(rand.Reader)
	cp, ck, _ := ed25519.GenerateKey(rand.Reader)
	issuer, _, _ := ed25519.GenerateKey(rand.Reader)
	ca := &x509.Certificate{SerialNumber: big.NewInt(1), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(24 * time.Hour)}
	der, e := x509.CreateCertificate(rand.Reader, ca, ca, cp, ck)
	if e != nil {
		t.Fatal(e)
	}
	c := trustprofile.Claims{Version: 1, Service: "release-test", Epoch: 1, Endpoint: "https://release.example:8444", Certificate: base64.StdEncoding.EncodeToString(der), Issuer: hex.EncodeToString(issuer), IssuedAt: now.Add(-time.Minute).Unix(), ExpiresAt: now.Add(time.Hour).Unix()}
	p, e := trustprofile.Sign(key, c)
	if e != nil {
		t.Fatal(e)
	}
	h := sha256.Sum256([]byte("trusted helper"))
	s := Specification{Version: 1, Channel: "release-test", Root: hex.EncodeToString(pub), Service: c.Service, MinimumEpoch: 1, HelperDigest: hex.EncodeToString(h[:]), Bundled: p, Test: true}
	c.Epoch++
	next, _ := trustprofile.Sign(key, c)
	return s, next, key, now
}

type memoryJournal struct {
	raw        []byte
	bad        bool
	failCommit bool
	closed     int
}

func (j *memoryJournal) Load() ([]byte, error) {
	if j.bad {
		return nil, trustprofile.ErrTrust
	}
	if j.raw == nil {
		return nil, trustprofile.ErrNotInstalled
	}
	return append([]byte(nil), j.raw...), nil
}
func (j *memoryJournal) Commit(old, next []byte) error {
	if j.failCommit || string(old) != string(j.raw) {
		return trustprofile.ErrTrust
	}
	j.raw = append([]byte(nil), next...)
	return nil
}
func (j *memoryJournal) Close() error { j.closed++; return nil }
func factoryFor(j *memoryJournal, creates *int) Factory {
	return func(create bool) (Journal, error) {
		if create {
			*creates++
			if j.raw != nil || j.bad {
				return nil, trustprofile.ErrTrust
			}
		}
		return j, nil
	}
}
func ownerFor(t *testing.T, s Specification, j *memoryJournal, creates *int, now *time.Time) *OfflineOwner {
	t.Helper()
	enc, e := EncodeForTrustedBuild(s, *now)
	if e != nil {
		t.Fatal(e)
	}
	r, e := DecodeCompiled(enc)
	if e != nil {
		t.Fatal(e)
	}
	o, e := NewOfflineOwner(r, factoryFor(j, creates), func() time.Time { return *now })
	if e != nil {
		t.Fatal(e)
	}
	return o
}
func TestCompiledReleaseStrictValidationAndStableChannel(t *testing.T) {
	s, next, _, now := releaseFixture(t)
	encoded, e := EncodeForTrustedBuild(s, now)
	if e != nil {
		t.Fatal(e)
	}
	a, e := DecodeCompiled(encoded)
	if e != nil {
		t.Fatal(e)
	}
	s.MinimumEpoch = 2
	s.Bundled = next
	s.HelperDigest = strings.Repeat("1", 64)
	updated, e := EncodeForTrustedBuild(s, now)
	if e != nil {
		t.Fatal(e)
	}
	b, _ := DecodeCompiled(updated)
	if a.Scope() != b.Scope() {
		t.Fatal("upgrade silently changes storage namespace")
	}
	for _, input := range []string{"", encoded + "!", base64.StdEncoding.EncodeToString([]byte(`{"version":1,"VERSION":1}`)), base64.StdEncoding.EncodeToString([]byte(`{} {}`))} {
		if _, e := DecodeCompiled(input); e == nil {
			t.Fatal("ambiguous compiled release")
		}
	}
	for _, mutate := range []func(*Specification){func(s *Specification) { s.Channel = "../bad" }, func(s *Specification) { s.Root = strings.Repeat("0", 64) }, func(s *Specification) { s.MinimumEpoch = 0 }, func(s *Specification) { s.MinimumEpoch = 3 }, func(s *Specification) { s.Service = "other" }, func(s *Specification) { s.HelperDigest = "bad" }} {
		bad := s
		mutate(&bad)
		if _, e := EncodeForTrustedBuild(bad, now); e == nil {
			t.Fatal("bad trusted-build specification")
		}
	}
}
func TestOfflineInstallUpgradeRaisedFloorAndExplicitRecovery(t *testing.T) {
	s, next, key, now := releaseFixture(t)
	j := &memoryJournal{}
	creates := 0
	v1 := ownerFor(t, s, j, &creates, &now)
	if _, e := v1.Initialize(); e != nil {
		t.Fatal(e)
	}
	if _, e := v1.Initialize(); e == nil {
		t.Fatal("existing install overwritten")
	}
	s.MinimumEpoch = 2
	s.Bundled = next
	v2 := ownerFor(t, s, j, &creates, &now)
	if _, e := v2.Inspect(); e == nil {
		t.Fatal("trusted raised floor falls back")
	}
	wrong := next
	wrong.Claims.Service = "other"
	wrong, _ = trustprofile.Sign(key, wrong.Claims)
	prior := append([]byte(nil), j.raw...)
	if _, e := v2.Upgrade(context.Background(), wrong); e == nil || string(j.raw) != string(prior) {
		t.Fatal("invalid candidate mutated journal")
	}
	if _, e := v1.Inspect(); e != nil {
		t.Fatal("old committed record unexpectedly lost")
	}
	j.failCommit = true
	if _, e := v2.Upgrade(context.Background(), next); e == nil {
		t.Fatal("commit failure ignored")
	}
	j.failCommit = false
	if _, e := v1.Inspect(); e != nil {
		t.Fatal("explicit inspection old commit")
	}
	if _, e := v2.Upgrade(context.Background(), next); e != nil {
		t.Fatal(e)
	}
	if p, e := v1.Inspect(); e != nil || p != next {
		t.Fatal("old software lost persistent floor")
	}
	if _, e := v1.Upgrade(context.Background(), v1.release.spec.Bundled); e == nil {
		t.Fatal("old binary rolls configuration back")
	}
	mutated := next
	mutated.Claims.Endpoint = "https://other.example:8444"
	mutated, _ = trustprofile.Sign(key, mutated.Claims)
	if _, e := v2.Upgrade(context.Background(), mutated); e == nil {
		t.Fatal("same epoch replacement")
	}
	before := creates
	j.bad = true
	if _, e := v2.Inspect(); e == nil {
		t.Fatal("corrupt open")
	}
	if _, e := v2.Upgrade(context.Background(), next); e == nil || creates != before {
		t.Fatal("open failure turned into initialization")
	}
}
func TestExpiredBundlePreflightAndWrongRootPreserveState(t *testing.T) {
	s, _, _, now := releaseFixture(t)
	j := &memoryJournal{}
	creates := 0
	o := ownerFor(t, s, j, &creates, &now)
	now = time.Unix(s.Bundled.Claims.ExpiresAt, 0)
	if _, e := o.Initialize(); e == nil || creates != 0 {
		t.Fatal("expired install created pending native state")
	}
	now = now.Add(-time.Minute)
	if _, e := o.Initialize(); e != nil {
		t.Fatal(e)
	}
	other, _, _, at := releaseFixture(t)
	other.Channel = s.Channel
	wrong := ownerFor(t, other, j, &creates, &at)
	if wrong.release.Scope() != o.release.Scope() {
		t.Fatal("wrong root selected fresh storage")
	}
	if _, e := wrong.Inspect(); e == nil {
		t.Fatal("root replacement on reopen")
	}
	if _, e := wrong.Initialize(); e == nil {
		t.Fatal("wrong root reinitializes")
	}
}
func TestHelperDigestPinBeforeNativeAccess(t *testing.T) {
	s, _, _, now := releaseFixture(t)
	encoded, _ := EncodeForTrustedBuild(s, now)
	r, _ := DecodeCompiled(encoded)
	d := t.TempDir()
	d, e := filepath.EvalSymlinks(d)
	if e != nil { t.Fatal(e) }
	file := filepath.Join(d, "helper")
	if os.WriteFile(file, []byte("trusted helper"), 0700) != nil {
		t.Fatal("fixture")
	}
	f, e := r.NativeFactory(file)
	if e != nil {
		t.Fatal(e)
	}
	if os.WriteFile(file, []byte("changed helper"), 0700) != nil {
		t.Fatal("mutate")
	}
	if _, e := f(false); !errors.Is(e, trustprofile.ErrTrust) {
		t.Fatal("changed helper launched")
	}
	alias := filepath.Join(d, "alias")
	if os.Symlink(file, alias) == nil {
		if _, e := r.NativeFactory(alias); e == nil {
			t.Fatal("symlink helper")
		}
	}
}
