package installeddevice

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/netip"
	"strings"
	"time"

	"remoteapp.local/selfhost-poc/invite"
)

// EnrollmentNode is a trusted in-process adapter, never a parent/peer supplied
// capability. Prepare must own exclusive private node storage before returning.
// Register consumes its secret only in memory, opens no data listener, and must
// honor cancellation. CloseJoined must wait for every node worker, even on error.
type EnrollmentNode interface {
	PublicKey() string
	Register(context.Context, string) (nodeKey, address string, err error)
	CloseJoined() error
}

type EnrollmentFactory func(context.Context) (EnrollmentNode, error)

type enrollment struct {
	node                 EnrollmentNode
	ticket, key, address string
	attempted            bool
}

type enrollmentResult struct {
	Status string `json:"status"`
	Passed bool   `json:"passed"`
	Active bool   `json:"active"`
}

func nodeKeyOK(k string) bool {
	return strings.HasPrefix(k, "nodekey:") && canonical(strings.TrimPrefix(k, "nodekey:"), 32) && k != "nodekey:"+strings.Repeat("0", 64)
}

func overlayOK(s string) bool {
	ip, err := netip.ParseAddr(s)
	return err == nil && ip.String() == s && netip.MustParsePrefix("100.120.0.0/24").Contains(ip) && s != "100.120.0.0" && s != "100.120.0.255"
}

// attempted is latched BEFORE any external effect. Unknown HTTP/registration
// outcomes are not retried, and node state is not deleted or marked cleaned.
// Issuer reconciliation, not a client guess, owns remote ticket cleanup.
func (e *enrollment) start(ctx context.Context, g *authorizationGate, client networkRequester, factory EnrollmentFactory, grant invite.Grant) (enrollmentResult, error) {
	result := enrollmentResult{Status: "network_enrollment"}
	deny := func() (enrollmentResult, error) { return result, invite.ErrDenied }
	if e.attempted || factory == nil || client == nil || ctx.Err() != nil || g == nil || g.signer == nil || g.clock == nil || g.grant != grant || !g.validGrant(grant, g.clock()) || !bytes.Equal(g.public, g.signer.PublicKey()) {
		return deny()
	}
	e.attempted = true
	// Ticket issuance is limited to 120s by the authority. Bound the complete
	// prepare/enroll/Up sequence to 90s and the verified grant's remaining life.
	budget := time.Unix(grant.Claims.ExpiresAt, 0).Sub(g.clock())
	if budget > 90*time.Second {
		budget = 90 * time.Second
	}
	if budget <= 0 {
		return deny()
	}
	call, cancel := context.WithTimeout(ctx, budget)
	defer cancel()
	var err error
	e.node, err = factory(call)
	if err != nil || e.node == nil || call.Err() != nil {
		return deny()
	}
	e.key = e.node.PublicKey()
	if !nodeKeyOK(e.key) || !g.validGrant(grant, g.clock()) || !bytes.Equal(g.public, g.signer.PublicKey()) {
		return deny()
	}
	var nonce [16]byte
	if _, err = rand.Read(nonce[:]); err != nil {
		return deny()
	}
	intent, err := invite.SignProvisionWithSigner(g.signer, invite.ProvisionBody{GrantID: grant.Claims.ID, NodeKey: e.key, Nonce: hex.EncodeToString(nonce[:]), IssuedAt: g.clock().Unix()})
	if err != nil || call.Err() != nil || !g.validGrant(grant, g.clock()) || !bytes.Equal(g.public, g.signer.PublicKey()) {
		return deny()
	}
	httpCtx, httpCancel := context.WithTimeout(call, 5*time.Second)
	reply, err := client.Request(httpCtx, invite.NetworkRequestBody{Action: "enroll", Grant: grant, Intent: intent})
	httpFailed := httpCtx.Err() != nil
	httpCancel()
	// Always clear our credential copy, including a malformed mixed reply.
	if reply.Credential != nil {
		defer func() { *reply.Credential = invite.RegistrationCredential{} }()
	}
	if err != nil || httpFailed || call.Err() != nil || !canonical(reply.Ticket, 16) || reply.Credential == nil || reply.Credential.Secret == "" || len(reply.Credential.Secret) > 256 || reply.State != nil || reply.Proof != nil || reply.TLS != nil || !g.validGrant(grant, g.clock()) || !bytes.Equal(g.public, g.signer.PublicKey()) || e.node.PublicKey() != e.key {
		return deny()
	}
	e.ticket = reply.Ticket // private, for the later live-proof/bind phase only
	k, address, err := e.node.Register(call, reply.Credential.Secret)
	if err != nil || call.Err() != nil || k != e.key || e.node.PublicKey() != e.key || !overlayOK(address) || !g.validGrant(grant, g.clock()) || !bytes.Equal(g.public, g.signer.PublicKey()) {
		return deny()
	}
	e.address = address
	result.Passed = true
	// Up/key/IP is not live possession, binding, active policy or data authority.
	return result, nil
}

func (e *enrollment) closeJoined() error {
	if e.node == nil {
		return nil
	}
	err := e.node.CloseJoined()
	e.ticket, e.key, e.address = "", "", ""
	return err
}
