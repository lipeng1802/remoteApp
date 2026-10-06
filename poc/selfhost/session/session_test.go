package session

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"io"
	"net"
	"net/netip"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/invite"
)

type fixture struct {
	t                    *testing.T
	root                 string
	s                    *invite.Store
	now                  atomic.Int64
	owner, client, other ed25519.PrivateKey
	server, probe        Config
	request              string
}

func setup(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{t: t, root: filepath.Join(t.TempDir(), "private")}
	f.now.Store(1800000000)
	f.s = f.open()
	_, f.owner, _ = ed25519.GenerateKey(rand.Reader)
	_, f.client, _ = ed25519.GenerateKey(rand.Reader)
	_, f.other, _ = ed25519.GenerateKey(rand.Reader)
	target := f.execute(f.owner, invite.Body{Action: "register"})
	f.execute(f.client, invite.Body{Action: "register"})
	inv := f.execute(f.owner, invite.Body{Action: "invite"})
	req := f.execute(f.client, invite.Body{Action: "claim", TargetCode: target.DeviceCode, Code: inv.Code})
	f.request = req.RequestID
	approved := f.execute(f.owner, invite.Body{Action: "approve", RequestID: req.RequestID, Allow: true})
	f.server = Config{Issuer: f.s.PublicKey(), Key: f.owner, Grant: *approved.Grant, Now: f.clock, Binding: Binding{
		Target: f.owner.Public().(ed25519.PublicKey), Controller: f.client.Public().(ed25519.PublicKey),
		TargetIP: netip.MustParseAddr("127.0.0.1"), ControllerIP: netip.MustParseAddr("127.0.0.1"),
	}}
	f.probe = f.server
	f.probe.Key = f.client
	t.Cleanup(func() { f.s.Close() })
	return f
}
func (f *fixture) clock() time.Time { return time.Unix(f.now.Load(), 0) }
func (f *fixture) open() *invite.Store {
	s, err := invite.Open(f.root, f.clock)
	if err != nil {
		f.t.Fatal(err)
	}
	return s
}
func (f *fixture) execute(key ed25519.PrivateKey, b invite.Body) invite.Result {
	f.t.Helper()
	n := make([]byte, 16)
	rand.Read(n)
	b.Nonce = hex.EncodeToString(n)
	b.IssuedAt = f.now.Load()
	r, err := f.s.Execute(invite.Sign(key, b))
	if err != nil {
		f.t.Fatal(err)
	}
	return r
}
func (f *fixture) check(ctx context.Context, g invite.Grant, a, b string) bool {
	return ctx.Err() == nil && f.s.Authorized(g, a, b)
}
func sockets(t *testing.T) (net.Conn, net.Conn) {
	t.Helper()
	l, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	c, err := net.DialTimeout("tcp4", l.Addr().String(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	s, err := l.Accept()
	if err != nil {
		c.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close(); c.Close() })
	return s, c
}

type running struct {
	cancel        context.CancelFunc
	server, probe chan error
	ticks         chan struct{}
}

func (f *fixture) run(check Check) *running {
	s, c := sockets(f.t)
	ctx, cancel := context.WithCancel(context.Background())
	r := &running{cancel: cancel, server: make(chan error, 1), probe: make(chan error, 1), ticks: make(chan struct{}, 32)}
	go func() { r.server <- Serve(ctx, s, f.server, check) }()
	go func() {
		r.probe <- Probe(ctx, c, f.probe, func() {
			select {
			case r.ticks <- struct{}{}:
			default:
			}
		})
	}()
	f.t.Cleanup(cancel)
	return r
}
func tick(t *testing.T, r *running) {
	t.Helper()
	select {
	case <-r.ticks:
	case err := <-r.probe:
		t.Fatalf("probe stopped before payload: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("payload timeout")
	}
}
func ended(t *testing.T, r *running) {
	t.Helper()
	for _, ch := range []chan error{r.server, r.probe} {
		select {
		case <-ch:
		case <-time.After(2 * time.Second):
			t.Fatal("connection did not stop")
		}
	}
}
func denied(t *testing.T, r *running) {
	t.Helper()
	ended(t, r)
	if len(r.ticks) != 0 {
		t.Fatal("unauthorized payload emitted")
	}
}
func TestApprovedMutualProofAndCancellation(t *testing.T) {
	f := setup(t)
	r := f.run(f.check)
	tick(t, r)
	tick(t, r)
	r.cancel()
	ended(t, r)
}
func TestRevocationClosesActiveAndRestartRejectsOldGrant(t *testing.T) {
	f := setup(t)
	r := f.run(f.check)
	tick(t, r)
	f.execute(f.owner, invite.Body{Action: "revoke", GrantID: f.server.Grant.Claims.ID})
	ended(t, r)
	if err := f.s.Close(); err != nil {
		t.Fatal(err)
	}
	f.s = f.open()
	denied(t, f.run(f.check))
}
func TestExpiryClosesIdleConnection(t *testing.T) {
	f := setup(t)
	s, c := sockets(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- Serve(ctx, s, f.server, f.check) }()
	n, _ := nonce()
	if write(c, hello{n}) != nil {
		t.Fatal("hello")
	}
	var w welcome
	if read(c, &w) != nil {
		t.Fatal("welcome")
	}
	if write(c, proof{signed(f.client, "controller", w.Transcript)}) != nil {
		t.Fatal("proof")
	}
	// One response ensures the handshake has completed; no subsequent requests.
	c.Write([]byte{1})
	b := make([]byte, len(Payload))
	if _, err := io.ReadFull(c, b); err != nil || string(b) != Payload {
		t.Fatal(err)
	}
	f.now.Add(301)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("idle stream survived expiry")
	}
}
func TestAuthorityUnavailableClosesActiveStream(t *testing.T) {
	f := setup(t)
	var down atomic.Bool
	check := func(ctx context.Context, g invite.Grant, a, b string) bool {
		if down.Load() {
			<-ctx.Done()
			return false
		}
		return f.check(ctx, g, a, b)
	}
	r := f.run(check)
	tick(t, r)
	down.Store(true)
	ended(t, r)
}
func TestOfflineValidGrantCannotBypassOnlineDenial(t *testing.T) {
	f := setup(t)
	denied(t, f.run(func(context.Context, invite.Grant, string, string) bool { return false }))
}
func TestClosedBackendFailsClosed(t *testing.T) {
	f := setup(t)
	r := f.run(f.check)
	tick(t, r)
	f.s.Close()
	ended(t, r)
}
func TestUntrustedIssuerOrGrantMutationRejected(t *testing.T) {
	for _, kind := range []string{"issuer", "permission", "controller", "expiry"} {
		t.Run(kind, func(t *testing.T) {
			f := setup(t)
			switch kind {
			case "issuer":
				f.server.Issuer = f.other.Public().(ed25519.PublicKey)
			case "permission":
				f.server.Grant.Claims.Permission = "input"
			case "controller":
				f.server.Grant.Claims.Controller = Identity(f.other.Public().(ed25519.PublicKey))
			case "expiry":
				f.now.Add(301)
			}
			denied(t, f.run(f.check))
		})
	}
}
func TestWrongLocalKeyAndActualNetworkBindingRejected(t *testing.T) {
	for _, kind := range []string{"key", "source", "target"} {
		t.Run(kind, func(t *testing.T) {
			f := setup(t)
			switch kind {
			case "key":
				f.probe.Key = f.other
			case "source":
				f.server.Binding.ControllerIP = netip.MustParseAddr("127.0.0.2")
			case "target":
				f.probe.Binding.TargetIP = netip.MustParseAddr("127.0.0.2")
			}
			denied(t, f.run(f.check))
		})
	}
}
func TestPossessionProofRejectsThirdDeviceAndWrongDomain(t *testing.T) {
	for _, kind := range []string{"third", "domain"} {
		t.Run(kind, func(t *testing.T) {
			f := setup(t)
			s, c := sockets(t)
			done := make(chan error, 1)
			go func() { done <- Serve(context.Background(), s, f.server, f.check) }()
			n, _ := nonce()
			write(c, hello{n})
			var w welcome
			if read(c, &w) != nil {
				t.Fatal("welcome")
			}
			key, role := f.other, "controller"
			if kind == "domain" {
				key, role = f.client, "target"
			}
			write(c, proof{signed(key, role, w.Transcript)})
			select {
			case err := <-done:
				if err != ErrDenied {
					t.Fatal("proof accepted")
				}
			case <-time.After(time.Second):
				t.Fatal("proof timeout")
			}
			var b [1]byte
			if n, _ := c.Read(b[:]); n != 0 {
				t.Fatal("payload leaked")
			}
		})
	}
}
func TestCapturedProofCannotReplayOnNewConnection(t *testing.T) {
	f := setup(t)
	var captured proof
	for i := 0; i < 2; i++ {
		s, c := sockets(t)
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() { done <- Serve(ctx, s, f.server, f.check) }()
		n, _ := nonce()
		write(c, hello{n})
		var w welcome
		if read(c, &w) != nil {
			t.Fatal("welcome")
		}
		if i == 0 {
			captured = proof{signed(f.client, "controller", w.Transcript)}
		}
		write(c, captured)
		if i == 0 {
			cancel()
		} else {
			select {
			case err := <-done:
				if err != ErrDenied {
					t.Fatal("replay accepted")
				}
			case <-time.After(time.Second):
				t.Fatal("replay timeout")
			}
			cancel()
			continue
		}
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("cancel timeout")
		}
	}
}
func TestFrameBoundsAndStrictJSON(t *testing.T) {
	for _, raw := range [][]byte{{0, 0}, {8, 1}, append([]byte{0, 13}, []byte(`{"unknown":1}`)...), append([]byte{0, 5}, []byte(`{} {}`)...)} {
		s, c := sockets(t)
		done := make(chan error, 1)
		go func() { var h hello; done <- read(s, &h) }()
		c.Write(raw)
		select {
		case err := <-done:
			if err != ErrDenied {
				t.Fatal("invalid frame accepted")
			}
		case <-time.After(time.Second):
			t.Fatal("frame check blocked")
		}
	}
}

