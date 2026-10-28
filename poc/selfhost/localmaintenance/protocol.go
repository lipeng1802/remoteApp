// Package localmaintenance implements TEST-only, device-key authenticated
// maintenance between independent local processes. It does not authenticate a
// publisher or authorize a remote network peer, reset, path, or identity choice.
package localmaintenance

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"remoteapp.local/selfhost-poc/trustinstall"
	"strconv"
)

var ErrDenied = errors.New("local_maintenance_denied")

type Signer interface {
	PublicKey() ed25519.PublicKey
	Sign([]byte) ([]byte, error)
}

type frame struct {
	Op          string `json:"op"`
	Version     int    `json:"version"`
	Scope       string `json:"scope"`
	Release     string `json:"release_sha256"`
	Instance    string `json:"instance,omitempty"`
	ClientNonce string `json:"client_nonce"`
	ServerNonce string `json:"server_nonce,omitempty"`
	Epoch       uint64 `json:"epoch,omitempty"`
	Transaction string `json:"transaction,omitempty"`
	Signature   string `json:"signature,omitempty"`
}

func releaseDigest(r trustinstall.Release) string {
	raw, _ := json.Marshal(r.Specification())
	h := sha256.Sum256(append([]byte("remoteapp-local-maintenance-release-v1\n"), raw...))
	return hex.EncodeToString(h[:])
}

func canonical(value string, size int) bool {
	raw, e := hex.DecodeString(value)
	return e == nil && len(raw) == size && hex.EncodeToString(raw) == value
}
func random(size int) (string, error) {
	raw := make([]byte, size)
	if _, e := rand.Read(raw); e != nil {
		return "", ErrDenied
	}
	return hex.EncodeToString(raw), nil
}

// Domain separation prevents reflecting a server proof as a client request or
// reusing a ready/finish signature as a fresh authorization. The native vault
// only accepts these exact domains followed by a canonical SHA-256 hex digest.
func signingMessage(f frame) ([]byte, error) {
	switch f.Op {
	case "challenge", "authenticate", "ready", "finish", "closed":
	default:
		return nil, ErrDenied
	}
	f.Signature = ""
	raw, e := json.Marshal(f)
	if e != nil {
		return nil, ErrDenied
	}
	h := sha256.Sum256(raw)
	return []byte("remoteapp-local-maintenance-" + f.Op + "-v1\n" + hex.EncodeToString(h[:])), nil
}
func sign(f frame, s Signer) (frame, error) {
	m, e := signingMessage(f)
	if e != nil || s == nil {
		return frame{}, ErrDenied
	}
	sig, e := s.Sign(m)
	if e != nil || !ed25519.Verify(s.PublicKey(), m, sig) {
		return frame{}, ErrDenied
	}
	f.Signature = hex.EncodeToString(sig)
	return f, nil
}
func verify(got, expected frame, public ed25519.PublicKey) bool {
	if !canonical(got.Signature, 64) {
		return false
	}
	sig, _ := hex.DecodeString(got.Signature)
	got.Signature = ""
	expected.Signature = ""
	if got != expected {
		return false
	}
	m, e := signingMessage(got)
	return e == nil && ed25519.Verify(public, m, sig)
}

func localAddress(address string) bool {
	host, port, e := net.SplitHostPort(address)
	n, portErr := strconv.Atoi(port)
	if e != nil || portErr != nil || n < 1 || n > 65535 || strconv.Itoa(n) != port {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
