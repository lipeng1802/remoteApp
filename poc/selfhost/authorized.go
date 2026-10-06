package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/netip"
	"time"

	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/session"
	"tailscale.com/tsnet"
)

type authorizationConfig struct {
	RejectProof bool                `json:"reject_proof,omitempty"` // isolated malicious-proof fixture, probe only
	Issuer      string              `json:"issuer"`
	PrivateKey  string              `json:"private_key"`
	Target      string              `json:"target"`
	Controller  string              `json:"controller"`
	State       invite.NetworkState `json:"state"`
}

func sessionConfig(c config) (session.Config, error) {
	a := c.Authorization
	if a == nil {
		return session.Config{}, errors.New("invalid_authorization")
	}
	if a.RejectProof && c.Role != "probe" {
		return session.Config{}, errors.New("invalid_authorization")
	}
	decode := func(s string, n int) []byte {
		b, e := hex.DecodeString(s)
		if e != nil || len(b) != n || hex.EncodeToString(b) != s {
			return nil
		}
		return b
	}
	pub, key, target, controller := decode(a.Issuer, 32), decode(a.PrivateKey, 64), decode(a.Target, 32), decode(a.Controller, 32)
	if pub == nil || key == nil || target == nil || controller == nil {
		return session.Config{}, errors.New("invalid_authorization")
	}
	expected := target
	if c.Role == "probe" {
		expected = controller
	}
	if !bytes.Equal(key, ed25519.NewKeyFromSeed(key[:32])) || !bytes.Equal(ed25519.PrivateKey(key).Public().(ed25519.PublicKey), expected) {
		return session.Config{}, errors.New("invalid_authorization")
	}
	b := a.State.Claims.Binding
	g := a.State.Claims.Grant
	if !invite.VerifyGrant(pub, g, session.Identity(target), session.Identity(controller), time.Now()) {
		return session.Config{}, errors.New("invalid_authorization")
	}
	if _, err := netip.ParseAddr(b.TargetIP); err != nil {
		return session.Config{}, errors.New("invalid_authorization")
	}
	if _, err := netip.ParseAddr(b.ControllerIP); err != nil {
		return session.Config{}, errors.New("invalid_authorization")
	}
	return session.Config{Issuer: pub, Key: key, Grant: g, Now: time.Now, Binding: session.Binding{Target: target, Controller: controller, TargetIP: netip.MustParseAddr(b.TargetIP), ControllerIP: netip.MustParseAddr(b.ControllerIP)}}, nil
}
func updateState(c config, line []byte) bool {
	d := json.NewDecoder(bytes.NewReader(line))
	d.DisallowUnknownFields()
	var n invite.NetworkState
	if d.Decode(&n) != nil {
		return false
	}
	var x any
	if d.Decode(&x) != io.EOF {
		return false
	}
	return c.cache != nil && c.cache.Update(n)
}

// Consult the authenticated current tsnet network map, not a client-supplied IP
// or hostname. Both this node and the remote node key must match provisioning.
func networkPeer(ctx context.Context, s *tsnet.Server, conn net.Conn, c config, server bool) bool {
	ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	lc, err := s.LocalClient()
	if err != nil {
		return false
	}
	status, err := lc.Status(ctx)
	if err != nil || status.Self == nil {
		return false
	}
	b := c.Authorization.State.Claims.Binding
	local, remote := b.TargetNode, b.ControllerNode
	if !server {
		local, remote = remote, local
	}
	if status.Self.PublicKey.String() != local {
		return false
	}
	peer, err := lc.WhoIs(ctx, conn.RemoteAddr().String())
	return err == nil && peer != nil && peer.Node != nil && peer.Node.Key.String() == remote
}
