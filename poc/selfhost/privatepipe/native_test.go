package privatepipe_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http/httptest"
	"os"
	"path/filepath"
	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/privatepipe"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestNativeVaultPrivateSignerHTTPSAndRestart(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("authority requires Unix private Store; run Mac HTTPS plus Windows native identity test")
	}
	exe := os.Getenv("REMOTEAPP_VAULT_TEST_EXECUTABLE")
	if exe == "" {
		t.Skip("native executable required; no memory/plaintext substitute")
	}
	var raw [16]byte
	rand.Read(raw[:])
	scope := hex.EncodeToString(raw[:])
	s, e := privatepipe.NewSigner(exe, scope, "", true)
	if e != nil {
		t.Fatal("native vault creation unavailable:", e)
	}
	defer s.Close()
	defer s.DeleteCreatedIdentity()
	pub := hex.EncodeToString(s.PublicKey())
	store, e := invite.Open(filepath.Join(t.TempDir(), "authority"), time.Now)
	if e != nil {
		t.Fatal(e)
	}
	defer store.Close()
	server := httptest.NewTLSServer(invite.NewAdmission().Wrap(invite.NetworkHandler(&invite.ProvisionBackend{Store: store})))
	defer server.Close()
	c, e := invite.NewNetworkClientWithSigner(server.URL, server.Certificate().Raw, s)
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	r, e := c.DeviceRequest(context.Background(), invite.Body{Action: "register"})
	if e != nil {
		t.Fatal("native signer HTTP registration")
	}
	reopened, e := privatepipe.NewSigner(exe, scope, pub, false)
	if e != nil {
		t.Fatal("identity reopen")
	}
	defer reopened.Close()
	c2, e := invite.NewNetworkClientWithSigner(server.URL, server.Certificate().Raw, reopened)
	if e != nil {
		t.Fatal(e)
	}
	defer c2.Close()
	r2, e := c2.DeviceRequest(context.Background(), invite.Body{Action: "register"})
	if e != nil || r2.DeviceID != r.DeviceID || r2.DeviceCode != r.DeviceCode {
		t.Fatal("stable native identity")
	}
	var other [16]byte
	rand.Read(other[:])
	if missing, e := privatepipe.NewSigner(exe, hex.EncodeToString(other[:]), pub, false); e == nil {
		missing.Close()
		t.Fatal("silent missing identity regeneration")
	}
	if wrong, e := privatepipe.NewSigner(exe, scope, string(make([]byte, 64)), false); e == nil {
		wrong.Close()
		t.Fatal("wrong expected identity accepted")
	}
	if _, e := reopened.Sign([]byte("arbitrary message")); e == nil {
		t.Fatal("cross-domain signing")
	}
}

func TestNativeVaultIdentityAndIsolation(t *testing.T) {
	exe := os.Getenv("REMOTEAPP_VAULT_TEST_EXECUTABLE")
	if exe == "" {
		t.Skip("native vault executable required")
	}
	var raw [16]byte
	rand.Read(raw[:])
	scope := hex.EncodeToString(raw[:])
	s, e := privatepipe.NewSigner(exe, scope, "", true)
	if e != nil {
		t.Fatal("native vault creation unavailable:", e)
	}
	defer s.Close()
	defer func() {
		if s.DeleteCreatedIdentity() != nil {
			t.Error("test identity cleanup failed")
		}
	}()
	message := []byte("remoteapp-invite-request-v1\n{}")
	if _, e = s.Sign(message); e != nil {
		t.Fatal("native signing")
	}
	pub := hex.EncodeToString(s.PublicKey())
	next, e := privatepipe.NewSigner(exe, scope, pub, false)
	if e != nil {
		t.Fatal("native reopen")
	}
	defer next.Close()
	if _, e = next.Sign(message); e != nil {
		t.Fatal("reopened signature")
	}
	for _, domain := range []string{"remoteapp-provision-intent-v1", "remoteapp-node-possession-v1", "remoteapp-session-target-v1", "remoteapp-session-controller-v1", "remoteapp-session-product-tls-target-v1", "remoteapp-session-product-tls-controller-v1"} {
		if _, e = next.Sign([]byte(domain + "\n{}")); e != nil {
			t.Fatal("native helper domain", domain)
		}
	}
	var other [16]byte
	rand.Read(other[:])
	if missing, e := privatepipe.NewSigner(exe, hex.EncodeToString(other[:]), pub, false); e == nil {
		missing.Close()
		t.Fatal("missing identity regenerated")
	}
	if wrong, e := privatepipe.NewSigner(exe, scope, strings.Repeat("0", 64), false); e == nil {
		wrong.Close()
		t.Fatal("wrong public identity")
	}
}

func TestNativeVaultStrictJoinedReopen(t *testing.T) {
	exe := os.Getenv("REMOTEAPP_VAULT_TEST_EXECUTABLE")
	if exe == "" {
		t.Skip("native executable required; no fixture substitution")
	}
	var raw [16]byte
	if _, e := rand.Read(raw[:]); e != nil {
		t.Fatal(e)
	}
	owner, e := privatepipe.NewSigner(exe, hex.EncodeToString(raw[:]), "", true)
	if e != nil {
		t.Fatal("test identity unavailable", e)
	}
	defer owner.Close()
	defer func() {
		if e := owner.DeleteCreatedIdentity(); e != nil {
			t.Error("exact test identity cleanup unconfirmed", e)
		}
	}()
	reader, e := privatepipe.NewSigner(exe, hex.EncodeToString(raw[:]), hex.EncodeToString(owner.PublicKey()), false)
	if e != nil {
		t.Fatal(e)
	}
	defer reader.CloseJoined()
	if _, e := reader.Sign([]byte("remoteapp-invite-request-v1\n{}")); e != nil {
		t.Fatal(e)
	}
	if e := reader.CloseJoined(); e != nil {
		t.Fatal("native process not strictly joined", e)
	}
	if e := reader.CloseJoined(); e != nil {
		t.Fatal("native join not sticky", e)
	}
}
