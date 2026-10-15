package session

import (
	"context"
	"encoding/hex"
	"net"
	"time"
)

// DeliverProductSecret sends one product HMAC/fingerprint pair only after the
// controller proves possession of its device identity on the exact authorized
// grant and binding. The target signature commits to the secret hash; the
// secret itself is carried only inside the authenticated overlay connection.
func DeliverProductSecret(ctx context.Context, conn net.Conn, c Config, check Check, key, fingerprint string) error {
	defer conn.Close()
	secret := productSecret{Version: 1, Key: key, Fingerprint: fingerprint}
	if ctx == nil || !c.valid(conn, true) || !validProductSecret(secret) || !authorized(ctx, c, check) {
		return ErrDenied
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer closeOnCancel(ctx, conn)()
	conn.SetDeadline(time.Now().Add(3 * time.Second))
	var h hello
	if read(conn, &h) != nil || !validNonce(h.Nonce) {
		return ErrDenied
	}
	n, err := nonce()
	if err != nil {
		return ErrDenied
	}
	t := productSecretTranscript{1, c.Grant, n, h.Nonce, c.Binding.TargetIP.String(), c.Binding.ControllerIP.String(), productSecretHash(secret)}
	sig, err := c.productSecretSignature("product-secret-target", t, c.Binding.Target)
	if err != nil || write(conn, productSecretWelcome{t, sig}) != nil {
		return ErrDenied
	}
	var p proof
	if read(conn, &p) != nil || !verifiesProductSecret(c.Binding.Controller, "product-secret-controller", t, p.Signature) || !authorized(ctx, c, check) {
		return ErrDenied
	}
	if write(conn, secret) != nil {
		return ErrDenied
	}
	return nil
}

// ReceiveProductSecret verifies the target's signed secret hash, proves the
// controller identity, then accepts exactly one canonical 32-byte HMAC and
// SHA-256 certificate fingerprint. No peer-selected endpoint is involved.
func ReceiveProductSecret(ctx context.Context, conn net.Conn, c Config, check Check) (key, fingerprint string, err error) {
	defer conn.Close()
	if ctx == nil || !c.valid(conn, false) || !authorized(ctx, c, check) {
		return "", "", ErrDenied
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer closeOnCancel(ctx, conn)()
	conn.SetDeadline(time.Now().Add(3 * time.Second))
	n, err := nonce()
	if err != nil || write(conn, hello{n}) != nil {
		return "", "", ErrDenied
	}
	var w productSecretWelcome
	if read(conn, &w) != nil {
		return "", "", ErrDenied
	}
	t := w.Transcript
	if t.Version != 1 || t.Grant != c.Grant || t.ClientNonce != n || !validNonce(t.ServerNonce) || t.TargetIP != c.Binding.TargetIP.String() || t.ControllerIP != c.Binding.ControllerIP.String() || len(t.SecretHash) != 64 || !verifiesProductSecret(c.Binding.Target, "product-secret-target", t, w.Signature) {
		return "", "", ErrDenied
	}
	sig, err := c.productSecretSignature("product-secret-controller", t, c.Binding.Controller)
	if err != nil || write(conn, proof{sig}) != nil || !authorized(ctx, c, check) {
		return "", "", ErrDenied
	}
	var secret productSecret
	if read(conn, &secret) != nil || !validProductSecret(secret) || productSecretHash(secret) != t.SecretHash {
		return "", "", ErrDenied
	}
	// Reparse the fingerprint to make canonicality explicit at the boundary.
	if raw, e := hex.DecodeString(secret.Fingerprint); e != nil || len(raw) != 32 || hex.EncodeToString(raw) != secret.Fingerprint {
		return "", "", ErrDenied
	}
	return secret.Key, secret.Fingerprint, nil
}
