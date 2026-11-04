package programremoval

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/autonomousbinding"
	"remoteapp.local/selfhost-poc/enrollmentstore"
	"remoteapp.local/selfhost-poc/installrelease"
	"remoteapp.local/selfhost-poc/trustinstall"
	"remoteapp.local/selfhost-poc/trustprofile"
)

type boundJournal struct {
	raw                          []byte
	closed, closeFailure, mutate bool
	loads                        int
}

func (j *boundJournal) Load() ([]byte, error) {
	j.loads++
	if j.closed {
		return nil, autonomousbinding.ErrBinding
	}
	raw := append([]byte(nil), j.raw...)
	if j.mutate && j.loads > 1 {
		raw = append(raw, ' ')
	}
	return raw, nil
}
func (j *boundJournal) Commit([]byte, []byte) error { return autonomousbinding.ErrBinding }
func (j *boundJournal) Close() error {
	j.closed = true
	if j.closeFailure {
		return autonomousbinding.ErrBinding
	}
	return nil
}

type boundTestSigner struct {
	key    ed25519.PrivateKey
	closes int
}

func (s *boundTestSigner) PublicKey() ed25519.PublicKey  { return s.key.Public().(ed25519.PublicKey) }
func (s *boundTestSigner) Sign(b []byte) ([]byte, error) { return ed25519.Sign(s.key, b), nil }
func (s *boundTestSigner) CloseJoined() error            { s.closes++; return nil }
func boundRelease(t *testing.T) (trustinstall.Release, autonomousbinding.Record, *boundTestSigner) {
	t.Helper()
	now := time.Now()
	root, key, _ := ed25519.GenerateKey(rand.Reader)
	issuer, _, _ := ed25519.GenerateKey(rand.Reader)
	cp, ck, _ := ed25519.GenerateKey(rand.Reader)
	cert := &x509.Certificate{SerialNumber: big.NewInt(1), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour)}
	der, e := x509.CreateCertificate(rand.Reader, cert, cert, cp, ck)
	if e != nil {
		t.Fatal(e)
	}
	p, e := trustprofile.Sign(key, trustprofile.Claims{Version: 1, Service: "bound-removal", Epoch: 1, Endpoint: "https://removal.example:8444", Certificate: base64.StdEncoding.EncodeToString(der), Issuer: hex.EncodeToString(issuer), IssuedAt: now.Add(-time.Minute).Unix(), ExpiresAt: now.Add(time.Hour).Unix()})
	if e != nil {
		t.Fatal(e)
	}
	spec := trustinstall.Specification{Version: 1, Channel: "bound-removal-" + hex.EncodeToString(root[:6]), Root: hex.EncodeToString(root), Service: "bound-removal", MinimumEpoch: 1, HelperDigest: strings.Repeat("a", 64), Bundled: p, Test: true}
	encoded, e := trustinstall.EncodeForTrustedBuild(spec, now)
	if e != nil {
		t.Fatal(e)
	}
	r, e := trustinstall.DecodeCompiled(encoded)
	if e != nil {
		t.Fatal(e)
	}
	_, device, _ := ed25519.GenerateKey(rand.Reader)
	s := &boundTestSigner{key: device}
	vault := strings.Repeat("b", 32)
	pub := hex.EncodeToString(s.PublicKey())
	enrollment, _ := enrollmentstore.Scope(r.Scope(), vault, pub)
	return r, autonomousbinding.Record{Version: 1, ReleaseScope: r.Scope(), VaultScope: vault, PublicKey: pub, EnrollmentScope: enrollment}, s
}
func TestBoundRemovalIdentityOnlyFromExistingCheckedBinding(t *testing.T) {
	for _, mode := range []string{"valid", "changed", "close_failed", "wrong_signer", "absent"} {
		t.Run(mode, func(t *testing.T) {
			r, record, signer := boundRelease(t)
			raw, _ := json.Marshal(record)
			before := append([]byte(nil), raw...)
			j := &boundJournal{raw: raw, mutate: mode == "changed", closeFailure: mode == "close_failed"}
			opened := false
			scope, _ := autonomousbinding.Scope(r.Scope())
			factory := func(got string, create, test bool) (autonomousbinding.Journal, error) {
				if got != scope || create || !test {
					t.Fatal("identity selector or creation")
				}
				if mode == "absent" {
					return nil, autonomousbinding.ErrBinding
				}
				return j, nil
			}
			s, e := openBoundRemovalSigner(r, factory, func(got autonomousbinding.Record) (BoundSigner, error) {
				opened = true
				if !j.closed || got != record {
					t.Fatal("signer opened before inspection join or selected foreign identity")
				}
				if mode == "wrong_signer" {
					_, signer.key, _ = ed25519.GenerateKey(rand.Reader)
				}
				return signer, nil
			})
			if mode == "valid" {
				if e != nil || s != signer {
					t.Fatal(e)
				}
				s.CloseJoined()
			} else if e == nil {
				t.Fatal("bad binding admitted")
			}
			if (mode == "changed" || mode == "close_failed" || mode == "absent") && opened {
				t.Fatal("failed binding reached signer")
			}
			if mode == "wrong_signer" && signer.closes != 1 {
				t.Fatal("mismatched signer not joined")
			}
			if !bytes.Equal(j.raw, before) {
				t.Fatal("native binding was rewritten")
			}
		})
	}
}
func TestArmedRemovalWaitDoesNotHoldBindingOrAcquirePermissionAndJoinsOnCancel(t *testing.T) {
	r, record, signer := boundRelease(t)
	raw, _ := json.Marshal(record)
	parent, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	root := filepath.Join(parent, "install")
	if _, e = installrelease.PrepareProtectedLayout(root, true); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var productBinding *autonomousbinding.Store
	opens := 0
	records := func(_ string, create, test bool) (autonomousbinding.Journal, error) {
		if create || !test {
			t.Fatal("binding creation")
		}
		opens++
		return &boundJournal{raw: raw}, nil
	}
	trust := func(bool) (trustinstall.Journal, error) {
		t.Fatal("permission/deletion acquired before explicit trigger")
		return nil, trustprofile.ErrTrust
	}
	called := false
	_, e = ArmedAuthenticatedBoundTest(ctx, r, trust, records, root, "127.0.0.1:1", func(got autonomousbinding.Record) (BoundSigner, error) {
		if got != record {
			t.Fatal("identity changed")
		}
		return signer, nil
	}, func(life context.Context) error {
		called = true
		if life.Err() != nil {
			t.Fatal("already expired")
		}
		productBinding, e = autonomousbinding.OpenWithJournal(r.Scope(), true, records)
		if e != nil {
			t.Fatal("arm kept binding locked", e)
		}
		cancel()
		return life.Err()
	})
	if e == nil || !called || signer.closes != 1 || opens != 2 {
		t.Fatal("cancel did not join exact signer", e, called, signer.closes, opens)
	}
	if productBinding.Check() != nil {
		t.Fatal("armed client changed active binding")
	}
	productBinding.Close()
	if _, e = os.Lstat(filepath.Join(root, installrelease.RemovalPlanName)); !os.IsNotExist(e) {
		t.Fatal("cancel wrote a removal plan")
	}
}
