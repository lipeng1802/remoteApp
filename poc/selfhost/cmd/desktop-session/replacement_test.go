package main

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
	"strings"
	"sync"
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/clientowner"
	"remoteapp.local/selfhost-poc/clientruntime"
	"remoteapp.local/selfhost-poc/trustinstall"
	"remoteapp.local/selfhost-poc/trustprofile"
)

type replacementStore struct {
	mu     sync.Mutex
	raw    []byte
	held   bool
	closes int
}
type replacementLease struct {
	store  *replacementStore
	closed bool
}

func (j *replacementLease) Load() ([]byte, error) {
	j.store.mu.Lock()
	defer j.store.mu.Unlock()
	if j.closed {
		return nil, trustprofile.ErrTrust
	}
	if j.store.raw == nil {
		return nil, trustprofile.ErrNotInstalled
	}
	return append([]byte(nil), j.store.raw...), nil
}
func (j *replacementLease) Commit(old, next []byte) error {
	j.store.mu.Lock()
	defer j.store.mu.Unlock()
	if j.closed || string(old) != string(j.store.raw) {
		return trustprofile.ErrTrust
	}
	j.store.raw = append([]byte(nil), next...)
	return nil
}
func (j *replacementLease) Close() error {
	j.store.mu.Lock()
	defer j.store.mu.Unlock()
	if !j.closed {
		j.closed = true
		j.store.held = false
		j.store.closes++
	}
	return nil
}

// Pure backend lifecycle regression; not Windows native or network acceptance.
func replacementBackend(t *testing.T) (*backend, *replacementStore) {
	t.Helper()
	now := time.Now()
	pub, key, _ := ed25519.GenerateKey(rand.Reader)
	cp, ck, _ := ed25519.GenerateKey(rand.Reader)
	issuer, _, _ := ed25519.GenerateKey(rand.Reader)
	ca := &x509.Certificate{SerialNumber: big.NewInt(1), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour)}
	der, e := x509.CreateCertificate(rand.Reader, ca, ca, cp, ck)
	if e != nil {
		t.Fatal(e)
	}
	p, e := trustprofile.Sign(key, trustprofile.Claims{Version: 1, Service: "replace-test", Epoch: 1, Endpoint: "https://replace.example:8444", Certificate: base64.StdEncoding.EncodeToString(der), Issuer: hex.EncodeToString(issuer), IssuedAt: now.Add(-time.Minute).Unix(), ExpiresAt: now.Add(30 * time.Minute).Unix()})
	if e != nil {
		t.Fatal(e)
	}
	sum := sha256.Sum256(pub)
	encoded, e := trustinstall.EncodeForTrustedBuild(trustinstall.Specification{Version: 1, Channel: "replace-" + hex.EncodeToString(sum[:8]), Root: hex.EncodeToString(pub), Service: "replace-test", MinimumEpoch: 1, HelperDigest: strings.Repeat("a", 64), Bundled: p, Test: true}, now)
	if e != nil {
		t.Fatal(e)
	}
	r, e := trustinstall.DecodeCompiled(encoded)
	if e != nil {
		t.Fatal(e)
	}
	store := &replacementStore{}
	f := func(bool) (trustinstall.Journal, error) {
		store.mu.Lock()
		defer store.mu.Unlock()
		if store.held {
			return nil, trustprofile.ErrTrust
		}
		store.held = true
		return &replacementLease{store: store}, nil
	}
	j, e := f(true)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = trustprofile.InitializeInstallation(r.Anchor(), p, j, time.Now); e != nil {
		t.Fatal(e)
	}
	j.Close()
	o, e := clientowner.Open(r, f, time.Now)
	if e != nil {
		t.Fatal(e)
	}
	b := &backend{owner: o, release: r, factory: f}
	t.Cleanup(func() { b.Close(context.Background()) })
	return b, store
}

func faultBackend(t *testing.T, b *backend) *clientowner.Owner {
	t.Helper()
	old := b.owner
	h, e := old.Start(context.Background(), func(context.Context, trustprofile.Profile) error { return errors.New("fault") })
	if e != nil {
		t.Fatal(e)
	}
	<-h.Done()
	b.handle = h
	b.observation = &clientruntime.ProductObservation{}
	if e = b.RetireFault(context.Background()); e != nil {
		t.Fatal(e)
	}
	return old
}

func TestDesktopReplacementIsIdleAndTeardownClosesCurrentOwner(t *testing.T) {
	b, store := replacementBackend(t)
	old := faultBackend(t, b)
	if e := b.ReopenFault(context.Background()); e != nil {
		t.Fatal(e)
	}
	if b.owner == old || b.handle != nil || b.observation != nil || b.SessionState().Joined {
		t.Fatal("stale session reused")
	}
	if _, e := b.Count(context.Background()); e == nil {
		t.Fatal("old product count retained")
	}
	if _, e := old.Start(context.Background(), func(context.Context, trustprofile.Profile) error { return nil }); e == nil {
		t.Fatal("old owner revived")
	}
	if e := b.ReopenFault(context.Background()); e == nil {
		t.Fatal("idle replacement reopened")
	}
	if e := b.Close(context.Background()); e != nil {
		t.Fatal(e)
	}
	store.mu.Lock()
	held := store.held
	closes := store.closes
	store.mu.Unlock()
	if held || closes != 3 {
		t.Fatal("current replacement journal not closed")
	}
	if e := b.ReopenFault(context.Background()); e == nil {
		t.Fatal("final teardown reopened")
	}
}

func TestDesktopReplacementFailurePreservesOldTelemetry(t *testing.T) {
	b, _ := replacementBackend(t)
	old := faultBackend(t, b)
	handle, observation := b.handle, b.observation
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if e := b.ReopenFault(ctx); e == nil || b.owner != old || b.handle != handle || b.observation != observation {
		t.Fatal("failed replacement published")
	}
	if !b.SessionState().Joined || !b.SessionState().Fault {
		t.Fatal("old terminal evidence lost")
	}
}

func TestDesktopConcurrentReplacementHasOnlyOneWinner(t *testing.T) {
	b, _ := replacementBackend(t)
	faultBackend(t, b)
	var wg sync.WaitGroup
	results := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); results <- b.ReopenFault(context.Background()) }()
	}
	wg.Wait()
	close(results)
	successes := 0
	for e := range results {
		if e == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatal("replacement not serialized")
	}
}

func TestDesktopReplacementRacingCloseNeverLeavesLease(t *testing.T) {
	for i := 0; i < 8; i++ {
		b, store := replacementBackend(t)
		faultBackend(t, b)
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); b.ReopenFault(context.Background()) }()
		go func() {
			defer wg.Done()
			if e := b.Close(context.Background()); e != nil {
				t.Error("final close", e)
			}
		}()
		wg.Wait()
		store.mu.Lock()
		held := store.held
		store.mu.Unlock()
		if held || !b.closing {
			t.Fatal("replacement outlived final close")
		}
		if e := b.ReopenFault(context.Background()); e == nil {
			t.Fatal("replacement after final close")
		}
	}
}
