package localmaintenance

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/privatepipe"
	"remoteapp.local/selfhost-poc/trustinstall"
	"remoteapp.local/selfhost-poc/trustprofile"
)

type memorySigner struct{ key ed25519.PrivateKey }

func (s memorySigner) PublicKey() ed25519.PublicKey  { return s.key.Public().(ed25519.PublicKey) }
func (s memorySigner) Sign(m []byte) ([]byte, error) { return ed25519.Sign(s.key, m), nil }

type ownerStub struct{ maint, closed atomic.Int32 }

func (o *ownerStub) MaintainDurableState(c context.Context, _ trustinstall.Release, action func(context.Context, trustprofile.MaintenanceState) error) error {
	o.maint.Add(1)
	return action(c, trustprofile.MaintenanceState{State: "pending", Epoch: 2, Transaction: strings.Repeat("a", 32)})
}
func (o *ownerStub) Close(context.Context) error { o.closed.Add(1); return nil }

func releaseFixture(t *testing.T, test bool) trustinstall.Release {
	t.Helper()
	pub, key, _ := ed25519.GenerateKey(rand.Reader)
	cp, ck, _ := ed25519.GenerateKey(rand.Reader)
	issuer, _, _ := ed25519.GenerateKey(rand.Reader)
	now := time.Now()
	ca := &x509.Certificate{SerialNumber: big.NewInt(1), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour)}
	der, e := x509.CreateCertificate(rand.Reader, ca, ca, cp, ck)
	if e != nil {
		t.Fatal(e)
	}
	c := trustprofile.Claims{Version: 1, Service: "local-maint-test", Epoch: 1, Endpoint: "https://test.example:8444", Certificate: base64.StdEncoding.EncodeToString(der), Issuer: hex.EncodeToString(issuer), IssuedAt: now.Add(-time.Minute).Unix(), ExpiresAt: now.Add(time.Hour).Unix()}
	p, e := trustprofile.Sign(key, c)
	if e != nil {
		t.Fatal(e)
	}
	s := trustinstall.Specification{Version: 1, Channel: "local-maint-test", Root: hex.EncodeToString(pub), Service: c.Service, MinimumEpoch: 1, HelperDigest: strings.Repeat("a", 64), Bundled: p, Test: test}
	encoded, e := trustinstall.EncodeForTrustedBuild(s, now)
	if e != nil {
		t.Fatal(e)
	}
	r, e := trustinstall.DecodeCompiled(encoded)
	if e != nil {
		t.Fatal(e)
	}
	return r
}

func testEndpoint(t *testing.T) (trustinstall.Release, memorySigner, *ownerStub, *Server, <-chan error) {
	t.Helper()
	r := releaseFixture(t, true)
	_, key, _ := ed25519.GenerateKey(rand.Reader)
	signer := memorySigner{key}
	o := &ownerStub{}
	l, e := net.Listen("tcp4", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	s, e := NewTest(r, o, signer, l)
	if e != nil {
		l.Close()
		t.Fatal(e)
	}
	done := make(chan error, 1)
	go func() { _, e := s.Serve(context.Background()); done <- e; close(done) }()
	t.Cleanup(func() {
		s.Close()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("server did not join")
		}
	})
	return r, signer, o, s, done
}

func challengeFor(t *testing.T, r trustinstall.Release, s *Server) (net.Conn, *privatepipe.Reader, frame) {
	t.Helper()
	c, e := net.Dial("tcp", s.listener.Addr().String())
	if e != nil {
		t.Fatal(e)
	}
	c.SetDeadline(time.Now().Add(3 * time.Second))
	reader := privatepipe.NewBoundedReader(c, 4096)
	hello := frame{Op: "hello", Version: 1, Scope: r.Scope(), Release: releaseDigest(r), ClientNonce: strings.Repeat("b", 64)}
	if json.NewEncoder(c).Encode(hello) != nil {
		t.Fatal("hello")
	}
	var challenge frame
	if reader.Read(&challenge) != nil {
		t.Fatal("challenge")
	}
	return c, reader, challenge
}

