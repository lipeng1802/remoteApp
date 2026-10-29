package grantrevocation

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/big"
	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/trustinstall"
	"remoteapp.local/selfhost-poc/trustprofile"
	"strings"
	"testing"
	"time"
)

type fakeAPI struct {
	calls  []invite.Body
	reply  func(invite.Body) (invite.Result, error)
	closed bool
}

func (f *fakeAPI) DeviceRequest(_ context.Context, b invite.Body) (invite.Result, error) {
	f.calls = append(f.calls, b)
	return f.reply(b)
}
func (f *fakeAPI) Close() { f.closed = true }
func fixture(t *testing.T) (*Channel, invite.Grant, *fakeAPI, *time.Time) {
	t.Helper()
	now := time.Now().Truncate(time.Second)
	root, key, _ := ed25519.GenerateKey(rand.Reader)
	pub, _, _ := ed25519.GenerateKey(rand.Reader)
	issuer, issuerKey, _ := ed25519.GenerateKey(rand.Reader)
	ca, caKey, _ := ed25519.GenerateKey(rand.Reader)
	cert := &x509.Certificate{SerialNumber: big.NewInt(1), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour)}
	der, e := x509.CreateCertificate(rand.Reader, cert, cert, ca, caKey)
	if e != nil {
		t.Fatal(e)
	}
	p, _ := trustprofile.Sign(key, trustprofile.Claims{Version: 1, Service: "revoke-test", Epoch: 1, Endpoint: "https://127.0.0.1:8444", Certificate: base64.StdEncoding.EncodeToString(der), Issuer: hex.EncodeToString(issuer), IssuedAt: now.Add(-time.Minute).Unix(), ExpiresAt: now.Add(20 * time.Minute).Unix()})
	api := &fakeAPI{reply: func(b invite.Body) (invite.Result, error) { return invite.Result{Status: "revoked"}, nil }}
	c, e := newChannel(trustinstall.Specification{Test: true, Root: hex.EncodeToString(root), Service: p.Claims.Service, MinimumEpoch: 1, Bundled: p}, pub, api, func() time.Time { return now })
	if e != nil {
		t.Fatal(e)
	}
	id := sha256.Sum256(pub)
	claims := invite.Claims{Version: 1, ID: strings.Repeat("a", 32), Target: hex.EncodeToString(id[:]), Controller: strings.Repeat("b", 64), Permission: "fixed-payload", IssuedAt: now.Unix(), ExpiresAt: now.Add(5 * time.Minute).Unix()}
	raw, _ := json.Marshal(claims)
	g := invite.Grant{Claims: claims, Signature: hex.EncodeToString(ed25519.Sign(issuerKey, append([]byte("remoteapp-invite-grant-v1\n"), raw...)))}
	return c, g, api, &now
}
func pin(t *testing.T, c *Channel, g invite.Grant) {
	t.Helper()
	if e := c.Observe("request", g); e != nil {
		t.Fatal(e)
	}
	if e := c.Pin(g); e != nil {
		t.Fatal(e)
	}
}

func TestBoundRevokeUsesOnlyObservedGrantAndOwner(t *testing.T) {
	c, g, api, _ := fixture(t)
	if c.Pin(g) == nil || c.Revoke(context.Background()) == nil {
		t.Fatal("unobserved grant accepted")
	}
	bad := g
	bad.Signature = "invalid"
	if c.Observe("request", bad) == nil {
		t.Fatal("unsigned grant accepted")
	}
	pin(t, c, g)
	if c.Observe("other", g) == nil || c.Pin(g) == nil {
		t.Fatal("pinned grant replaced")
	}
	if e := c.Revoke(context.Background()); e != nil {
		t.Fatal(e)
	}
	if len(api.calls) != 1 || api.calls[0] != (invite.Body{Action: "revoke", GrantID: g.Claims.ID}) {
		t.Fatal("revocation was not exact")
	}
	if e := c.Revoke(context.Background()); e != nil || len(api.calls) != 1 {
		t.Fatal("confirmed mutation repeated")
	}
}
func TestUnknownReceiptQueriesBeforeAnyExplicitRetry(t *testing.T) {
	for _, state := range []string{"revoked", "approved", "foreign", "unknown"} {
		t.Run(state, func(t *testing.T) {
			c, g, api, _ := fixture(t)
			pin(t, c, g)
			mutations := 0
			api.reply = func(b invite.Body) (invite.Result, error) {
				if b.Action == "revoke" {
					mutations++
					if mutations == 1 {
						return invite.Result{}, errors.New("receipt lost")
					}
					return invite.Result{Status: "revoked"}, nil
				}
				if b.Action != "status" || b.RequestID != "request" {
					t.Fatal("wrong resolution query")
				}
				switch state {
				case "revoked":
					return invite.Result{RequestID: "request", Status: "revoked"}, nil
				case "approved":
					return invite.Result{RequestID: "request", Status: "approved", Grant: &g}, nil
				case "foreign":
					x := g
					x.Claims.ID = strings.Repeat("c", 32)
					return invite.Result{RequestID: "request", Status: "approved", Grant: &x}, nil
				default:
					return invite.Result{}, errors.New("status unavailable")
				}
			}
			if c.Revoke(context.Background()) == nil {
				t.Fatal("lost receipt accepted")
			}
			e := c.Revoke(context.Background())
			if (state == "revoked" || state == "approved") != (e == nil) {
				t.Fatal("unexpected resolution")
			}
			if len(api.calls) < 2 || api.calls[1].Action != "status" {
				t.Fatal("blind mutation retry")
			}
			want := 1
			if state == "approved" {
				want = 2
			}
			if mutations != want {
				t.Fatal("mutation repeated without status proof")
			}
		})
	}
}
func TestExpiredClosedControllerAndCancelledContextsCannotSign(t *testing.T) {
	for _, mode := range []string{"expired", "rollback", "closed", "controller", "cancelled"} {
		t.Run(mode, func(t *testing.T) {
			c, g, api, clock := fixture(t)
			pin(t, c, g)
			ctx := context.Background()
			switch mode {
			case "rollback":
				*clock = clock.Add(-time.Hour)
			case "expired":
				*clock = clock.Add(time.Hour)
			case "closed":
				c.Close()
			case "controller":
				c.subject = g.Claims.Controller
			case "cancelled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			if c.Revoke(ctx) == nil || len(api.calls) != 0 {
				t.Fatal("ineligible caller signed")
			}
		})
	}
	if _, e := New(trustinstall.Specification{}, nil, false); e == nil {
		t.Fatal("production enabled")
	}
}
