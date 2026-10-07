package invite

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"net/http/httptest"
	"testing"
	"time"
)

func TestStateWatchSnapshotGrantDeadlinePrecedesStateLease(t *testing.T) {
	f, g, b, _ := provisionFixture(t)
	bindPair(t, f, g, b)
	n, _ := b.NetworkState(g)
	n.Claims.IssuedAt = time.Unix(g.Claims.ExpiresAt-1, 0).UnixNano()
	n.Claims.ExpiresAt = n.Claims.IssuedAt + int64(3*time.Second)
	key, _ := hex.DecodeString(f.s.state.PrivateKey)
	n.Signature = hex.EncodeToString(ed25519.Sign(key, networkBytes(n.Claims)))
	now := time.Unix(g.Claims.ExpiresAt, 0)
	if !VerifyNetworkState(f.s.PublicKey(), n, now) {
		t.Fatal("test lease not valid")
	}
	w := &StateWatch{latest: &n, issuer: f.s.PublicKey(), grant: g, binding: n.Claims.Binding, clock: func() time.Time { return now }}
	if _, e := w.Snapshot(); e != ErrDenied {
		t.Fatal("state lease extended grant")
	}
}

func TestStateWatchRefreshRevocationNoRevivalAndStop(t *testing.T) {
	f, g, b, _ := provisionFixture(t)
	bindPair(t, f, g, b)
	server := httptest.NewTLSServer(NetworkHandler(b))
	defer server.Close()
	c, e := NewNetworkClient(server.URL, server.Certificate().Raw, f.client)
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	c.now = func() time.Time { return f.now }
	n, _ := b.NetworkState(g)
	w, e := c.StartStateWatch(context.Background(), f.s.PublicKey(), g, n.Claims.Binding, func() time.Time { return f.now })
	if e != nil {
		t.Fatal(e)
	}
	defer w.Stop()
	wait := func(active bool) {
		t.Helper()
		end := time.Now().Add(3 * time.Second)
		for time.Now().Before(end) {
			n, e := w.Snapshot()
			if e == nil && n.Claims.Active == active {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatal("watch state deadline")
	}
	wait(true)
	time.Sleep(600 * time.Millisecond)
	wait(true)
	f.exec(t, f.owner, Body{Action: "revoke", GrantID: g.Claims.ID})
	wait(false)
	select {
	case <-w.done:
	case <-time.After(2 * time.Second):
		t.Fatal("inactive watch not terminal")
	}
	w.Stop()
	if _, e = w.Snapshot(); e != ErrDenied {
		t.Fatal("stopped cache authorized")
	}
}
func TestStateWatchAuthorityUnavailableClearsCachedActive(t *testing.T) {
	f, g, b, _ := provisionFixture(t)
	bindPair(t, f, g, b)
	server := httptest.NewTLSServer(NetworkHandler(b))
	defer server.Close()
	c, _ := NewNetworkClient(server.URL, server.Certificate().Raw, f.client)
	defer c.Close()
	c.now = func() time.Time { return f.now }
	n, _ := b.NetworkState(g)
	w, e := c.StartStateWatch(context.Background(), f.s.PublicKey(), g, n.Claims.Binding, func() time.Time { return f.now })
	if e != nil {
		t.Fatal(e)
	}
	defer w.Stop()
	end := time.Now().Add(2 * time.Second)
	for {
		if _, e = w.Snapshot(); e == nil {
			break
		}
		if time.Now().After(end) {
			t.Fatal("initial state")
		}
		time.Sleep(10 * time.Millisecond)
	}
	f.s.Close()
	select {
	case <-w.done:
	case <-time.After(2 * time.Second):
		t.Fatal("failed query did not end watch")
	}
	if _, e = w.Snapshot(); e != ErrDenied {
		t.Fatal("failed watch retained active")
	}
}
