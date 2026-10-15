package installeddevice

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/clientruntime"
	"remoteapp.local/selfhost-poc/deviceservice"
	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/prerelease"
	"remoteapp.local/selfhost-poc/trustinstall"
	"remoteapp.local/selfhost-poc/trustprofile"
)

type testSigner struct {
	key     ed25519.PrivateKey
	join    <-chan struct{}
	joined  chan struct{}
	joinErr error
}

func (s *testSigner) PublicKey() ed25519.PublicKey  { return s.key.Public().(ed25519.PublicKey) }
func (s *testSigner) Sign(b []byte) ([]byte, error) { return ed25519.Sign(s.key, b), nil }
func (s *testSigner) Close()                        {}
func (s *testSigner) CloseJoined() error {
	if s.join != nil {
		<-s.join
	}
	close(s.joined)
	return s.joinErr
}

func serviceFixture(t *testing.T) (trustinstall.Release, trustprofile.Profile) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := l.Addr().String()
	l.Close()
	dir := filepath.Join(root, "service")
	if err = prerelease.Create(dir, "https://"+address, "installed-device-test", time.Now()); err != nil {
		t.Fatal(err)
	}
	anchor, err := os.ReadFile(filepath.Join(dir, "client-public/installation-anchor.json"))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "client-public/service-profile.json"))
	if err != nil {
		t.Fatal(err)
	}
	spec, err := trustinstall.PublicSpecification(anchor, raw, []byte("fixture-journal"), "installed-device-fixture", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := trustinstall.EncodeForTrustedBuild(spec, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	release, err := trustinstall.DecodeCompiled(encoded)
	if err != nil {
		t.Fatal(err)
	}
	profile, err := trustprofile.Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done, ready := make(chan error, 1), make(chan struct{}, 1)
	go func() {
		done <- deviceservice.Run(ctx, deviceservice.Config{Directory: dir, ReleaseRoot: spec.Root, Service: spec.Service, MinimumEpoch: 1, Listen: address}, func(net.Addr) { ready <- struct{}{} })
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(8 * time.Second):
			t.Error("service not joined")
		}
	})
	select {
	case <-ready:
	case err := <-done:
		t.Fatal(err)
	case <-time.After(5 * time.Second):
		t.Fatal("service not ready")
	}
	return release, profile
}

func TestInstalledProfileDirectHTTPSAndSignerJoinBarrier(t *testing.T) {
	release, profile := serviceFixture(t)
	_, key, _ := ed25519.GenerateKey(rand.Reader)
	join := make(chan struct{})
	signer := &testSigner{key: key, join: join, joined: make(chan struct{})}
	commands := make(chan Request, 1)
	replies := make(chan any, 3)
	done := make(chan error, 1)
	run := Runner(release, func() (clientruntime.Signer, error) { return signer, nil }, commands, func(v any) error { replies <- v; return nil })
	go func() { done <- run(context.Background(), profile) }()
	read := func() map[string]any {
		t.Helper()
		select {
		case v := <-replies:
			raw, _ := json.Marshal(v)
			var m map[string]any
			json.Unmarshal(raw, &m)
			return m
		case <-time.After(5 * time.Second):
			t.Fatal("reply deadline")
			return nil
		}
	}
	if read()["status"] != "ready" {
		t.Fatal("not ready")
	}
	commands <- Request{Op: "device", Device: invite.Body{Action: "register"}}
	registered := read()
	if registered["passed"] != true || registered["device"].(map[string]any)["device_id"] == "" {
		t.Fatal("direct signed HTTPS register")
	}
	commands <- Request{Op: "device", Device: invite.Body{Action: "invite"}}
	if reply := read(); reply["passed"] != true || reply["device"].(map[string]any)["code"] == "" {
		t.Fatal("direct signed HTTPS invite")
	}
	commands <- Request{Op: "stop"}
	select {
	case <-done:
		t.Fatal("cancel/stop is not signer join")
	case <-time.After(30 * time.Millisecond):
	}
	close(join)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("runner join")
	}
	select {
	case <-signer.joined:
	default:
		t.Fatal("signer not joined")
	}
}

