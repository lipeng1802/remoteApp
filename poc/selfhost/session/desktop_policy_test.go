package session

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/invite"
)

type desktopWireFixture struct {
	server, controller Config
	issuer             ed25519.PrivateKey
	allowed            atomic.Bool
	now                atomic.Int64
}

func desktopWireSetup(t *testing.T) *desktopWireFixture {
	t.Helper()
	f := &desktopWireFixture{}
	f.allowed.Store(true)
	f.now.Store(time.Unix(1800000000, 0).UnixNano())
	clock := func() time.Time { return time.Unix(0, f.now.Load()) }
	pub, issuer, _ := ed25519.GenerateKey(rand.Reader)
	f.issuer = issuer
	tp, tk, _ := ed25519.GenerateKey(rand.Reader)
	cp, ck, _ := ed25519.GenerateKey(rand.Reader)
	policy, _ := invite.DesktopGrantPolicy(invite.DesktopControl)
	g := invite.Grant{Claims: invite.Claims{Version: 1, ID: strings.Repeat("a", 32), Target: Identity(tp), Controller: Identity(cp), Permission: string(invite.DesktopControl), IssuedAt: clock().Unix(), ExpiresAt: clock().Unix() + 300}}
	raw, _ := json.Marshal(g.Claims)
	g.Signature = hex.EncodeToString(ed25519.Sign(issuer, append([]byte("remoteapp-invite-grant-v1\n"), raw...)))
	f.server = Config{GrantPolicy: policy, Issuer: pub, Key: tk, Grant: g, Now: clock, Binding: Binding{Target: tp, Controller: cp, TargetIP: netip.MustParseAddr("127.0.0.1"), ControllerIP: netip.MustParseAddr("127.0.0.1")}}
	f.controller = f.server
	f.controller.Key = ck
	return f
}

func (f *desktopWireFixture) check(ctx context.Context, g invite.Grant, a, b string) bool {
	return ctx.Err() == nil && f.allowed.Load() && g == f.server.Grant && a == g.Claims.Target && b == g.Claims.Controller
}

func TestDesktopMutualSecretRequiresBothLocalScopes(t *testing.T) {
	for _, kind := range []string{"same", "controller-view", "target-fixed"} {
		t.Run(kind, func(t *testing.T) {
			f := desktopWireSetup(t)
			if kind == "controller-view" {
				f.controller.GrantPolicy, _ = invite.DesktopGrantPolicy(invite.DesktopView)
			}
			if kind == "target-fixed" {
				f.server.GrantPolicy = invite.GrantPolicy{}
			}
			s, c := sockets(t)
			ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
			defer cancel()
			done := make(chan error, 1)
			go func() {
				done <- DeliverProductSecret(ctx, s, f.server, f.check, "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=", strings.Repeat("c", 64))
			}()
			key, fp, err := ReceiveProductSecret(ctx, c, f.controller, f.check)
			serverErr := <-done
			if kind == "same" {
				if err != nil || serverErr != nil || key == "" || fp != strings.Repeat("c", 64) {
					t.Fatal("scoped secret exchange rejected", err, serverErr)
				}
			} else if err == nil || key != "" || fp != "" || serverErr == nil {
				t.Fatal("mismatched scope received secret")
			}
		})
	}
}

func TestDesktopOpaqueTunnelRevocationJoinsBothCopyWorkers(t *testing.T) {
	f := desktopWireSetup(t)
	s, c := sockets(t)
	app, clientLocal := net.Pipe()
	service, serverLocal := net.Pipe()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	defer app.Close()
	defer service.Close()
	done := make(chan error, 2)
	go func() {
		done <- Tunnel(ctx, s, f.server, f.check, true, func(context.Context) (net.Conn, error) { return serverLocal, nil })
	}()
	go func() {
		done <- Tunnel(ctx, c, f.controller, f.check, false, func(context.Context) (net.Conn, error) { return clientLocal, nil })
	}()
	defer func() { cancel(); s.Close(); c.Close(); serverLocal.Close(); clientLocal.Close() }()
	written := make(chan error, 1)
	go func() { _, err := app.Write([]byte("desktop-opaque")); written <- err }()
	service.SetReadDeadline(time.Now().Add(3 * time.Second))
	buf := make([]byte, len("desktop-opaque"))
	if _, err := io.ReadFull(service, buf); err != nil || string(buf) != "desktop-opaque" {
		cancel()
		t.Fatal("mutually authenticated opaque stream failed", err)
	}
	if err := <-written; err != nil {
		t.Fatal(err)
	}
	f.allowed.Store(false)
	for i := 0; i < 2; i++ {
		select {
		case <-done:
		case <-ctx.Done():
			t.Fatal("revoked copy workers not joined")
		}
	}
}

func TestDesktopStateCachePinsScopeAbsoluteLeaseAndRevocation(t *testing.T) {
	f := desktopWireSetup(t)
	g := f.server.Grant
	b := invite.NetworkBinding{TargetNode: "nodekey:" + strings.Repeat("a", 64), ControllerNode: "nodekey:" + strings.Repeat("b", 64), TargetIP: "100.120.0.42", ControllerIP: "100.120.0.93"}
	sign := func(active bool) invite.NetworkState {
		n := invite.NetworkState{Claims: invite.NetworkClaims{Version: 1, Grant: g, Binding: b, Active: active, IssuedAt: f.now.Load(), ExpiresAt: f.now.Load() + int64(3*time.Second)}}
		raw, _ := json.Marshal(n.Claims)
		n.Signature = hex.EncodeToString(ed25519.Sign(f.issuer, append([]byte("remoteapp-network-state-v1\n"), raw...)))
		return n
	}
	p := f.server.GrantPolicy
	good := NewStateCacheForPolicy(p, f.server.Issuer, g, b, f.server.Now)
	active := sign(true)
	if !good.Update(active) || !good.Check(context.Background(), g, g.Claims.Target, g.Claims.Controller) {
		t.Fatal("desktop cache rejected valid state")
	}
	wrong, _ := invite.DesktopGrantPolicy(invite.DesktopView)
	for _, policy := range []invite.GrantPolicy{wrong, {}} {
		if NewStateCacheForPolicy(policy, f.server.Issuer, g, b, f.server.Now).Update(active) {
			t.Fatal("cache inferred scope from issuer state")
		}
	}
	if !good.Update(sign(false)) || good.Update(active) || good.Check(context.Background(), g, g.Claims.Target, g.Claims.Controller) {
		t.Fatal("revocation replay restored scoped state")
	}
	late := NewStateCacheForPolicy(p, f.server.Issuer, g, b, f.server.Now)
	f.now.Add(int64(2 * time.Second))
	if !late.Update(active) {
		t.Fatal("valid delayed receipt rejected")
	}
	f.now.Add(int64(2 * time.Second))
	if late.Update(active) || late.Check(context.Background(), g, g.Claims.Target, g.Claims.Controller) {
		t.Fatal("receipt extended original lease")
	}
}
