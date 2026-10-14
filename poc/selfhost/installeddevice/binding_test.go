package installeddevice

import (
	"context"
	"remoteapp.local/selfhost-poc/invite"
	"strings"
	"testing"
	"time"
)

type proofNodeFixture struct {
	*enrollmentNodeFixture
	ready    bool
	starts   int
	startErr error
}

func (n *proofNodeFixture) StartProof(ctx context.Context, s invite.DeviceSigner, p invite.VerifierPermit) error {
	n.starts++
	n.ready = n.startErr == nil
	return n.startErr
}
func (n *proofNodeFixture) ProofReady() bool { return n.ready }

func TestInternalBindingSignedPermitOrderingAndFailure(t *testing.T) {
	for _, scenario := range []string{"normal", "signature", "old_challenge", "wrong_ticket", "expired", "mixed", "start_fail", "bind_fail", "bind_mixed", "proof_fault"} {
		t.Run(scenario, func(t *testing.T) {
			g, grant, key, clockClient := authorizationFixture(t)
			if _, e := g.request(context.Background(), clockClient, invite.NetworkRequestBody{Action: "grant_clock", Grant: grant}); e != nil {
				t.Fatal("clock fixture")
			}
			n := &proofNodeFixture{enrollmentNodeFixture: nodeFixture()}
			if scenario == "start_fail" {
				n.startErr = invite.ErrDenied
			}
			e := enrollment{node: n, ticket: strings.Repeat("e", 32), key: n.key, address: n.address, attempted: true}
			binds := 0
			client := requestFunc(func(ctx context.Context, b invite.NetworkRequestBody) (invite.NetworkReply, error) {
				if b.Action == "verifier" {
					c := invite.VerifierClaims{Version: 1, GrantID: grant.Claims.ID, Device: g.self, Ticket: e.ticket, NodeKey: e.key, VerifierKey: "nodekey:" + strings.Repeat("2", 64), Challenge: b.Challenge, IssuedAt: g.clock().Unix(), ExpiresAt: g.clock().Unix() + 20}
					switch scenario {
					case "old_challenge":
						c.Challenge = strings.Repeat("1", 64)
					case "wrong_ticket":
						c.Ticket = strings.Repeat("1", 32)
					case "expired":
						c.ExpiresAt = c.IssuedAt
					}
					p := invite.VerifierPermit{Claims: c, Signature: testSignature(key, "remoteapp-verifier-permit-v1\n", c)}
					if scenario == "signature" {
						p.Signature = strings.Repeat("0", 128)
					}
					r := invite.NetworkReply{Verifier: &p}
					if scenario == "mixed" {
						r.Ticket = e.ticket
					}
					return r, nil
				}
				if b.Action != "bind" || n.starts != 1 || !n.ready || b.Ticket != e.ticket || b.Grant != grant || b.Challenge != "" {
					t.Fatal("bind before trusted proof readiness")
				}
				binds++
				switch scenario {
				case "bind_fail":
					return invite.NetworkReply{}, invite.ErrDenied
				case "bind_mixed":
					return invite.NetworkReply{Ticket: e.ticket}, nil
				case "proof_fault":
					n.ready = false
				}
				return invite.NetworkReply{}, nil
			})
			r, err := e.bind(context.Background(), g, client)
			if scenario == "normal" {
				if err != nil || !r.Passed || r.Active || binds != 1 {
					t.Fatal("normal fixture binding")
				}
			} else {
				if err == nil || r.Passed {
					t.Fatal("binding failure accepted")
				}
			}
			if scenario == "signature" || scenario == "old_challenge" || scenario == "wrong_ticket" || scenario == "expired" || scenario == "mixed" {
				if n.starts != 0 || binds != 0 {
					t.Fatal("untrusted permit had side effect")
				}
			}
			if e.closeJoined() != nil {
				t.Fatal("fixture close")
			}
		})
	}
}

func TestBindingCannotUseOfflineNodeOrExpiredGrant(t *testing.T) {
	g, grant := calibratedFixture(t)
	n := nodeFixture()
	e := enrollment{node: n, key: n.key, address: n.address, ticket: strings.Repeat("e", 32), attempted: true}
	client := requestFunc(func(context.Context, invite.NetworkRequestBody) (invite.NetworkReply, error) {
		t.Fatal("unexpected HTTP")
		return invite.NetworkReply{}, nil
	})
	if _, err := e.bind(context.Background(), g, client); err == nil {
		t.Fatal("offline node proof fallback")
	}
	proof := &proofNodeFixture{enrollmentNodeFixture: n}
	e.node = proof
	g.clock = func() time.Time { return time.Unix(grant.Claims.ExpiresAt, 0) }
	if _, err := e.bind(context.Background(), g, client); err == nil {
		t.Fatal("expired grant binding")
	}
}
