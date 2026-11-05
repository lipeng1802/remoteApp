package installeddevice

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"net"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/clientruntime"
	"remoteapp.local/selfhost-poc/grantrevocation"
	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/trustinstall"
	"remoteapp.local/selfhost-poc/trustprofile"
)

func desktopHTTPSFixture(t *testing.T) (trustinstall.Release, trustprofile.Profile) {
	t.Helper()
	dir, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	st, e := invite.OpenDesktopAuthorization(filepath.Join(dir, "authority"), time.Now)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { st.Close() })
	root, key, _ := ed25519.GenerateKey(rand.Reader)
	ca, caKey, _ := ed25519.GenerateKey(rand.Reader)
	now := time.Now().Truncate(time.Second)
	cert := &x509.Certificate{SerialNumber: big.NewInt(1), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour)}
	der, e := x509.CreateCertificate(rand.Reader, cert, cert, ca, caKey)
	if e != nil {
		t.Fatal(e)
	}
	h := httptest.NewUnstartedServer(invite.Handler(st))
	h.TLS = &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: caKey}}, MinVersion: tls.VersionTLS13}
	h.StartTLS()
	t.Cleanup(h.Close)
	p, e := trustprofile.Sign(key, trustprofile.Claims{Version: 1, Service: "desktop-runner-test", Epoch: 1, Endpoint: h.URL, Certificate: base64.StdEncoding.EncodeToString(der), Issuer: hex.EncodeToString(st.PublicKey()), IssuedAt: now.Add(-time.Minute).Unix(), ExpiresAt: now.Add(20 * time.Minute).Unix()})
	if e != nil {
		t.Fatal(e)
	}
	s := trustinstall.Specification{Version: 1, Channel: "desktop-runner-fixture", Root: hex.EncodeToString(root), Service: p.Claims.Service, MinimumEpoch: 1, HelperDigest: strings.Repeat("1", 64), Bundled: p, Test: true}
	encoded, e := trustinstall.EncodeForTrustedBuild(s, time.Now())
	if e != nil {
		t.Fatal(e)
	}
	r, e := trustinstall.DecodeCompiled(encoded)
	if e != nil {
		t.Fatal(e)
	}
	return r, p
}

func TestDesktopRunnersHTTPSConsentQueryAndIndependentRevocation(t *testing.T) {
	for _, scope := range []invite.DesktopPermission{invite.DesktopView, invite.DesktopControl} {
		t.Run(string(scope), func(t *testing.T) {
			release, profile := desktopHTTPSFixture(t)
			type peer struct {
				commands chan Request
				replies  chan any
				done     chan error
				signer   *testSigner
				key      ed25519.PrivateKey
			}
			start := func() *peer {
				_, key, _ := ed25519.GenerateKey(rand.Reader)
				p := &peer{commands: make(chan Request), replies: make(chan any, 1), done: make(chan error, 1), signer: &testSigner{key: key, joined: make(chan struct{})}, key: key}
				life, cancel := context.WithCancel(context.Background())
				run := CoordinatedDesktopSession(release, func() (clientruntime.Signer, error) { return p.signer, nil }, p.commands, func(v any) error { p.replies <- v; return nil }, func(context.Context) (EnrollmentNode, error) {
					t.Error("node opened before provision")
					return nil, invite.ErrDenied
				}, func(context.Context, *SessionRuntime) error {
					t.Error("desktop started before provision")
					return invite.ErrDenied
				}, scope)
				go func() { p.done <- run(life, profile) }()
				t.Cleanup(func() {
					cancel()
					select {
					case <-p.signer.joined:
					case <-time.After(5 * time.Second):
						t.Error("signer/runner not joined")
					}
				})
				return p
			}
			read := func(p *peer) []byte {
				select {
				case v := <-p.replies:
					raw, e := json.Marshal(v)
					if e != nil {
						t.Fatal(e)
					}
					return raw
				case <-time.After(5 * time.Second):
					t.Fatal("runner reply timeout")
					return nil
				}
			}
			call := func(p *peer, b invite.Body, passed bool) invite.Result {
				p.commands <- Request{Op: "device", Device: b}
				var result struct {
					Status string        `json:"status"`
					Passed bool          `json:"passed"`
					Device invite.Result `json:"device"`
				}
				if json.Unmarshal(read(p), &result) != nil || result.Status != "device_result" || result.Passed != passed {
					t.Fatal("unexpected device result", b.Action)
				}
				return result.Device
			}
			target, controller := start(), start()
			for _, p := range []*peer{target, controller} {
				var ready map[string]string
				if json.Unmarshal(read(p), &ready) != nil || len(ready) != 2 || ready["status"] != "ready" || ready["Permission"] != string(scope) {
					t.Fatal("scoped readiness missing")
				}
			}
			id := call(target, invite.Body{Action: "register"}, true)
			call(controller, invite.Body{Action: "register"}, true)
			inv := call(target, invite.Body{Action: "invite", Permission: scope}, true)
			pending := call(controller, invite.Body{Action: "claim", Permission: scope, TargetCode: id.DeviceCode, Code: inv.Code}, true)
			incoming := call(target, invite.Body{Action: "incoming"}, true)
			if incoming.RequestID != pending.RequestID || incoming.Permission != scope {
				t.Fatal("incoming scope mismatch")
			}
			call(controller, invite.Body{Action: "approve", RequestID: pending.RequestID, Permission: scope, Allow: true}, false)
			approved := call(target, invite.Body{Action: "approve", RequestID: pending.RequestID, Permission: scope, Allow: true}, true)
			queried := call(controller, invite.Body{Action: "status", RequestID: pending.RequestID}, true)
			if approved.Grant == nil || queried.Grant == nil || *approved.Grant != *queried.Grant || queried.Permission != scope {
				t.Fatal("approved query rejected/lost scope")
			}
			var channels []*grantrevocation.Channel
			for _, p := range []*peer{target, controller} {
				signer, _ := invite.NewMemorySigner(p.key) // independent lease/transport, same device identity
				ch, e := grantrevocation.NewDesktop(release.Specification(), signer, 0, scope)
				if e != nil {
					t.Fatal(e)
				}
				t.Cleanup(ch.Close)
				if ch.Observe(pending.RequestID, *approved.Grant) != nil || ch.Pin(*approved.Grant) != nil {
					t.Fatal("independent exact grant pin")
				}
				channels = append(channels, ch)
				p.commands <- Request{Op: "stop"}
				select {
				case err := <-p.done:
					if err != nil {
						t.Fatal("runner stop failed")
					}
				case <-time.After(5 * time.Second):
					t.Fatal("runner join deadline")
				}
				select {
				case <-p.signer.joined:
				default:
					t.Fatal("runner returned before signer joined")
				}
			}
			if channels[1].Revoke(context.Background()) == nil {
				t.Fatal("controller gained revoke operation")
			}
			if channels[0].Revoke(context.Background()) != nil || channels[0].ConfirmRevoked(context.Background()) != nil || channels[1].ConfirmRevoked(context.Background()) != nil {
				t.Fatal("independent scoped revoke/query failed after runner exit")
			}
		})
	}
}
