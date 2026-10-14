package devicevalidation

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"testing"
	"time"
)

func TestAuthorityMaterialSeparatesTLSAndServerIssuer(t *testing.T) {
	pub, _, e := ed25519.GenerateKey(rand.Reader)
	if e != nil {
		t.Fatal("test key")
	}
	spec, fixture, e := NewAuthorityMaterial([]byte("public helper fixture"), time.Now(), pub)
	if e != nil || spec.Bundled.Claims.Issuer != hex.EncodeToString(pub) {
		t.Fatal("fixed server issuer")
	}
	pair, e := tls.X509KeyPair([]byte(fixture.Certificate), []byte(fixture.Key))
	if e != nil {
		t.Fatal("TLS fixture")
	}
	leaf := pair.PrivateKey.(ed25519.PrivateKey).Public().(ed25519.PublicKey)
	if hex.EncodeToString(leaf) == spec.Bundled.Claims.Issuer {
		t.Fatal("TLS key reused as issuer")
	}
	if _, _, e = NewAuthorityMaterial([]byte("helper"), time.Now(), nil); e == nil {
		t.Fatal("missing issuer accepted")
	}
}
