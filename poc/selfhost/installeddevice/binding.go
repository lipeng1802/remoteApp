package installeddevice

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"remoteapp.local/selfhost-poc/invite"
	"time"
)

type proofEnrollmentNode interface {
	StartProof(context.Context, invite.DeviceSigner, invite.VerifierPermit) error
	ProofReady() bool
}

// Entirely internal: parent supplies no ticket, challenge, verifier or keys.
func (e *enrollment) bind(ctx context.Context, g *authorizationGate, client networkRequester) (enrollmentResult, error) {
	result := enrollmentResult{Status: "network_binding"}
	deny := func() (enrollmentResult, error) { return result, invite.ErrDenied }
	proof, ok := e.node.(proofEnrollmentNode)
	valid := func() bool {
		return ctx.Err() == nil && g != nil && g.clock != nil && g.signer != nil && g.validGrant(g.grant, g.clock()) && bytes.Equal(g.public, g.signer.PublicKey()) && e.node != nil && e.node.PublicKey() == e.key && overlayOK(e.address)
	}
	if !ok || client == nil || !canonical(e.ticket, 16) || !valid() {
		return deny()
	}
	var nonce [32]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return deny()
	}
	challenge := hex.EncodeToString(nonce[:])
	call, cancel := context.WithTimeout(ctx, 5*time.Second)
	reply, err := client.Request(call, invite.NetworkRequestBody{Action: "verifier", Grant: g.grant, Ticket: e.ticket, Challenge: challenge})
	failed := call.Err() != nil
	cancel()
	if reply.Credential != nil {
		*reply.Credential = invite.RegistrationCredential{}
	}
	if err != nil || failed || !valid() || reply.Verifier == nil || reply.Ticket != "" || reply.Credential != nil || reply.State != nil || reply.Proof != nil || reply.TLS != nil || reply.Session != nil || !invite.VerifyVerifierPermit(g.issuer, *reply.Verifier, g.grant, g.self, e.ticket, e.key, challenge, g.clock()) {
		return deny()
	}
	remaining := time.Unix(reply.Verifier.Claims.ExpiresAt, 0).Sub(g.clock())
	proofCtx, proofCancel := context.WithTimeout(ctx, remaining)
	// Node joins listener before SDK and signer. Failed start still owns any
	// partially created worker until closeJoined; never release native early.
	e.proofCancel = proofCancel
	if proof.StartProof(proofCtx, g.signer, *reply.Verifier) != nil || !proof.ProofReady() || !valid() {
		return deny()
	}
	budget := remaining
	if budget > 12*time.Second {
		budget = 12 * time.Second
	}
	bindCtx, bindCancel := context.WithTimeout(proofCtx, budget)
	bound, err := client.Request(bindCtx, invite.NetworkRequestBody{Action: "bind", Grant: g.grant, Ticket: e.ticket})
	failed = bindCtx.Err() != nil
	bindCancel()
	if bound.Credential != nil {
		*bound.Credential = invite.RegistrationCredential{}
	}
	if err != nil || failed || !valid() || !proof.ProofReady() || bound != (invite.NetworkReply{}) {
		return deny()
	}
	result.Passed = true // not active policy or payload permission
	return result, nil
}
