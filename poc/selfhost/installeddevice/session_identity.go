package installeddevice

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"time"

	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/streamgate"
)

// SessionPeers is public-only trusted session material. It is constructed
// inside the installed owner from an issuer-signed reply and is never decoded
// from a parent frame. Accessors return copies so work cannot mutate the pin.
type SessionPeers struct {
	target, controller ed25519.PublicKey
	binding            invite.NetworkBinding
	grant              invite.Grant
	server             bool
	runtime            *SessionRuntime
}

func (p SessionPeers) TargetPublic() ed25519.PublicKey {
	return append(ed25519.PublicKey(nil), p.target...)
}
func (p SessionPeers) ControllerPublic() ed25519.PublicKey {
	return append(ed25519.PublicKey(nil), p.controller...)
}
func (p SessionPeers) Binding() invite.NetworkBinding { return p.binding }
func (p SessionPeers) Grant() invite.Grant            { return p.grant }
func (p SessionPeers) Server() bool                   { return p.server }

func (e *enrollment) prepareSessionPeers(ctx context.Context, g *authorizationGate, client networkRequester) error {
	if ctx == nil || ctx.Err() != nil || e.peers != nil || e.watch == nil || e.watch.Faulted() || g == nil || g.clock == nil || client == nil || e.binding == (invite.NetworkBinding{}) {
		return invite.ErrDenied
	}
	select {
	case <-e.watch.Done():
		return invite.ErrDenied
	default:
	}
	var nonce [32]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return invite.ErrDenied
	}
	challenge := hex.EncodeToString(nonce[:])
	call, cancel := context.WithTimeout(ctx, 5*time.Second)
	reply, err := client.Request(call, invite.NetworkRequestBody{Action: "session_identity", Grant: g.grant, Challenge: challenge})
	failed := call.Err() != nil
	cancel()
	if err != nil || failed || reply.Session == nil || reply.Ticket != "" || reply.Credential != nil || reply.State != nil || reply.Proof != nil || reply.TLS != nil || reply.Verifier != nil ||
		!invite.VerifySessionIdentity(g.issuer, *reply.Session, g.grant, e.binding, challenge, g.clock()) || !g.validGrant(g.grant, g.clock()) ||
		!bytes.Equal(g.public, g.signer.PublicKey()) || e.node == nil || e.node.PublicKey() != e.key {
		return invite.ErrDenied
	}
	target, targetErr := hex.DecodeString(reply.Session.Claims.TargetPublic)
	controller, controllerErr := hex.DecodeString(reply.Session.Claims.ControllerPublic)
	if targetErr != nil || controllerErr != nil || len(target) != ed25519.PublicKeySize || len(controller) != ed25519.PublicKeySize {
		return invite.ErrDenied
	}
	server := bytes.Equal(g.public, target)
	if !server && !bytes.Equal(g.public, controller) {
		return invite.ErrDenied
	}
	state, err := e.state(ctx, g, client, &e.binding)
	if err != nil || !state.Claims.Active {
		return invite.ErrDenied
	}
	payloadGate, err := streamgate.NewWithSigner(g.issuer, target, controller, g.signer, state, server, g.clock)
	if err != nil || !e.watch.AttachGate(payloadGate) {
		if payloadGate != nil {
			payloadGate.Deny()
		}
		return invite.ErrDenied
	}
	e.peers = &SessionPeers{target: append(ed25519.PublicKey(nil), target...), controller: append(ed25519.PublicKey(nil), controller...), binding: e.binding, grant: g.grant, server: server}
	e.peers.runtime = &SessionRuntime{peers: e.peers, gate: payloadGate, node: enrolledSessionServer(e.node)}
	return nil
}