func TestPendingOrDeniedRequestDoesNotRestoreOldAuthorization(t *testing.T) {
	for _, reject := range []bool{false, true} {
		f := setup(t)
		f.execute(f.owner, invite.Body{Action: "revoke", GrantID: f.server.Grant.Claims.ID})
		target := f.execute(f.owner, invite.Body{Action: "register"})
		inv := f.execute(f.owner, invite.Body{Action: "invite"})
		req := f.execute(f.client, invite.Body{Action: "claim", TargetCode: target.DeviceCode, Code: inv.Code})
		if reject {
			f.execute(f.owner, invite.Body{Action: "approve", RequestID: req.RequestID, Allow: false})
		}
		denied(t, f.run(f.check))
	}
}

func TestProbeRejectsForgedTargetAndTranscript(t *testing.T) {
	for _, kind := range []string{"target", "nonce", "grant", "address"} {
		t.Run(kind, func(t *testing.T) {
			f := setup(t)
			s, c := sockets(t)
			done := make(chan error, 1)
			go func() { done <- Probe(context.Background(), c, f.probe, nil) }()
			var h hello
			if read(s, &h) != nil {
				t.Fatal("hello")
			}
			n, _ := nonce()
			tr := transcript{1, f.probe.Grant, n, h.Nonce, f.probe.Binding.TargetIP.String(), f.probe.Binding.ControllerIP.String()}
			key := f.owner
			switch kind {
			case "target":
				key = f.other
			case "nonce":
				tr.ClientNonce = n
			case "grant":
				tr.Grant.Claims.ID = n[:32]
			case "address":
				tr.TargetIP = "127.0.0.2"
			}
			write(s, welcome{tr, signed(key, "target", tr)})
			select {
			case err := <-done:
				if err != ErrDenied {
					t.Fatal("forged welcome accepted")
				}
			case <-time.After(time.Second):
				t.Fatal("welcome timeout")
			}
			var p proof
			if read(s, &p) == nil {
				t.Fatal("proof sent to unverified target")
			}
		})
	}
}

