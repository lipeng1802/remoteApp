package clientowner

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
	"sync"
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/trustinstall"
	"remoteapp.local/selfhost-poc/trustprofile"
)

type journal struct {
	mu      sync.Mutex
	raw     []byte
	closed  bool
	fail    bool
	commits int
}

func (j *journal) Load() ([]byte, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.closed {
		return nil, trustprofile.ErrTrust
	}
	if j.raw == nil {
		return nil, trustprofile.ErrNotInstalled
	}
	return append([]byte(nil), j.raw...), nil
}
func (j *journal) Commit(old, next []byte) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.closed || j.fail || string(old) != string(j.raw) {
		return trustprofile.ErrTrust
	}
	j.raw = append([]byte(nil), next...)
	j.commits++
	return nil
}
func (j *journal) Close() error { j.mu.Lock(); defer j.mu.Unlock(); j.closed = true; return nil }

func fixture(t *testing.T, digest string) (trustinstall.Release, trustprofile.Profile) {
	t.Helper()
	now := time.Now()
	pub, key, _ := ed25519.GenerateKey(rand.Reader)
	cp, ck, _ := ed25519.GenerateKey(rand.Reader)
	issuer, _, _ := ed25519.GenerateKey(rand.Reader)
	ca := &x509.Certificate{SerialNumber: big.NewInt(1), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(24 * time.Hour)}
	der, e := x509.CreateCertificate(rand.Reader, ca, ca, cp, ck)
	if e != nil {
		t.Fatal(e)
	}
	c := trustprofile.Claims{Version: 1, Service: "live-test", Epoch: 1, Endpoint: "https://live.example:8444", Certificate: base64.StdEncoding.EncodeToString(der), Issuer: hex.EncodeToString(issuer), IssuedAt: now.Add(-time.Minute).Unix(), ExpiresAt: now.Add(time.Hour).Unix()}
	p, _ := trustprofile.Sign(key, c)
	var random [8]byte
	rand.Read(random[:])
	s := trustinstall.Specification{Version: 1, Channel: "live-" + hex.EncodeToString(random[:]), Root: hex.EncodeToString(pub), Service: c.Service, MinimumEpoch: 1, HelperDigest: digest, Bundled: p, Test: true}
	enc, e := trustinstall.EncodeForTrustedBuild(s, now)
	if e != nil {
		t.Fatal(e)
	}
	r, e := trustinstall.DecodeCompiled(enc)
	if e != nil {
		t.Fatal(e)
	}
	c.Epoch++
	next, _ := trustprofile.Sign(key, c)
	return r, next
}
func setup(t *testing.T) (*Owner, *journal, trustprofile.Profile) {
	t.Helper()
	r, next := fixture(t, strings.Repeat("a", 64))
	j := &journal{}
	if _, e := trustprofile.InitializeInstallation(r.Anchor(), r.Specification().Bundled, j, time.Now); e != nil {
		t.Fatal(e)
	}
	o, e := Open(r, func(create bool) (trustinstall.Journal, error) {
		if create {
			t.Fatal("live owner creates state")
		}
		return j, nil
	}, time.Now)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		o.Close(ctx)
	})
	return o, j, next
}
func mustStart(t *testing.T, o *Owner, run Runner) *Handle {
	t.Helper()
	h, e := o.Start(context.Background(), run)
	if e != nil {
		t.Fatal(e)
	}
	return h
}

func TestUpgradeJoinsBeforeCommitAndNeedsExplicitRestart(t *testing.T) {
	o, j, next := setup(t)
	cancelled, release := make(chan struct{}), make(chan struct{})
	h := mustStart(t, o, func(ctx context.Context, p trustprofile.Profile) error {
		if p.Claims.Epoch != 1 {
			t.Error("old epoch")
		}
		<-ctx.Done()
		close(cancelled)
		<-release
		return nil
	})
	bad := next
	bad.Signature = strings.Repeat("0", 128)
	if o.Upgrade(context.Background(), bad) == nil {
		t.Fatal("invalid profile")
	}
	select {
	case <-cancelled:
		t.Fatal("invalid profile stopped old session")
	default:
	}
	upgrade := make(chan error, 1)
	go func() { upgrade <- o.Upgrade(context.Background(), next) }()
	<-cancelled
	j.mu.Lock()
	commits := j.commits
	j.mu.Unlock()
	if commits != 1 {
		t.Fatal("commit before join")
	}
	select {
	case <-upgrade:
		t.Fatal("upgrade before worker join")
	default:
	}
	close(release)
	if e := <-upgrade; e != nil {
		t.Fatal(e)
	}
	<-h.Done()
	j.mu.Lock()
	commits = j.commits
	j.mu.Unlock()
	if commits != 2 {
		t.Fatal("commit absent")
	}
	h = mustStart(t, o, func(ctx context.Context, p trustprofile.Profile) error {
		if p.Claims.Epoch != 2 {
			t.Error("new epoch absent")
		}
		return nil
	})
	if e := h.Wait(context.Background()); e != nil {
		t.Fatal(e)
	}
}

