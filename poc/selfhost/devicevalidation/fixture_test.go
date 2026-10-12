package devicevalidation

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/trustinstall"
)

func TestFixtureVerifiesSignatureIdentityNonceAndTime(t *testing.T) {
	pub, key, _ := ed25519.GenerateKey(rand.Reader)
	v := &verifier{public: pub, nonces: make(map[string]bool)}
	post := func(env invite.Envelope) int {
		raw, _ := json.Marshal(env)
		r := httptest.NewRequest("POST", "https://127.0.0.1/v1/request", bytes.NewReader(raw))
		r.Header.Set("Content-Type", "application/json")
		r.TLS = &tls.ConnectionState{Version: tls.VersionTLS13}
		w := httptest.NewRecorder()
		v.ServeHTTP(w, r)
		return w.Code
	}
	valid := invite.Sign(key, invite.Body{Action: "register", Nonce: "0123456789abcdef0123456789abcdef", IssuedAt: time.Now().Unix()})
	if post(valid) != 200 || v.registers != 1 {
		t.Fatal("valid signature rejected")
	}
	if post(valid) != 403 {
		t.Fatal("replay accepted")
	}
	for _, mode := range []string{"signature", "identity", "nonce", "time", "action"} {
		var raw [16]byte
		rand.Read(raw[:])
		body := invite.Body{Action: "invite", Nonce: hex.EncodeToString(raw[:]), IssuedAt: time.Now().Unix()}
		if mode == "time" {
			body.IssuedAt -= 60
		}
		if mode == "nonce" {
			body.Nonce = "bad"
		}
		if mode == "action" {
			body.Action = "approve"
		}
		env := invite.Sign(key, body)
		if mode == "signature" {
			env.Signature = "bad"
		}
		if mode == "identity" {
			_, other, _ := ed25519.GenerateKey(rand.Reader)
			env = invite.Sign(other, body)
		}
		if post(env) != 403 {
			t.Fatal("negative signature fixture accepted", mode)
		}
	}
	if v.registers != 1 || v.invites != 0 {
		t.Fatal("invalid requests changed evidence counts")
	}
}

func TestMaterialIsTestOnlyAndTLSMatchesCompiledProfile(t *testing.T) {
	spec, fixture, err := NewMaterial([]byte("journal-fixture"), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := trustinstall.EncodeForTrustedBuild(spec, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	release, err := trustinstall.DecodeCompiled(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = fixturePair(release, fixture); err != nil {
		t.Fatal("TLS fixture mismatches profile")
	}
	spec.Test = false
	encoded, err = trustinstall.EncodeForTrustedBuild(spec, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	release, err = trustinstall.DecodeCompiled(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = fixturePair(release, fixture); err == nil {
		t.Fatal("production namespace accepted by test validator")
	}
}
