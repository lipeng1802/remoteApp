package vaultbridge

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net"
	"os"
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/privatepipe"
)

func TestNativeBridgeHandler(t *testing.T) {
	exe := os.Getenv("REMOTEAPP_VAULT_TEST_EXECUTABLE")
	if exe == "" {
		t.Skip("native vault executable required")
	}
	var raw [16]byte
	if _, e := rand.Read(raw[:]); e != nil {
		t.Fatal(e)
	}
	scope := hex.EncodeToString(raw[:])
	s, e := privatepipe.NewSigner(exe, scope, "", true)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	defer func() {
		if s.DeleteCreatedIdentity() != nil {
			t.Error("native test cleanup")
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	ln, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	done := make(chan struct{})
	go func() { Serve(ctx, ln, s, scope, hex.EncodeToString(s.PublicKey())); close(done) }()
	defer func() { cancel(); <-done }()
	for i := 0; i < 2; i++ {
		c, e := net.Dial("tcp", ln.Addr().String())
		if e != nil {
			t.Fatal(e)
		}
		c.SetDeadline(time.Now().Add(3 * time.Second))
		enc := json.NewEncoder(c)
		read := privatepipe.NewReader(c).Read
		enc.Encode(map[string]string{"Scope": scope, "Expected": hex.EncodeToString(s.PublicKey())})
		var reply privatepipe.VaultReply
		if read(&reply) != nil || reply.Status != "ready" {
			t.Fatal("pinned reopen")
		}
		message := []byte("remoteapp-node-possession-v1\n{}")
		enc.Encode(map[string]string{"Op": "sign", "Message": base64.StdEncoding.EncodeToString(message)})
		if read(&reply) != nil {
			t.Fatal("signature reply")
		}
		signature, _ := hex.DecodeString(reply.Signature)
		if !ed25519.Verify(s.PublicKey(), message, signature) {
			t.Fatal("native signature invalid")
		}
		enc.Encode(map[string]string{"Op": "stop"})
		if read(&reply) != nil || reply.Status != "closed" {
			t.Fatal("stop")
		}
		c.Close()
	}
	t.Log("CHECK native signer through bridge handler and repeat pinned connection PASS; not Windows pipe acceptance")
}