func TestTimeoutLatchesClosedAndRetainsNativeLeaseUntilJoin(t *testing.T) {
	o, j, next := setup(t)
	release := make(chan struct{})
	started := make(chan struct{})
	mustStart(t, o, func(ctx context.Context, _ trustprofile.Profile) error {
		close(started)
		<-ctx.Done()
		<-release
		return nil
	})
	<-started
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if o.Upgrade(ctx, next) == nil {
		t.Fatal("join timeout accepted")
	}
	if _, e := o.Start(context.Background(), func(context.Context, trustprofile.Profile) error { return nil }); e == nil {
		t.Fatal("timeout reconnect")
	}
	ctx2, cancel2 := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel2()
	if o.Close(ctx2) == nil {
		t.Fatal("close skipped join")
	}
	j.mu.Lock()
	closed, commits := j.closed, j.commits
	j.mu.Unlock()
	if closed || commits != 1 {
		t.Fatal("pending runner released lease or committed")
	}
	close(release)
	if e := o.Close(context.Background()); e != nil {
		t.Fatal(e)
	}
	j.mu.Lock()
	closed = j.closed
	j.mu.Unlock()
	if !closed {
		t.Fatal("lease not released")
	}
}

func TestRunnerFaultAndCommitFailureRejectReuse(t *testing.T) {
	for _, mode := range []string{"runner", "commit"} {
		t.Run(mode, func(t *testing.T) {
			o, j, next := setup(t)
			if mode == "runner" {
				h := mustStart(t, o, func(context.Context, trustprofile.Profile) error { return errors.New("fault") })
				<-h.Done()
			} else {
				j.mu.Lock()
				j.fail = true
				j.mu.Unlock()
				if o.Upgrade(context.Background(), next) == nil {
					t.Fatal("commit fault accepted")
				}
			}
			if _, e := o.Start(context.Background(), func(context.Context, trustprofile.Profile) error { return nil }); e == nil {
				t.Fatal("fault reopened owner")
			}
		})
	}
}

func TestGroupSealsBeforeJoining(t *testing.T) {
	g := NewGroup(context.Background())
	release := make(chan struct{})
	started := make(chan struct{})
	if !g.Go(func(ctx context.Context) { close(started); <-ctx.Done(); <-release }) {
		t.Fatal("start")
	}
	<-started
	done := g.Seal()
	if g.Go(func(context.Context) {}) {
		t.Fatal("new worker during join")
	}
	select {
	case <-done:
		t.Fatal("unjoined")
	default:
	}
	close(release)
	<-done
	if g.Seal() != done {
		t.Fatal("idempotent seal")
	}
}

func TestNativeLiveOwnerRetainsLeaseOnTimeoutAndReopensEpoch(t *testing.T) {
	exe := os.Getenv("REMOTEAPP_JOURNAL_TEST_EXECUTABLE")
	if exe == "" {
		t.Skip("native helper required; not memory acceptance")
	}
	real, e := filepath.EvalSymlinks(exe)
	if e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(real)
	if e != nil {
		t.Fatal(e)
	}
	sum := sha256.Sum256(b)
	r, next := fixture(t, hex.EncodeToString(sum[:]))
	f, e := r.NativeFactory(real)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		if e := trustprofile.DeleteTestDesktopJournal(real, r.Scope()); e != nil {
			t.Error("exact cleanup", e)
		}
	})
	offline, e := trustinstall.NewOfflineOwner(r, f, time.Now)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = offline.Initialize(); e != nil {
		t.Fatal(e)
	}
	o, e := Open(r, f, time.Now)
	if e != nil {
		t.Fatal(e)
	}
	release := make(chan struct{})
	started := make(chan struct{})
	h := mustStart(t, o, func(ctx context.Context, _ trustprofile.Profile) error {
		close(started)
		<-ctx.Done()
		<-release
		return nil
	})
	<-started
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	if o.Upgrade(ctx, next) == nil {
		t.Fatal("timeout accepted")
	}
	if second, e := f(false); e == nil {
		second.Close()
		t.Fatal("native lease released before join")
	}
	close(release)
	<-h.Done()
	if e = o.Close(context.Background()); e != nil {
		t.Fatal(e)
	}
	o, e = Open(r, f, time.Now)
	if e != nil {
		t.Fatal("explicit reopen", e)
	}
	defer o.Close(context.Background())
	if e = o.Upgrade(context.Background(), next); e != nil {
		t.Fatal(e)
	}
	h = mustStart(t, o, func(_ context.Context, p trustprofile.Profile) error {
		if p.Claims.Epoch != 2 {
			return trustprofile.ErrTrust
		}
		return nil
	})
	if e = h.Wait(context.Background()); e != nil {
		t.Fatal(e)
	}
}
