package streamgate

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/invite"
)

func TestDynamicGateValidationAndLease(t *testing.T) {
	for _, mode := range []string{"valid", "wrong_role", "wrong_key", "wrong_issuer", "wrong_subject", "tamper_binding", "inactive", "expired", "revoke_replay", "binding_update", "deny"} {
		t.Run(mode, func(t *testing.T) {
			now := time.Unix(1800000000, 0)
			s, e := invite.Open(filepath.Join(t.TempDir(), "private"), func() time.Time { return now })
			if e != nil {
				t.Fatal(e)
			}
			defer s.Close()
			_, target, _ := ed25519.GenerateKey(rand.Reader)
			_, client, _ := ed25519.GenerateKey(rand.Reader)
			_, other, _ := ed25519.GenerateKey(rand.Reader)
			exec := func(k ed25519.PrivateKey, b invite.Body) invite.Result {
				n := make([]byte, 16)
				rand.Read(n)
				b.Nonce = hex.EncodeToString(n)
				b.IssuedAt = now.Unix()
				v, e := s.Execute(invite.Sign(k, b))
				if e != nil {
					t.Fatal(e)
				}
				return v
			}
			tgt := exec(target, invite.Body{Action: "register"})
			exec(client, invite.Body{Action: "register"})
			inv := exec(target, invite.Body{Action: "invite"})
			req := exec(client, invite.Body{Action: "claim", TargetCode: tgt.DeviceCode, Code: inv.Code})
			grant := *exec(target, invite.Body{Action: "approve", RequestID: req.RequestID, Allow: true}).Grant
			b := invite.NetworkBinding{TargetNode: "nodekey:" + strings.Repeat("a", 64), ControllerNode: "nodekey:" + strings.Repeat("b", 64), TargetIP: "100.120.0.42", ControllerIP: "100.120.0.93"}
			n, e := s.NetworkState(grant, b)
			if e != nil {
				t.Fatal(e)
			}
			issuer, tp, cp, key, server := s.PublicKey(), target.Public().(ed25519.PublicKey), client.Public().(ed25519.PublicKey), target, true
			switch mode {
			case "wrong_role":
				server = false
			case "wrong_key":
				key = other
			case "wrong_issuer":
				issuer = other.Public().(ed25519.PublicKey)
			case "wrong_subject":
				cp = other.Public().(ed25519.PublicKey)
			case "tamper_binding":
				n.Claims.Binding.ControllerIP = "100.120.0.94"
			case "inactive":
				exec(target, invite.Body{Action: "revoke", GrantID: grant.Claims.ID})
				n, _ = s.NetworkState(grant, b)
			case "expired":
				now = now.Add(4 * time.Second)
			}
			g, e := New(issuer, tp, cp, key, n, server, func() time.Time { return now })
			switch mode {
			case "valid", "revoke_replay", "binding_update", "deny":
				if e != nil || !g.active(context.Background()) {
					t.Fatal("valid dynamic gate")
				}
			default:
				if e == nil {
					t.Fatal("invalid bootstrap accepted")
				}
				return
			}
			switch mode {
			case "valid":
				now = now.Add(4 * time.Second)
				if g.active(context.Background()) || g.Update(n) {
					t.Fatal("expired state extended")
				}
			case "revoke_replay":
				exec(target, invite.Body{Action: "revoke", GrantID: grant.Claims.ID})
				inactive, _ := s.NetworkState(grant, b)
				if !g.Update(inactive) || !g.Fresh(n) || g.Update(n) || g.active(context.Background()) {
					t.Fatal("revocation replay")
				}
			case "binding_update":
				b.ControllerIP = "100.120.0.94"
				n, _ = s.NetworkState(grant, b)
				if g.Update(n) {
					t.Fatal("authority binding switch silently accepted")
				}
			case "deny":
				g.Deny()
				if g.active(context.Background()) {
					t.Fatal("deny not latched")
				}
			}
		})
	}
}