func TestIndependentMaintenanceMutualProofAndOneShot(t *testing.T) {
	r, signer, o, s, done := testEndpoint(t)
	state, e := RequestTest(context.Background(), r, s.listener.Addr().String(), signer.PublicKey(), signer)
	if e != nil || state.State != "pending" || state.Epoch != 2 || state.Transaction != strings.Repeat("a", 32) {
		t.Fatal(state, e)
	}
	if e = <-done; e != nil || o.maint.Load() != 1 || o.closed.Load() != 1 {
		t.Fatal("owner sequence", e)
	}
	if _, e = RequestTest(context.Background(), r, s.listener.Addr().String(), signer.PublicKey(), signer); e == nil {
		t.Fatal("endpoint rearmed")
	}
}

func TestIndependentMaintenanceBindsExactCompiledRelease(t *testing.T) {
	r, signer, o, s, _ := testEndpoint(t)
	other := releaseFixture(t, true)
	if other.Scope() != r.Scope() || releaseDigest(other) == releaseDigest(r) {
		t.Fatal("fixture must share channel but differ in signed release")
	}
	if _, e := RequestTest(context.Background(), other, s.listener.Addr().String(), signer.PublicKey(), signer); e == nil {
		t.Fatal("same-channel foreign release authorized")
	}
	if o.maint.Load() != 0 || o.closed.Load() != 0 {
		t.Fatal("foreign release retired owner")
	}
	if _, e := RequestTest(context.Background(), r, s.listener.Addr().String(), signer.PublicKey(), signer); e != nil {
		t.Fatal(e)
	}
}

func TestIndependentMaintenanceRejectsReplayReflectionAndOverrides(t *testing.T) {
	r, signer, o, s, _ := testEndpoint(t)
	c, _, challenge := challengeFor(t, r, s)
	challenge.Op = "authenticate"
	proof, e := sign(challenge, signer)
	if e != nil {
		t.Fatal(e)
	}
	c.Close()
	for _, mode := range []string{"replay", "reflection", "wrong-key", "scope", "release", "epoch", "override", "duplicate", "oversize"} {
		t.Run(mode, func(t *testing.T) {
			c, reader, current := challengeFor(t, r, s)
			defer c.Close()
			var send any
			switch mode {
			case "replay":
				if current.ServerNonce == proof.ServerNonce {
					t.Fatal("server nonce reused")
				}
				send = proof
			case "reflection":
				send = current
			case "wrong-key":
				_, k, _ := ed25519.GenerateKey(rand.Reader)
				current.Op = "authenticate"
				send, _ = sign(current, memorySigner{k})
			case "scope":
				current.Op = "authenticate"
				current.Scope = strings.Repeat("f", 32)
				send, _ = sign(current, signer)
			case "epoch":
				current.Op = "authenticate"
				current.Epoch = 9
				send, _ = sign(current, signer)
			case "release":
				current.Op = "authenticate"
				current.Release = strings.Repeat("f", 64)
				send, _ = sign(current, signer)
			case "override":
				c.Write([]byte("{\"op\":\"authenticate\",\"path\":\"/tmp/remove\"}\n"))
			case "duplicate":
				c.Write([]byte("{\"op\":\"authenticate\",\"op\":\"authenticate\"}\n"))
			case "oversize":
				c.Write([]byte(strings.Repeat("x", 5000) + "\n"))
			}
			if send != nil {
				json.NewEncoder(c).Encode(send)
			}
			var result frame
			if reader.Read(&result) == nil {
				t.Fatal("unauthorized request received reply")
			}
			if o.maint.Load() != 0 || o.closed.Load() != 0 {
				t.Fatal("unauthorized request retired owner")
			}
		})
	}
	if _, e = RequestTest(context.Background(), r, s.listener.Addr().String(), signer.PublicKey(), signer); e != nil {
		t.Fatal("invalid attempt poisoned authorized request", e)
	}
}

