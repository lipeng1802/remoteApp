package invite

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestDesktopPolicyHTTPSWatchPinsScopeAndJoinsAfterRevocation(t *testing.T) {
	for _, kind := range []string{"revoke", "mixed_reply"} {
		t.Run(kind, func(t *testing.T) {
			f, g, p, b, key := policyFixture(t, DesktopControl)
			var queries atomic.Int64
			var mixed atomic.Bool
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var req NetworkRequest
				if decodeHTTP(r.Body, &req) != nil || req.Body.Action != "state" || req.Body.Grant != g {
					http.Error(w, "denied", 403)
					return
				}
				pub, _ := hex.DecodeString(req.PublicKey)
				sig, _ := hex.DecodeString(req.Signature)
				if req.PublicKey != hex.EncodeToString(f.client.Public().(ed25519.PublicKey)) || !ed25519.Verify(pub, networkRequestBytes(req.Body), sig) {
					http.Error(w, "denied", 403)
					return
				}
				queries.Add(1)
				n := NetworkState{Claims: NetworkClaims{Version: 1, Grant: g, Binding: b, Active: f.s.AuthorizedDesktop(g, g.Claims.Target, g.Claims.Controller, DesktopControl), IssuedAt: f.now.UnixNano(), ExpiresAt: f.now.Add(3 * time.Second).UnixNano()}}
				n.Signature = hex.EncodeToString(ed25519.Sign(key, networkBytes(n.Claims)))
				w.Header().Set("Content-Type", "application/json")
				reply := NetworkReply{State: &n}
				if mixed.Load() {
					reply.Proof = &ClockProof{}
				}
				json.NewEncoder(w).Encode(reply)
			}))
			defer server.Close()
			c, err := NewNetworkClient(server.URL, server.Certificate().Raw, f.client)
			if err != nil {
				t.Fatal(err)
			}
			defer c.Close()
			c.now = func() time.Time { return f.now }
			if watch, e := c.StartStateWatch(context.Background(), f.s.PublicKey(), g, b, c.now); e != ErrDenied || watch != nil {
				t.Fatal("legacy watch accepted desktop grant")
			}
			wrong, _ := DesktopGrantPolicy(DesktopView)
			if watch, e := c.StartStateWatchForPolicy(context.Background(), wrong, f.s.PublicKey(), g, b, c.now); e != ErrDenied || watch != nil || queries.Load() != 0 {
				t.Fatal("wrong scope reached network")
			}
			watch, err := c.StartStateWatchForPolicy(context.Background(), p, f.s.PublicKey(), g, b, c.now)
			if err != nil {
				t.Fatal(err)
			}
			defer watch.Stop()
			end := time.Now().Add(2 * time.Second)
			for {
				n, e := watch.Snapshot()
				if e == nil && n.Claims.Active {
					break
				}
				if time.Now().After(end) {
					t.Fatal("initial scoped watch")
				}
				time.Sleep(10 * time.Millisecond)
			}
			if kind == "revoke" {
				f.exec(t, f.owner, Body{Action: "revoke", GrantID: g.Claims.ID})
			} else {
				mixed.Store(true)
			}
			select {
			case <-watch.Done():
			case <-time.After(2 * time.Second):
				t.Fatal("revoked watch not joined")
			}
			if kind == "revoke" {
				if n, e := watch.Snapshot(); e != nil || n.Claims.Active {
					t.Fatal("inactive revocation result missing")
				}
			} else if _, e := watch.Snapshot(); e != ErrDenied {
				t.Fatal("mixed state/clock receipt retained cached authorization")
			}
			watch.Stop()
			if _, e := watch.Snapshot(); e != ErrDenied {
				t.Fatal("stopped scoped watch retained authorization")
			}

		})
	}
}

func TestDesktopWatchGrantDeadlinePrecedesSignedStateLease(t *testing.T) {
	f, g, p, b, key := policyFixture(t, DesktopView)
	now := time.Unix(g.Claims.ExpiresAt, 0)
	n := NetworkState{Claims: NetworkClaims{Version: 1, Grant: g, Binding: b, Active: true, IssuedAt: now.Add(-time.Second).UnixNano(), ExpiresAt: now.Add(2 * time.Second).UnixNano()}}
	n.Signature = hex.EncodeToString(ed25519.Sign(key, networkBytes(n.Claims)))
	if !VerifyNetworkState(f.s.PublicKey(), n, now) {
		t.Fatal("fixture state lease not current")
	}
	watch := &StateWatch{policy: p, latest: &n, issuer: f.s.PublicKey(), grant: g, binding: b, clock: func() time.Time { return now }}
	if _, e := watch.Snapshot(); e != ErrDenied {
		t.Fatal("state lease extended desktop grant")
	}
}
