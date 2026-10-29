package testretirement

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"remoteapp.local/selfhost-poc/enrollmentstore"
	"remoteapp.local/selfhost-poc/invite"
	"strings"
	"testing"
	"time"
)

type retirementCall func(context.Context, invite.Body) (invite.Result, error)

func (f retirementCall) DeviceRequest(ctx context.Context, b invite.Body) (invite.Result, error) {
	return f(ctx, b)
}

func TestOnlineProofAfterSignerJoinCannotAcceptUnknownOrExpiredReceipt(t *testing.T) {
	for _, mode := range []string{"valid", "wall_skew", "transport", "mixed", "signature", "node", "address", "challenge", "role", "join_failure", "expires_during_join", "rollback_during_join", "cancel_during_join"} {
		t.Run(mode, func(t *testing.T) {
			pub, key, _ := ed25519.GenerateKey(rand.Reader)
			now := time.Now()
			subject := strings.Repeat("a", 64)
			node := enrollmentstore.Record{Phase: "registered", NodeKey: "nodekey:" + strings.Repeat("b", 64), Address: "100.120.0.42"}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			calls, joined := 0, 0
			client := retirementCall(func(ctx context.Context, b invite.Body) (invite.Result, error) {
				calls++
				if b.Action != "node_retirement" || b.NodeKey != node.NodeKey || len(b.Challenge) != 64 || b.GrantID != "" || b.RequestID != "" {
					t.Fatal("runtime selectors escaped native record")
				}
				if mode == "transport" {
					return invite.Result{}, errors.New("private unknown")
				}
				c := invite.NodeRetirementClaims{Version: 1, Device: subject, NodeKey: node.NodeKey, Address: node.Address, GrantID: strings.Repeat("c", 32), Challenge: b.Challenge, IssuedAt: now.UnixNano(), ExpiresAt: now.Add(3 * time.Second).UnixNano()}
				if mode == "wall_skew" {
					c.IssuedAt = now.Add(20 * time.Second).UnixNano()
					c.ExpiresAt = c.IssuedAt + int64(3*time.Second)
				}
				switch mode {
				case "node":
					c.NodeKey = "nodekey:" + strings.Repeat("f", 64)
				case "address":
					c.Address = "100.120.0.93"
				case "challenge":
					c.Challenge = strings.Repeat("0", 64)
				case "role":
					c.Device = strings.Repeat("0", 64)
				}
				raw, _ := json.Marshal(c)
				p := invite.NodeRetirement{Claims: c, Signature: hex.EncodeToString(ed25519.Sign(key, append([]byte("remoteapp-node-retirement-v1\n"), raw...)))}
				if mode == "signature" {
					p.Signature = strings.Repeat("0", 128)
				}
				r := invite.Result{Status: "node_retired", Retirement: &p}
				if mode == "mixed" {
					r.DeviceCode = "PRIVATE"
				}
				return r, nil
			})
			err := confirmWithClient(ctx, client, pub, subject, node, func() time.Time { return now }, func() error {
				joined++
				if mode == "join_failure" {
					return errors.New("join incomplete")
				}
				if mode == "expires_during_join" {
					now = now.Add(3 * time.Second)
				}
				if mode == "rollback_during_join" {
					now = now.Add(-time.Second)
				}
				if mode == "cancel_during_join" {
					cancel()
				}
				return nil
			})
			if (err == nil) != (mode == "valid" || mode == "wall_skew") || calls != 1 || joined != 1 {
				t.Fatal("proof/join gate or blind retry")
			}
		})
	}
}
