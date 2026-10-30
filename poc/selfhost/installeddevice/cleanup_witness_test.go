package installeddevice

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"math/big"
	"remoteapp.local/selfhost-poc/cleanupwitness"
	"remoteapp.local/selfhost-poc/clientruntime"
	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/trustinstall"
	"remoteapp.local/selfhost-poc/trustprofile"
	"strings"
	"testing"
	"time"
)

// Outer root barriers are fixtures; signer/runner hooks execute real code.
func TestRunnerCleanupWitnessSeparatesFaultAndSignerFailure(t *testing.T) {
	// This test sends no HTTPS requests; avoid the unrelated file-bootstrap
	// directory-sync prerequisite (unsupported by this Windows fixture).
	now := time.Now()
	pub, key, _ := ed25519.GenerateKey(rand.Reader)
	ca, caKey, _ := ed25519.GenerateKey(rand.Reader)
	issuer, _, _ := ed25519.GenerateKey(rand.Reader)
	cert := &x509.Certificate{SerialNumber: big.NewInt(1), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour)}
	der, e := x509.CreateCertificate(rand.Reader, cert, cert, ca, caKey)
	if e != nil {
		t.Fatal(e)
	}
	profile, e := trustprofile.Sign(key, trustprofile.Claims{Version: 1, Service: "cleanup-runner", Epoch: 1, Endpoint: "https://127.0.0.1:1", Certificate: base64.StdEncoding.EncodeToString(der), Issuer: hex.EncodeToString(issuer), IssuedAt: now.Add(-time.Minute).Unix(), ExpiresAt: now.Add(20 * time.Minute).Unix()})
	if e != nil {
		t.Fatal(e)
	}
	for _, mode := range []string{"business-fault", "signer-failure", "production"} {
		t.Run(mode, func(t *testing.T) {
			spec := trustinstall.Specification{Version: 1, Channel: "cleanup-runner-test", Root: hex.EncodeToString(pub), Service: profile.Claims.Service, MinimumEpoch: 1, HelperDigest: strings.Repeat("a", 64), Bundled: profile, Test: mode != "production"}
			encoded, e := trustinstall.EncodeForTrustedBuild(spec, time.Now())
			if e != nil {
				t.Fatal(e)
			}
			r, e := trustinstall.DecodeCompiled(encoded)
			if e != nil {
				t.Fatal(e)
			}
			w := &cleanupwitness.Witness{}
			for _, k := range []cleanupwitness.Resource{cleanupwitness.OwnerJournal, cleanupwitness.BindingJournal, cleanupwitness.CommandReader} {
				w.Begin(k)
				w.Complete(k, nil)
			}
			_, key, _ := ed25519.GenerateKey(rand.Reader)
			join := make(chan struct{})
			s := &testSigner{key: key, join: join, joined: make(chan struct{})}
			if mode == "signer-failure" {
				s.joinErr = trustprofile.ErrTrust
			}
			commands := make(chan Request, 1)
			ready := make(chan struct{}, 1)
			done := make(chan error, 1)
			run := Runner(r, func() (clientruntime.Signer, error) { return s, nil }, commands, func(v any) error {
				select {
				case ready <- struct{}{}:
				default:
				}
				return nil
			})
			go func() { done <- run(cleanupwitness.Context(context.Background(), w), profile) }()
			<-ready
			commands <- Request{Op: "invalid"}
			select {
			case <-done:
				t.Fatal("signer not actually joined")
			case <-time.After(20 * time.Millisecond):
			}
			close(join)
			if e := <-done; e == nil {
				t.Fatal("business fault hidden")
			}
			w.Seal()
			if w.Verified() != (mode == "business-fault") {
				t.Fatal("cleanup/production gates conflated with work fault")
			}
		})
	}
}

func TestEnrollmentWitnessJoinsFaultedWorkButRejectsNodeCleanupError(t *testing.T) {
	for _, bad := range []bool{false, true} {
		t.Run(map[bool]string{false: "fault-cleaned", true: "node-close-failed"}[bad], func(t *testing.T) {
			g, grant, key, e, binding := watchedFixture(t)
			w := &cleanupwitness.Witness{}
			for _, k := range []cleanupwitness.Resource{cleanupwitness.OwnerJournal, cleanupwitness.BindingJournal, cleanupwitness.CommandReader, cleanupwitness.Runner, cleanupwitness.Signer} {
				w.Begin(k)
				w.Complete(k, nil)
			}
			w.Arm(true)
			e.witness = w
			w.Begin(cleanupwitness.Node)
			client := requestFunc(func(context.Context, invite.NetworkRequestBody) (invite.NetworkReply, error) {
				return watchState(key, grant, binding, true, g.clock()), nil
			})
			if _, err := e.startWatch(context.Background(), g, client); err != nil {
				t.Fatal(err)
			}
			e.peers = &SessionPeers{grant: grant, binding: binding, server: true}
			e.peers.runtime = &SessionRuntime{peers: e.peers}
			if err := e.startSession(context.Background(), func(ctx context.Context, _ *SessionRuntime) error { <-ctx.Done(); return errors.New("business fault") }); err != nil {
				t.Fatal(err)
			}
			if bad {
				e.node.(*proofNodeFixture).err = invite.ErrDenied
			}
			if e.closeJoined() == nil {
				t.Fatal("work fault hidden")
			}
			w.Seal()
			if w.Verified() == bad {
				t.Fatal("node cleanup failure became proof")
			}
		})
	}
}
