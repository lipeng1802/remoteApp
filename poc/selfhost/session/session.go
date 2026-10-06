// Package session gates an isolated fixed-payload stream. It is not a product
// transport: the caller must supply an encrypted connection and trusted bindings.
package session

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/netip"
	"time"

	"remoteapp.local/selfhost-poc/invite"
)

var ErrDenied = errors.New("session_denied")

const Payload = "remoteapp-authorized-poc-v1"
const checkTimeout = 500 * time.Millisecond
const checkInterval = 250 * time.Millisecond

// Binding comes from a trusted provisioning path, never from the peer's hello.
// A valid address alone is not proof of Headscale node ownership.
type Binding struct {
	Target, Controller     ed25519.PublicKey
	TargetIP, ControllerIP netip.Addr
}

// Check must honor ctx and consult current durable state, not just VerifyGrant.
type Check func(ctx context.Context, g invite.Grant, target, controller string) bool

type Config struct {
	Issuer  ed25519.PublicKey
	Key     ed25519.PrivateKey
	Binding Binding
	Grant   invite.Grant
	Now     func() time.Time
}

type transcript struct {
	Version      int          `json:"version"`
	Grant        invite.Grant `json:"grant"`
	ServerNonce  string       `json:"server_nonce"`
	ClientNonce  string       `json:"client_nonce"`
	TargetIP     string       `json:"target_ip"`
	ControllerIP string       `json:"controller_ip"`
}
type hello struct {
	Nonce string `json:"nonce"`
}
type welcome struct {
	Transcript transcript `json:"transcript"`
	Signature  string     `json:"signature"`
}
type proof struct {
	Signature string `json:"signature"`
}

func Identity(pub ed25519.PublicKey) string {
	h := sha256.Sum256(pub)
	return hex.EncodeToString(h[:])
}
func nonce() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	return hex.EncodeToString(b), err
}
func validNonce(s string) bool {
	b, err := hex.DecodeString(s)
	return err == nil && len(b) == 32 && hex.EncodeToString(b) == s
}
func bytesFor(role string, t transcript) []byte {
	b, _ := json.Marshal(t)
	return append([]byte("remoteapp-session-"+role+"-v1\n"), b...)
}
func signed(key ed25519.PrivateKey, role string, t transcript) string {
	return hex.EncodeToString(ed25519.Sign(key, bytesFor(role, t)))
}
func verifies(pub ed25519.PublicKey, role string, t transcript, sig string) bool {
	b, err := hex.DecodeString(sig)
	return err == nil && len(b) == 64 && ed25519.Verify(pub, bytesFor(role, t), b)
}
func (c Config) valid(conn net.Conn, server bool) bool {
	b := c.Binding
	if c.Now == nil || len(c.Issuer) != 32 || len(c.Key) != 64 || len(b.Target) != 32 || len(b.Controller) != 32 || !b.TargetIP.IsValid() || !b.ControllerIP.IsValid() || b.TargetIP.Is4In6() || b.ControllerIP.Is4In6() {
		return false
	}
	if !bytes.Equal(c.Key, ed25519.NewKeyFromSeed(c.Key[:32])) {
		return false
	}
	local, remote := b.TargetIP, b.ControllerIP
	pub := b.Target
	if !server {
		local, remote, pub = remote, local, b.Controller
	}
	// Check both actual endpoints before reading anything from the peer.
	l, e1 := netip.ParseAddrPort(conn.LocalAddr().String())
	r, e2 := netip.ParseAddrPort(conn.RemoteAddr().String())
	return e1 == nil && e2 == nil && l.Addr().Unmap() == local && r.Addr().Unmap() == remote &&
		Identity(c.Key.Public().(ed25519.PublicKey)) == Identity(pub) &&
		invite.VerifyGrant(c.Issuer, c.Grant, Identity(b.Target), Identity(b.Controller), c.Now())
}

// Framing is bounded before allocation; strict decoding rejects extra fields and
// trailing JSON. No errors contain keys, grants, peer addresses or payload data.
func read(conn net.Conn, value any) error {
	var header [2]byte
	if _, err := io.ReadFull(conn, header[:]); err != nil {
		return ErrDenied
	}
	n := binary.BigEndian.Uint16(header[:])
	if n == 0 || n > 2048 {
		return ErrDenied
	}
	b := make([]byte, n)
	if _, err := io.ReadFull(conn, b); err != nil {
		return ErrDenied
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(value) != nil {
		return ErrDenied
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return ErrDenied
	}
	return nil
}
func write(conn net.Conn, value any) error {
	b, err := json.Marshal(value)
	if err != nil || len(b) > 2048 {
		return ErrDenied
	}
	var h [2]byte
	binary.BigEndian.PutUint16(h[:], uint16(len(b)))
	if _, err = io.Copy(conn, bytes.NewReader(append(h[:], b...))); err != nil {
		return ErrDenied
	}
	return nil
}

// Each check has a hard local timeout even if a remote authority stalls. The
// callback must observe cancellation so its worker does not leak on timeout.
func authorized(ctx context.Context, c Config, check Check) bool {
	if check == nil || !invite.VerifyGrant(c.Issuer, c.Grant, Identity(c.Binding.Target), Identity(c.Binding.Controller), c.Now()) {
		return false
	}
	ctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()
	result := make(chan bool, 1)
	go func() { result <- check(ctx, c.Grant, Identity(c.Binding.Target), Identity(c.Binding.Controller)) }()
	select {
	case ok := <-result:
		return ok && ctx.Err() == nil && invite.VerifyGrant(c.Issuer, c.Grant, Identity(c.Binding.Target), Identity(c.Binding.Controller), c.Now())
	case <-ctx.Done():
		return false
	}
}
func closeOnCancel(ctx context.Context, conn net.Conn) func() {
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			conn.Close()
		case <-done:
		}
	}()
	return func() { close(done) }
}

