package session

import (
	"context"
	"encoding/hex"
	"net"
	"remoteapp.local/selfhost-poc/startuptrace"
	"time"
)

// DeliverProductSecret sends one product HMAC/fingerprint pair only after the
// controller proves possession of its device identity on the exact authorized
// grant and binding. The target signature commits to the secret hash; the
// secret itself is carried only inside the authenticated overlay connection.
func DeliverProductSecret(ctx context.Context, conn net.Conn, c Config, check Check, key, fingerprint string) error {
	defer conn.Close()
	secret := productSecret{Version: 1, Key: key, Fingerprint: fingerprint}
	startuptrace.Emit(ctx, startuptrace.SecretTargetConfig)
	if ctx == nil || !c.valid(conn, true) || !validProductSecret(secret) {
		return ErrDenied
	}
	startuptrace.Emit(ctx, startuptrace.SecretTargetAuthorize)
	if !authorized(ctx, c, check) {
		return ErrDenied
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer closeOnCancel(ctx, conn)()
	conn.SetDeadline(time.Now().Add(3 * time.Second))
	startuptrace.Emit(ctx, startuptrace.SecretTargetHello)
	var h hello
	if read(conn, &h) != nil || !validNonce(h.Nonce) {
		return ErrDenied
	}
	startuptrace.Emit(ctx, startuptrace.SecretTargetNonce)
	n, err := nonce()
	if err != nil {
		return ErrDenied
	}
	t := productSecretTranscript{1, c.Grant, n, h.Nonce, c.Binding.TargetIP.String(), c.Binding.ControllerIP.String(), productSecretHash(secret)}
	startuptrace.Emit(ctx, startuptrace.SecretTargetSign)
	sig, err := c.productSecretSignature("product-secret-target", t, c.Binding.Target)
	if err != nil {
		return ErrDenied
	}
	startuptrace.Emit(ctx, startuptrace.SecretTargetWelcome)
	if write(conn, productSecretWelcome{t, sig}) != nil {
		return ErrDenied
	}
	startuptrace.Emit(ctx, startuptrace.SecretTargetProof)
	var p proof
	if read(conn, &p) != nil || !verifiesProductSecret(c.Binding.Controller, "product-secret-controller", t, p.Signature) {
		return ErrDenied
	}
	startuptrace.Emit(ctx, startuptrace.SecretTargetReauthorize)
	if !authorized(ctx, c, check) {
		return ErrDenied
	}
	startuptrace.Emit(ctx, startuptrace.SecretTargetSend)
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
	startuptrace.Emit(ctx, startuptrace.SecretControllerConfig)
	if ctx == nil || !c.valid(conn, false) {
		return "", "", ErrDenied
	}
	startuptrace.Emit(ctx, startuptrace.SecretControllerAuthorize)
	if !authorized(ctx, c, check) {
		return "", "", ErrDenied
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer closeOnCancel(ctx, conn)()
	conn.SetDeadline(time.Now().Add(3 * time.Second))
	startuptrace.Emit(ctx, startuptrace.SecretControllerHello)
	n, err := nonce()
	if err != nil || write(conn, hello{n}) != nil {
		return "", "", ErrDenied
	}
	startuptrace.Emit(ctx, startuptrace.SecretControllerWelcome)
	var w productSecretWelcome
	if read(conn, &w) != nil {
		return "", "", ErrDenied
	}
	startuptrace.Emit(ctx, startuptrace.SecretControllerVerify)
	t := w.Transcript
	if t.Version != 1 || t.Grant != c.Grant || t.ClientNonce != n || !validNonce(t.ServerNonce) || t.TargetIP != c.Binding.TargetIP.String() || t.ControllerIP != c.Binding.ControllerIP.String() || len(t.SecretHash) != 64 || !verifiesProductSecret(c.Binding.Target, "product-secret-target", t, w.Signature) {
		return "", "", ErrDenied
	}
	startuptrace.Emit(ctx, startuptrace.SecretControllerSign)
	sig, err := c.productSecretSignature("product-secret-controller", t, c.Binding.Controller)
	if err != nil {
		return "", "", ErrDenied
	}
	startuptrace.Emit(ctx, startuptrace.SecretControllerProof)
	if write(conn, proof{sig}) != nil {
		return "", "", ErrDenied
	}
	startuptrace.Emit(ctx, startuptrace.SecretControllerReauthorize)
	if !authorized(ctx, c, check) {
		return "", "", ErrDenied
	}
	startuptrace.Emit(ctx, startuptrace.SecretControllerReceive)
	var secret productSecret
	if read(conn, &secret) != nil || !validProductSecret(secret) || productSecretHash(secret) != t.SecretHash {
		return "", "", ErrDenied
	}
	startuptrace.Emit(ctx, startuptrace.SecretControllerCanonical)
	// Reparse the fingerprint to make canonicality explicit at the boundary.
	if raw, e := hex.DecodeString(secret.Fingerprint); e != nil || len(raw) != 32 || hex.EncodeToString(raw) != secret.Fingerprint {
		return "", "", ErrDenied
	}
	return secret.Key, secret.Fingerprint, nil
}