func TestIndependentMaintenanceDisconnectAfterReadyNeverClosesOwner(t *testing.T) {
	r, signer, o, s, done := testEndpoint(t)
	c, reader, base := challengeFor(t, r, s)
	base.Op = "authenticate"
	proof, _ := sign(base, signer)
	json.NewEncoder(c).Encode(proof)
	var ready frame
	if reader.Read(&ready) != nil || ready.Op != "ready" {
		t.Fatal("ready")
	}
	c.Close()
	if e := <-done; e == nil || o.maint.Load() != 1 || o.closed.Load() != 0 {
		t.Fatal("disconnect released owner", e)
	}
}

func TestIndependentMaintenanceCloseJoinsStalledReader(t *testing.T) {
	r, _, o, s, done := testEndpoint(t)
	c, _, _ := challengeFor(t, r, s)
	defer c.Close()
	s.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("stalled reader did not exit")
	}
	if o.maint.Load() != 0 || o.closed.Load() != 0 {
		t.Fatal("close mutated owner")
	}
}

func TestIndependentMaintenanceProductionAndNonlocalDenied(t *testing.T) {
	r := releaseFixture(t, false)
	_, key, _ := ed25519.GenerateKey(rand.Reader)
	signer := memorySigner{key}
	o := &ownerStub{}
	l, e := net.Listen("tcp4", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	if _, e = NewTest(r, o, signer, l); e == nil {
		t.Fatal("production endpoint enabled")
	}
	if _, e = RequestTest(context.Background(), r, l.Addr().String(), signer.PublicKey(), signer); e == nil {
		t.Fatal("production request enabled")
	}
	for _, a := range []string{"example.com:80", "0.0.0.0:80", "192.168.0.1:80", "[::]:80", "127.0.0.1:0", "127.0.0.1:http", "127.0.0.1:70000"} {
		if localAddress(a) {
			t.Fatal("nonlocal address", a)
		}
	}
}

func TestIndependentMaintenanceClientRejectsForgedServerTranscript(t *testing.T) {
	for _, mode := range []string{"challenge", "ready", "closed"} {
		t.Run(mode, func(t *testing.T) {
			r := releaseFixture(t, true)
			_, key, _ := ed25519.GenerateKey(rand.Reader)
			s := memorySigner{key}
			l, e := net.Listen("tcp4", "127.0.0.1:0")
			if e != nil {
				t.Fatal(e)
			}
			defer l.Close()
			done := make(chan struct{})
			go func() {
				defer close(done)
				c, e := l.Accept()
				if e != nil {
					return
				}
				defer c.Close()
				c.SetDeadline(time.Now().Add(3 * time.Second))
				read := privatepipe.NewBoundedReader(c, 4096)
				write := json.NewEncoder(c)
				var hello frame
				if read.Read(&hello) != nil {
					return
				}
				base := frame{Op: "challenge", Version: 1, Scope: r.Scope(), Release: releaseDigest(r), Instance: strings.Repeat("a", 32), ClientNonce: hello.ClientNonce, ServerNonce: strings.Repeat("c", 64)}
				challenge, _ := sign(base, s)
				if mode == "challenge" {
					challenge.Instance = strings.Repeat("d", 32)
				}
				write.Encode(challenge)
				var proof frame
				if read.Read(&proof) != nil {
					return
				}
				base.Op = "authenticate"
				if !verify(proof, base, s.PublicKey()) {
					return
				}
				base.Op = "ready"
				base.Epoch = 2
				base.Transaction = strings.Repeat("b", 32)
				ready, _ := sign(base, s)
				if mode == "ready" {
					ready.Epoch++
				}
				write.Encode(ready)
				var finish frame
				if read.Read(&finish) != nil {
					return
				}
				base.Op = "finish"
				if !verify(finish, base, s.PublicKey()) {
					return
				}
				base.Op = "closed"
				closed, _ := sign(base, s)
				if mode == "closed" {
					closed.Transaction = strings.Repeat("e", 32)
				}
				write.Encode(closed)
			}()
			if _, e = RequestTest(context.Background(), r, l.Addr().String(), s.PublicKey(), s); e == nil {
				t.Fatal("forged server stage accepted")
			}
			select {
			case <-done:
			case <-time.After(3 * time.Second):
				t.Fatal("peer not joined")
			}
		})
	}
}