// Serve emits no payload until mutual possession proof AND an online grant
// check succeed. Polling closes idle/active streams on expiry, revoke or outage.
func Serve(ctx context.Context, conn net.Conn, c Config, check Check) error {
	defer conn.Close()
	if !c.valid(conn, true) {
		return ErrDenied
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer closeOnCancel(ctx, conn)()
	if !authorized(ctx, c, check) {
		return ErrDenied
	}
	conn.SetDeadline(time.Now().Add(3 * time.Second))
	var h hello
	if read(conn, &h) != nil || !validNonce(h.Nonce) {
		return ErrDenied
	}
	n, err := nonce()
	if err != nil {
		return ErrDenied
	}
	t := transcript{1, c.Grant, n, h.Nonce, c.Binding.TargetIP.String(), c.Binding.ControllerIP.String()}
	if write(conn, welcome{t, signed(c.Key, "target", t)}) != nil {
		return ErrDenied
	}
	var p proof
	if read(conn, &p) != nil || !verifies(c.Binding.Controller, "controller", t, p.Signature) {
		return ErrDenied
	}
	go func() {
		tick := time.NewTicker(checkInterval)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				if !authorized(ctx, c, check) {
					cancel()
					return
				}
			}
		}
	}()
	for {
		conn.SetDeadline(time.Now().Add(3 * time.Second))
		var request [1]byte
		if _, err := io.ReadFull(conn, request[:]); err != nil || request[0] != 1 {
			return ErrDenied
		}
		// Recheck just before every response, not only when the socket opens.
		if !authorized(ctx, c, check) || ctx.Err() != nil {
			return ErrDenied
		}
		if _, err := io.WriteString(conn, Payload); err != nil {
			return ErrDenied
		}
	}
}

// Probe verifies the target's key and the exact grant/transcript, then requests
// only the fixed payload. The caller may cancel after any observed response.
func Probe(ctx context.Context, conn net.Conn, c Config, received func()) error {
	return probe(ctx, conn, c, received, false)
}

// RejectProof is an explicit isolated negative test: a fresh third private key
// signs the actual server challenge on an otherwise approved network node.
// Success requires EOF before any payload, not a timeout or arbitrary I/O error.
func RejectProof(ctx context.Context, conn net.Conn, c Config) error {
	return probe(ctx, conn, c, nil, true)
}
func probe(ctx context.Context, conn net.Conn, c Config, received func(), wrongProof bool) error {
	defer conn.Close()
	if !c.valid(conn, false) {
		return ErrDenied
	}
	defer closeOnCancel(ctx, conn)()
	conn.SetDeadline(time.Now().Add(3 * time.Second))
	n, err := nonce()
	if err != nil || write(conn, hello{n}) != nil {
		return ErrDenied
	}
	var w welcome
	if read(conn, &w) != nil {
		return ErrDenied
	}
	t := w.Transcript
	if t.Version != 1 || t.Grant != c.Grant || t.ClientNonce != n || !validNonce(t.ServerNonce) || t.TargetIP != c.Binding.TargetIP.String() || t.ControllerIP != c.Binding.ControllerIP.String() || !verifies(c.Binding.Target, "target", t, w.Signature) {
		return ErrDenied
	}
	key := c.Key
	if wrongProof {
		_, key, err = ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return ErrDenied
		}
	}
	if write(conn, proof{signed(key, "controller", t)}) != nil {
		return ErrDenied
	}
	if wrongProof {
		var b [1]byte
		n, err := conn.Read(b[:])
		if n == 0 && err == io.EOF {
			return nil
		}
		return ErrDenied
	}
	for {
		if !invite.VerifyGrant(c.Issuer, c.Grant, Identity(c.Binding.Target), Identity(c.Binding.Controller), c.Now()) {
			return ErrDenied
		}
		conn.SetDeadline(time.Now().Add(3 * time.Second))
		if _, err := conn.Write([]byte{1}); err != nil {
			return ErrDenied
		}
		b := make([]byte, len(Payload))
		if _, err := io.ReadFull(conn, b); err != nil || string(b) != Payload {
			return ErrDenied
		}
		if received != nil {
			received()
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}