func TestProfileTamperingRejectedAndSignerJoined(t *testing.T) {
	release, profile := serviceFixture(t)
	for _, mode := range []string{"endpoint", "issuer", "signature"} {
		t.Run(mode, func(t *testing.T) {
			p := profile
			switch mode {
			case "endpoint":
				p.Claims.Endpoint = "https://127.0.0.1:1"
			case "issuer":
				p.Claims.Issuer = strings.Repeat("a", 64)
			case "signature":
				p.Signature = strings.Repeat("0", 128)
			}
			_, key, _ := ed25519.GenerateKey(rand.Reader)
			signer := &testSigner{key: key, joined: make(chan struct{})}
			err := Runner(release, func() (clientruntime.Signer, error) { return signer, nil }, make(chan Request), func(any) error { t.Fatal("untrusted ready/result"); return nil })(context.Background(), p)
			if err == nil {
				t.Fatal("tampered profile")
			}
			select {
			case <-signer.joined:
			default:
				t.Fatal("signer not joined")
			}
		})
	}
}

func TestStrictLocalOperationsNoIdentityOrTrustReplacement(t *testing.T) {
	open := Request{Op: "open", VaultScope: strings.Repeat("a", 32), ExpectedPublic: strings.Repeat("b", 64)}
	if !ValidOpen(open) {
		t.Fatal("valid open")
	}
	open.Device = invite.Body{Action: "register"}
	if ValidOpen(open) {
		t.Fatal("open with action")
	}
	for _, r := range []Request{{Op: "initialize"}, {Op: "upgrade"}, {Op: "enroll"}, {Op: "device", VaultScope: "replacement", Device: invite.Body{Action: "register"}}, {Op: "device", Device: invite.Body{Action: "register", Nonce: "parent-nonce"}}, {Op: "device", Device: invite.Body{Action: "register", IssuedAt: 1}}, {Op: "stop", Device: invite.Body{Action: "register"}}} {
		if validCommand(r) {
			t.Fatal("invalid command allowed")
		}
	}
}

func TestSignerJoinFailureCannotReturnSuccessfulStop(t *testing.T) {
	release, profile := serviceFixture(t)
	_, key, _ := ed25519.GenerateKey(rand.Reader)
	signer := &testSigner{key: key, joined: make(chan struct{}), joinErr: trustprofile.ErrTrust}
	commands := make(chan Request, 1)
	commands <- Request{Op: "stop"}
	if Runner(release, func() (clientruntime.Signer, error) { return signer, nil }, commands, func(any) error { return nil })(context.Background(), profile) == nil {
		t.Fatal("failed signer join reported successful stop")
	}
}

func TestIdleCancellationAndDeadlineJoinSigner(t *testing.T) {
	release, profile := serviceFixture(t)
	for _, expiry := range []bool{false, true} {
		_, key, _ := ed25519.GenerateKey(rand.Reader)
		signer := &testSigner{key: key, joined: make(chan struct{})}
		var ctx context.Context
		var cancel context.CancelFunc
		if expiry {
			ctx, cancel = context.WithTimeout(context.Background(), 30*time.Millisecond)
		} else {
			ctx, cancel = context.WithCancel(context.Background())
		}
		ready := make(chan struct{}, 1)
		done := make(chan error, 1)
		go func() {
			done <- Runner(release, func() (clientruntime.Signer, error) { return signer, nil }, make(chan Request), func(any) error { ready <- struct{}{}; return nil })(ctx, profile)
		}()
		select {
		case <-ready:
		case <-time.After(3 * time.Second):
			cancel()
			t.Fatal("not ready")
		}
		if !expiry {
			cancel()
		}
		select {
		case err := <-done:
			if (err != nil) != expiry {
				t.Fatal("deadline must fault; explicit cancel must join normally")
			}
		case <-time.After(3 * time.Second):
			cancel()
			t.Fatal("idle client not joined")
		}
		cancel()
		select {
		case <-signer.joined:
		default:
			t.Fatal("signer not joined")
		}
	}
}

func TestAutonomousOpenCannotSelectInstalledIdentity(t *testing.T) {
	if !ValidAutonomousOpen(Request{Op: "open"}) {
		t.Fatal("empty autonomous open rejected")
	}
	for _, request := range []Request{
		{Op: "open", VaultScope: strings.Repeat("1", 32)},
		{Op: "open", ExpectedPublic: strings.Repeat("2", 64)},
		{Op: "open", Network: invite.NetworkRequestBody{Action: "state"}},
		{Op: "stop"},
	} {
		if ValidAutonomousOpen(request) {
			t.Fatal("runtime identity selector accepted")
		}
	}
}
