package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"remoteapp.local/selfhost-poc/invite"
	"strings"
	"testing"
	"time"
)

func TestGrantRecheckCannotInitializeOrReplaceGrant(t *testing.T) {
	g := invite.Grant{Signature: "accepted"}
	if grantOperationAllowed("recheck_grant", invite.Grant{}, g) {
		t.Fatal("recheck initialized grant")
	}
	if !grantOperationAllowed("recheck_grant", g, g) {
		t.Fatal("same grant not eligible for signed proof")
	}
	for _, change := range []invite.Grant{{Signature: "tampered"}, {}} {
		if grantOperationAllowed("recheck_grant", g, change) {
			t.Fatal("grant substituted")
		}
	}
	if grantOperationAllowed("use_grant", g, g) || grantOperationAllowed("unknown", g, g) {
		t.Fatal("unknown/repeated use accepted")
	}
	if !grantOperationAllowed("use_grant", invite.Grant{}, g) {
		t.Fatal("initial use changed")
	}
}

func TestReconnectionStateRequiresValidSignatureLeaseAndGrant(t *testing.T) {
	pub, key, _ := ed25519.GenerateKey(rand.Reader)
	now := time.Now()
	g := invite.Grant{Claims: invite.Claims{Version: 1, ID: strings.Repeat("a", 32), Target: "target", Controller: "controller", Permission: "fixed-payload", IssuedAt: now.Unix() - 1, ExpiresAt: now.Unix() + 299}}
	raw, _ := json.Marshal(g.Claims)
	g.Signature = hex.EncodeToString(ed25519.Sign(key, append([]byte("remoteapp-invite-grant-v1\n"), raw...)))
	n := invite.NetworkState{Claims: invite.NetworkClaims{Version: 1, Grant: g, Active: true, IssuedAt: now.UnixNano(), ExpiresAt: now.Add(3 * time.Second).UnixNano(), Binding: invite.NetworkBinding{TargetNode: "nodekey:" + strings.Repeat("b", 64), ControllerNode: "nodekey:" + strings.Repeat("c", 64), TargetIP: "100.120.0.1", ControllerIP: "100.120.0.2"}}}
	raw, _ = json.Marshal(n.Claims)
	n.Signature = hex.EncodeToString(ed25519.Sign(key, append([]byte("remoteapp-network-state-v1\n"), raw...)))
	if !verifiedState(hex.EncodeToString(pub), &n, g, "target", "controller", now) {
		t.Fatal("valid signed state rejected")
	}
	for _, scenario := range []string{"signature", "expired", "grant", "role", "issuer"} {
		changed := n
		grant := g
		target := "target"
		issuer := hex.EncodeToString(pub)
		at := now
		switch scenario {
		case "signature":
			changed.Signature = "bad"
		case "expired":
			at = now.Add(3 * time.Second)
		case "grant":
			grant.Signature = "tampered"
		case "role":
			target = "foreign"
		case "issuer":
			issuer = "bad"
		}
		if verifiedState(issuer, &changed, grant, target, "controller", at) {
			t.Fatal("invalid state accepted", scenario)
		}
	}
}
