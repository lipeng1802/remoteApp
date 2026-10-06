package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/invite"
)

func authorizedFixture(t *testing.T) (config, *invite.Store, ed25519.PrivateKey) {
	t.Helper()
	s, e := invite.Open(filepath.Join(t.TempDir(), "state"), time.Now)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.Close() })
	_, owner, _ := ed25519.GenerateKey(rand.Reader)
	_, client, _ := ed25519.GenerateKey(rand.Reader)
	exec := func(k ed25519.PrivateKey, b invite.Body) invite.Result {
		n := make([]byte, 16)
		rand.Read(n)
		b.Nonce = hex.EncodeToString(n)
		b.IssuedAt = time.Now().Unix()
		r, e := s.Execute(invite.Sign(k, b))
		if e != nil {
			t.Fatal(e)
		}
		return r
	}
	target := exec(owner, invite.Body{Action: "register"})
	exec(client, invite.Body{Action: "register"})
	i := exec(owner, invite.Body{Action: "invite"})
	r := exec(client, invite.Body{Action: "claim", TargetCode: target.DeviceCode, Code: i.Code})
	g := exec(owner, invite.Body{Action: "approve", RequestID: r.RequestID, Allow: true})
	b := invite.NetworkBinding{TargetNode: "nodekey:" + strings.Repeat("a", 64), ControllerNode: "nodekey:" + strings.Repeat("b", 64), TargetIP: "100.120.0.1", ControllerIP: "100.120.0.2"}
	n, e := s.NetworkState(*g.Grant, b)
	if e != nil {
		t.Fatal(e)
	}
	c := validConfig()
	c.SessionToken = ""
	c.Authorization = &authorizationConfig{Issuer: hex.EncodeToString(s.PublicKey()), PrivateKey: hex.EncodeToString(client), Target: hex.EncodeToString(owner.Public().(ed25519.PublicKey)), Controller: hex.EncodeToString(client.Public().(ed25519.PublicKey)), State: n}
	return c, s, owner
}
func TestAuthorizationBootstrapAndSignedRefresh(t *testing.T) {
	c, s, _ := authorizedFixture(t)
	raw, _ := json.Marshal(c)
	parsed, _, e := parseConfig(strings.NewReader(string(raw)))
	if e != nil {
		t.Fatal(e)
	}
	n, e := s.NetworkState(c.Authorization.State.Claims.Grant, c.Authorization.State.Claims.Binding)
	if e != nil {
		t.Fatal(e)
	}
	line, _ := json.Marshal(n)
	if !updateState(parsed, line) {
		t.Fatal("signed refresh rejected")
	}
	g := n.Claims.Grant
	if !parsed.cache.Check(context.Background(), g, g.Claims.Target, g.Claims.Controller) {
		t.Fatal("cache rejected")
	}
	n.Claims.Binding.ControllerIP = "100.120.0.3"
	line, _ = json.Marshal(n)
	if updateState(parsed, line) || updateState(parsed, []byte("{} {}")) {
		t.Fatal("invalid state accepted")
	}
}
func TestAuthorizationNoLegacyOrUntrustedMode(t *testing.T) {
	for _, kind := range []string{"token", "stream", "reject", "issuer", "binding", "missing-key"} {
		t.Run(kind, func(t *testing.T) {
			c, _, _ := authorizedFixture(t)
			switch kind {
			case "token":
				c.SessionToken = strings.Repeat("a", 64)
			case "stream":
				c.Stream = true
			case "reject":
				c.CheckReject = true
			case "issuer":
				c.Authorization.Issuer = strings.Repeat("a", 64)
			case "binding":
				c.Authorization.State.Claims.Binding.ControllerNode = c.Authorization.State.Claims.Binding.TargetNode
			case "missing-key":
				c.Authorization.PrivateKey = ""
			}
			raw, _ := json.Marshal(c)
			if _, _, e := parseConfig(strings.NewReader(string(raw))); e == nil {
				t.Fatal("invalid auth mode accepted")
			}
		})
	}
}
