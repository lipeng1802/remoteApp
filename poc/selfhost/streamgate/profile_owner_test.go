package streamgate

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/clientowner"
	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/session"
	"remoteapp.local/selfhost-poc/trustinstall"
	"remoteapp.local/selfhost-poc/trustprofile"
)

// Only installation persistence is simulated in this integration test. HTTPS
// calibration, signed autonomous watches, mutual stream proof and opaque tunnel
// workers are real implementations. No Headscale/WhoIs/product TLS claim.
type liveJournal struct {
	mu      sync.Mutex
	raw     []byte
	commits int
}

func (j *liveJournal) Load() ([]byte, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.raw == nil {
		return nil, trustprofile.ErrNotInstalled
	}
	return append([]byte(nil), j.raw...), nil
}
func (j *liveJournal) Commit(old, next []byte) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if string(old) != string(j.raw) {
		return trustprofile.ErrTrust
	}
	j.raw = append([]byte(nil), next...)
	j.commits++
	return nil
}
func (j *liveJournal) Close() error { return nil }

func TestInstalledOwnerJoinsRealHTTPSWatchAndOpaqueTunnelBeforeUpgrade(t *testing.T) {
	stopped, stop := context.WithCancel(context.Background())
	stop()
	if e := ProfileSession(stopped, trustinstall.Release{}, trustprofile.Profile{}, nil, nil, nil, invite.Grant{}, invite.NetworkBinding{}, false, func(context.Context, *Gate) error {
		t.Error("cancelled startup ran a stream worker")
		return nil
	}); e != nil {
		t.Fatal("cancelled startup was not a joined normal shutdown", e)
	}
	f := portableProfileSetup(t)
	pub, key, _ := ed25519.GenerateKey(rand.Reader)
	now := time.Now()
	c := trustprofile.Claims{Version: 1, Service: "live-integration", Epoch: 1, Endpoint: f.endpoint, Certificate: base64.StdEncoding.EncodeToString(f.certificate), Issuer: hex.EncodeToString(f.issuer), IssuedAt: now.Add(-time.Minute).Unix(), ExpiresAt: now.Add(time.Hour).Unix()}
	p, _ := trustprofile.Sign(key, c)
	spec := trustinstall.Specification{Version: 1, Channel: "live-integration", Root: hex.EncodeToString(pub), Service: c.Service, MinimumEpoch: 1, HelperDigest: strings.Repeat("a", 64), Bundled: p, Test: true}
	encoded, e := trustinstall.EncodeForTrustedBuild(spec, now)
	if e != nil {
		t.Fatal(e)
	}
	r, e := trustinstall.DecodeCompiled(encoded)
	if e != nil {
		t.Fatal(e)
	}
	j := &liveJournal{}
	if _, e = trustprofile.InitializeInstallation(r.Anchor(), p, j, time.Now); e != nil {
		t.Fatal(e)
	}
	o, e := clientowner.Open(r, func(bool) (trustinstall.Journal, error) { return j, nil }, time.Now)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		o.Close(ctx)
	})
	c.Epoch = 2
	next, _ := trustprofile.Sign(key, c)
	targetSigner, _ := invite.NewMemorySigner(f.target)
	state := f.networkState()
	target, e := NewWithSigner(f.issuer, f.target.Public().(ed25519.PublicKey), f.signer.PublicKey(), targetSigner, state, true, time.Now)
	if e != nil {
		t.Fatal(e)
	}
	a, b := net.Pipe()
	targetAddr := &net.TCPAddr{IP: net.ParseIP(f.binding.TargetIP), Port: 47476}
	controllerAddr := &net.TCPAddr{IP: net.ParseIP(f.binding.ControllerIP), Port: 50000}
	serverConn := overlayConn{a, targetAddr, controllerAddr}
	clientConn := overlayConn{b, controllerAddr, targetAddr}
	upstream, echo := net.Pipe()
	local, app := net.Pipe()
	t.Cleanup(func() {
		a.Close()
		b.Close()
		upstream.Close()
		echo.Close()
		local.Close()
		app.Close()
	})
	peerDone, echoDone := make(chan error, 1), make(chan struct{})
	go func() { defer close(echoDone); io.Copy(echo, echo) }()
	go func() {
		peerDone <- session.Tunnel(context.Background(), serverConn, target.cfg, target.check, true, func(context.Context) (net.Conn, error) { return upstream, nil })
	}()
	joined, allowReturn := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	t.Cleanup(func() { releaseOnce.Do(func() { close(allowReturn) }) })
	h, e := o.Start(context.Background(), func(ctx context.Context, p trustprofile.Profile) error {
		return ProfileSession(ctx, r, p, f.signer, f.target.Public().(ed25519.PublicKey), f.signer.PublicKey(), f.grant, f.binding, false, func(ctx context.Context, g *Gate) error {
			session.Tunnel(ctx, clientConn, g.cfg, g.check, false, func(context.Context) (net.Conn, error) { return local, nil })
			close(joined)
			<-allowReturn // prove actual runner join, not just cancellation
			return nil
		})
	})
	if e != nil {
		t.Fatal(e)
	}
	app.SetDeadline(time.Now().Add(4 * time.Second))
	written := make(chan error, 1)
	go func() { _, e := app.Write([]byte("PING")); written <- e }()
	var response [4]byte
	if _, e = io.ReadFull(app, response[:]); e != nil || string(response[:]) != "PING" {
		t.Fatal("real tunnel payload", e)
	}
	if e = <-written; e != nil {
		t.Fatal(e)
	}
	f.stall.Store(true)
	before := f.queries.Load()
	deadline := time.Now().Add(2 * time.Second)
	for f.queries.Load() <= before && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if f.queries.Load() <= before {
		t.Fatal("no in-flight real HTTPS watch")
	}
	upgraded := make(chan error, 1)
	go func() { upgraded <- o.Upgrade(context.Background(), next) }()
	select {
	case <-joined:
	case <-time.After(3 * time.Second):
		t.Fatal("tunnel not closed")
	}
	j.mu.Lock()
	commits := j.commits
	j.mu.Unlock()
	if commits != 1 {
		t.Fatal("commit before runner/watch join")
	}
	select {
	case <-upgraded:
		t.Fatal("published before worker join")
	default:
	}
	releaseOnce.Do(func() { close(allowReturn) })
	select {
	case e = <-upgraded:
		if e != nil {
			t.Fatal(e)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("upgrade did not join watch")
	}
	if e = h.Wait(context.Background()); e != nil {
		t.Fatal(e)
	}
	queries := f.queries.Load()
	time.Sleep(600 * time.Millisecond)
	if f.queries.Load() != queries {
		t.Fatal("post-upgrade queries from old watch")
	}
	if _, e = app.Write([]byte("PING")); e == nil {
		t.Fatal("old opaque tunnel survived")
	}
	select {
	case <-peerDone:
	case <-time.After(2 * time.Second):
		t.Fatal("peer stream not joined")
	}
	echo.Close()
	select {
	case <-echoDone:
	case <-time.After(time.Second):
		t.Fatal("peer echo worker not joined")
	}
	if e = o.Upgrade(context.Background(), p); e == nil {
		t.Fatal("downgrade")
	}
	newHandle, e := o.Start(context.Background(), func(_ context.Context, p trustprofile.Profile) error {
		if p.Claims.Epoch != 2 {
			return trustprofile.ErrTrust
		}
		return nil
	})
	if e != nil {
		t.Fatal("explicit restart", e)
	}
	if e = newHandle.Wait(context.Background()); e != nil {
		t.Fatal(e)
	}
}
