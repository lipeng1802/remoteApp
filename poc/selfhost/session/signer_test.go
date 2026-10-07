package session

import (
	"crypto/ed25519"
	"errors"
	"remoteapp.local/selfhost-poc/invite"
	"testing"
)

type failingSigner struct {
	pub   ed25519.PublicKey
	wrong bool
}

func (s failingSigner) PublicKey() ed25519.PublicKey { return s.pub }
func (s failingSigner) Sign([]byte) ([]byte, error) {
	if s.wrong {
		return make([]byte, 64), nil
	}
	return nil, errors.New("unavailable")
}

func TestProviderSessionAndRevocation(t *testing.T) {
	f := setup(t)
	f.server.Signer, _ = invite.NewMemorySigner(f.owner)
	f.server.Key = nil
	f.probe.Signer, _ = invite.NewMemorySigner(f.client)
	f.probe.Key = nil
	r := f.run(f.check)
	tick(t, r)
	f.execute(f.owner, invite.Body{Action: "revoke", GrantID: f.server.Grant.Claims.ID})
	ended(t, r)
}

func TestProviderFailureRejectsEveryProofDomain(t *testing.T) {
	f := setup(t)
	for _, role := range []string{"target", "controller", "product-tls-target", "product-tls-controller"} {
		c := f.server
		pub := c.Binding.Target
		if role == "controller" || role == "product-tls-controller" {
			pub = c.Binding.Controller
		}
		c.Key = nil
		for _, wrong := range []bool{false, true} {
			c.Signer = failingSigner{pub, wrong}
			if _, err := c.signature(role, transcript{}); err == nil {
				t.Fatal("provider failure accepted", role)
			}
		}
	}
	c := f.server
	c.Signer, _ = invite.NewMemorySigner(f.owner)
	if _, err := c.provider(); err == nil {
		t.Fatal("mixed private key/provider accepted")
	}
	c.Key = nil
	c.Signer, _ = invite.NewMemorySigner(f.other)
	if _, err := c.signature("target", transcript{}); err == nil {
		t.Fatal("wrong identity accepted")
	}
}
