package invite

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
)

type badProvider struct{ pub ed25519.PublicKey }

func (s badProvider) PublicKey() ed25519.PublicKey { return s.pub }
func (s badProvider) Sign([]byte) ([]byte, error)  { return make([]byte, 64), nil }
func TestProviderProvisionMatchesLegacyAndRejectsWrongSignature(t *testing.T) {
	pub, key, _ := ed25519.GenerateKey(rand.Reader)
	signer, _ := NewMemorySigner(key)
	b := ProvisionBody{GrantID: "grant", NodeKey: "node", Nonce: "nonce", IssuedAt: 1}
	got, err := SignProvisionWithSigner(signer, b)
	if err != nil || got != SignProvision(key, b) {
		t.Fatal("canonical provisioning changed")
	}
	if _, err := SignProvisionWithSigner(badProvider{pub}, b); err == nil {
		t.Fatal("bad provider signature accepted")
	}
	if _, err := SignProvisionWithSigner(nil, b); err == nil {
		t.Fatal("nil provider accepted")
	}
	if _, err := CheckedSignature(signer, make([]byte, 32), []byte("message")); err == nil {
		t.Fatal("wrong public identity accepted")
	}
}