func TestInitialAuthorityTimeoutAndMissingCheckDeny(t *testing.T) {
	for _, missing := range []bool{false, true} {
		f := setup(t)
		var check Check
		if !missing {
			check = func(ctx context.Context, _ invite.Grant, _, _ string) bool { <-ctx.Done(); return true }
		}
		denied(t, f.run(check))
	}
}

func TestNewExplicitGrantRequiredAfterRevocation(t *testing.T) {
	f := setup(t)
	old := f.probe.Grant
	f.execute(f.owner, invite.Body{Action: "revoke", GrantID: old.Claims.ID})
	target := f.execute(f.owner, invite.Body{Action: "register"})
	inv := f.execute(f.owner, invite.Body{Action: "invite"})
	req := f.execute(f.client, invite.Body{Action: "claim", TargetCode: target.DeviceCode, Code: inv.Code})
	g := f.execute(f.owner, invite.Body{Action: "approve", RequestID: req.RequestID, Allow: true})
	f.server.Grant = *g.Grant
	// Same application keys and addresses do not revive the previous grant.
	denied(t, f.run(f.check))
	f.probe.Grant = *g.Grant
	r := f.run(f.check)
	tick(t, r)
	r.cancel()
	ended(t, r)
}

func TestClockRollbackClosesActiveStream(t *testing.T) {
	f := setup(t)
	r := f.run(f.check)
	tick(t, r)
	f.now.Add(-60)
	ended(t, r)
}

func TestNegativeProbeRequiresServerProofRejection(t *testing.T) {
	f := setup(t)
	s, c := sockets(t)
	done := make(chan error, 1)
	go func() { done <- Serve(context.Background(), s, f.server, f.check) }()
	if err := RejectProof(context.Background(), c, f.probe); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != ErrDenied {
			t.Fatal("server accepted third key")
		}
	case <-time.After(time.Second):
		t.Fatal("rejection timeout")
	}
}
