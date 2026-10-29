package invite

import (
	"context"
	"crypto/ed25519"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNodeRetirementRealHTTPSRequiresRevokedAndActuallyCleanedProvision(t *testing.T) {
	f, g, b, _ := provisionFixture(t)
	bindPair(t, f, g, b)
	server := httptest.NewTLSServer(NetworkHandler(b))
	defer server.Close()
	client, err := NewNetworkClient(server.URL, server.Certificate().Raw, f.owner)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	client.now = func() time.Time { return f.now }
	body := Body{Action: "node_retirement", NodeKey: "nodekey:" + strings.Repeat("a", 64), Challenge: strings.Repeat("c", 64)}
	if _, err = client.DeviceRequest(context.Background(), body); err == nil {
		t.Fatal("live grant accepted for cleanup")
	}
	f.exec(t, f.owner, Body{Action: "revoke", GrantID: g.Claims.ID})
	if _, err = client.DeviceRequest(context.Background(), body); err == nil {
		t.Fatal("revoked but uncleaned node accepted")
	}
	if b.Reconcile(context.Background()) != nil {
		t.Fatal("actual cleanup")
	}
	r, err := client.DeviceRequest(context.Background(), body)
	if err != nil || r.Status != "node_retired" || r.Retirement == nil || !VerifyNodeRetirement(f.s.PublicKey(), *r.Retirement, deviceID(f.owner.Public().(ed25519.PublicKey)), body.NodeKey, "100.120.0.1", body.Challenge, f.now) {
		t.Fatal("issuer cleanup proof", err)
	}
}

func TestNodeRetirementOwnershipUnknownCleanupAndTemporalFailures(t *testing.T) {
	for _, mode := range []string{"foreign", "other_node", "not_registered", "expired_only", "cleanup_failure", "extra", "rollback", "bad_signature"} {
		t.Run(mode, func(t *testing.T) {
			f, g, b, d := provisionFixture(t)
			bindPair(t, f, g, b)
			if mode != "expired_only" {
				f.exec(t, f.owner, Body{Action: "revoke", GrantID: g.Claims.ID})
			} else {
				f.now = f.now.Add(301 * time.Second)
			}
			if mode == "cleanup_failure" {
				d.failCleanup = true
			}
			b.Reconcile(context.Background())
			key := f.owner
			if mode == "foreign" {
				key = f.client
			}
			if mode == "not_registered" {
				key = f.other
			}
			body := Body{Action: "node_retirement", NodeKey: "nodekey:" + strings.Repeat("a", 64), Challenge: strings.Repeat("c", 64), IssuedAt: f.now.Unix(), Nonce: strings.Repeat("e", 32)}
			if mode == "other_node" {
				body.NodeKey = "nodekey:" + strings.Repeat("f", 64)
			}
			if mode == "extra" {
				body.GrantID = g.Claims.ID
			}
			if mode == "rollback" {
				f.now = f.now.Add(-time.Second)
				body.IssuedAt = f.now.Unix()
			}
			e := Sign(key, body)
			if mode == "bad_signature" {
				e.Signature = strings.Repeat("0", 128)
			}
			if r, err := f.s.Execute(e); err == nil || r.Retirement != nil {
				t.Fatal("unsafe retirement proof")
			}
		})
	}
}

func TestNodeRetirementProofReplayBindingAndExpiry(t *testing.T) {
	f, g, b, _ := provisionFixture(t)
	bindPair(t, f, g, b)
	f.exec(t, f.owner, Body{Action: "revoke", GrantID: g.Claims.ID})
	if b.Reconcile(context.Background()) != nil {
		t.Fatal("cleanup")
	}
	body := Body{Action: "node_retirement", NodeKey: "nodekey:" + strings.Repeat("a", 64), Challenge: strings.Repeat("c", 64), IssuedAt: f.now.Unix(), Nonce: strings.Repeat("e", 32)}
	envelope := Sign(f.owner, body)
	r, err := f.s.Execute(envelope)
	if err != nil || r.Retirement == nil {
		t.Fatal("proof")
	}
	if _, err = f.s.Execute(envelope); err == nil {
		t.Fatal("request replay")
	}
	p := *r.Retirement
	pub := f.owner.Public().(ed25519.PublicKey)
	for _, mode := range []string{"valid", "device", "node", "address", "challenge", "expired", "future", "signature"} {
		t.Run(mode, func(t *testing.T) {
			device, node, address, challenge := deviceID(pub), body.NodeKey, "100.120.0.1", body.Challenge
			now := f.now
			proof := p
			switch mode {
			case "device":
				device = strings.Repeat("0", 64)
			case "node":
				node = "nodekey:" + strings.Repeat("f", 64)
			case "address":
				address = "100.120.0.2"
			case "challenge":
				challenge = strings.Repeat("f", 64)
			case "expired":
				now = now.Add(3 * time.Second)
			case "future":
				now = now.Add(-time.Nanosecond)
			case "signature":
				proof.Signature = strings.Repeat("0", 128)
			}
			if VerifyNodeRetirement(f.s.PublicKey(), proof, device, node, address, challenge, now) != (mode == "valid") {
				t.Fatal("proof binding or validity failed")
			}
		})
	}
}
