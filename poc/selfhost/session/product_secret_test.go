package session

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"strings"
	"testing"
	"time"
)

const productSecretTestKey = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="

func TestProductSecretMutualProofAndHashBinding(t *testing.T) {
	f := portableSetup(t)
	server, controller := sockets(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- DeliverProductSecret(ctx, server, f.server, f.portableCheck, productSecretTestKey, strings.Repeat("c", 64))
	}()
	key, fingerprint, err := ReceiveProductSecret(ctx, controller, f.probe, f.portableCheck)
	if err != nil || key != productSecretTestKey || fingerprint != strings.Repeat("c", 64) {
		t.Fatal("verified secret delivery", err)
	}
	if err := <-done; err != nil {
		t.Fatal("target delivery", err)
	}
}

func TestProductSecretRejectsWrongRoleAndMalformedMaterial(t *testing.T) {
	for _, mode := range []string{"wrong_identity", "bad_key", "bad_fingerprint"} {
		t.Run(mode, func(t *testing.T) {
			f := portableSetup(t)
			server, controller := sockets(t)
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			key, fingerprint := productSecretTestKey, strings.Repeat("c", 64)
			probe := f.probe
			switch mode {
			case "wrong_identity":
				_, probe.Key, _ = ed25519.GenerateKey(rand.Reader)
			case "bad_key":
				key = strings.Repeat("a", 44)
			case "bad_fingerprint":
				fingerprint = strings.Repeat("z", 64)
			}
			done := make(chan error, 1)
			go func() { done <- DeliverProductSecret(ctx, server, f.server, f.portableCheck, key, fingerprint) }()
			gotKey, gotFingerprint, err := ReceiveProductSecret(ctx, controller, probe, f.portableCheck)
			if err == nil || gotKey != "" || gotFingerprint != "" {
				t.Fatal("invalid product secret accepted")
			}
			if err := <-done; err == nil {
				t.Fatal("invalid delivery accepted by target")
			}
		})
	}
}

func TestProductSecretRejectsValueThatDoesNotMatchSignedHash(t *testing.T) {
	f := portableSetup(t)
	server, controller := sockets(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		defer server.Close()
		var h hello
		if read(server, &h) != nil {
			done <- ErrDenied
			return
		}
		n, _ := nonce()
		signedSecret := productSecret{Version: 1, Key: productSecretTestKey, Fingerprint: strings.Repeat("c", 64)}
		transcript := productSecretTranscript{1, f.server.Grant, n, h.Nonce, f.server.Binding.TargetIP.String(), f.server.Binding.ControllerIP.String(), productSecretHash(signedSecret)}
		sig, _ := f.server.productSecretSignature("product-secret-target", transcript, f.server.Binding.Target)
		if write(server, productSecretWelcome{transcript, sig}) != nil {
			done <- ErrDenied
			return
		}
		var p proof
		if read(server, &p) != nil {
			done <- ErrDenied
			return
		}
		tampered := signedSecret
		tampered.Fingerprint = strings.Repeat("d", 64)
		done <- write(server, tampered)
	}()
	key, fingerprint, err := ReceiveProductSecret(ctx, controller, f.probe, f.portableCheck)
	if err == nil || key != "" || fingerprint != "" {
		t.Fatal("secret hash mismatch accepted")
	}
	if err := <-done; err != nil {
		t.Fatal("malicious fixture did not complete", err)
	}
}
