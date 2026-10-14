package registrationvalidation

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/invite"
)

func TestAuthorityFixtureChecksNativeSignatureReplayAndInternalIntent(t *testing.T) {
	pub, k, _ := ed25519.GenerateKey(rand.Reader)
	_, issuer, _ := ed25519.GenerateKey(rand.Reader)
	g := invite.Grant{Claims: invite.Claims{ID: strings.Repeat("a", 32)}}
	a := &authority{issuer: issuer, public: pub, grant: g, secret: "fixture-secret", nonces: map[string]bool{}}
	post := func(e invite.NetworkRequest) (int, invite.NetworkReply) {
		b, _ := json.Marshal(e)
		r := httptest.NewRequest("POST", "https://127.0.0.1/v1/network", bytes.NewReader(b))
		r.TLS = &tls.ConnectionState{}
		w := httptest.NewRecorder()
		a.ServeHTTP(w, r)
		var reply invite.NetworkReply
		json.Unmarshal(w.Body.Bytes(), &reply)
		return w.Code, reply
	}
	body := invite.NetworkRequestBody{Action: "grant_clock", Grant: g, Nonce: strings.Repeat("1", 32), IssuedAt: time.Now().Unix(), Challenge: strings.Repeat("2", 64)}
	e := invite.NetworkRequest{PublicKey: hex.EncodeToString(pub), Body: body}
	e.Signature = sign(k, "remoteapp-network-api-v1\nPOST /v1/network\n", body)
	code, r := post(e)
	if code != 200 || r.Proof == nil {
		t.Fatal("valid signature rejected")
	}
	if _, err := invite.AuthorityClock(issuer.Public().(ed25519.PublicKey), *r.Proof, g.Claims.ID, body.Challenge, time.Now()); err != nil {
		t.Fatal("fixture issuer proof invalid")
	}
	if code, _ := post(e); code != 403 {
		t.Fatal("nonce replay accepted")
	}
	body.Nonce = strings.Repeat("3", 32)
	e.Body = body
	e.Signature = strings.Repeat("0", 128)
	if code, _ := post(e); code != 403 {
		t.Fatal("bad signature accepted")
	}
	signer, _ := invite.NewMemorySigner(k)
	intent, err := invite.SignProvisionWithSigner(signer, invite.ProvisionBody{GrantID: g.Claims.ID, NodeKey: "nodekey:" + strings.Repeat("4", 64), Nonce: strings.Repeat("5", 32), IssuedAt: time.Now().Unix()})
	if err != nil {
		t.Fatal(err)
	}
	body = invite.NetworkRequestBody{Action: "enroll", Grant: g, Intent: intent, Nonce: strings.Repeat("6", 32), IssuedAt: time.Now().Unix()}
	e.Body = body
	e.Signature = sign(k, "remoteapp-network-api-v1\nPOST /v1/network\n", body)
	if code, r := post(e); code != 200 || r.Credential == nil || r.Credential.Secret != "fixture-secret" || a.mints != 1 {
		t.Fatal("signed intent refused")
	}
	body.Nonce = strings.Repeat("7", 32)
	body.Intent.Signature = strings.Repeat("0", 128)
	e.Body = body
	e.Signature = sign(k, "remoteapp-network-api-v1\nPOST /v1/network\n", body)
	if code, _ := post(e); code != 403 || a.mints != 1 {
		t.Fatal("invalid intent minted")
	}
	body.Nonce = strings.Repeat("8", 32)
	body.Intent = intent
	e.Body = body
	e.Signature = sign(k, "remoteapp-network-api-v1\nPOST /v1/network\n", body)
	a.unknown = true
	if code, r := post(e); code != 503 || r.Credential != nil || a.mints != 2 {
		t.Fatal("unknown fixture exported credential")
	}
}
