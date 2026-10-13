package installeddevice

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/trustprofile"
)

type networkRequester interface {
	Request(context.Context, invite.NetworkRequestBody) (invite.NetworkReply, error)
}

// The calibrated clock and verified state stay inside the leased client. Only
// fixed diagnostics leave this layer; these are not capabilities to open data.
type authorizationGate struct {
	issuer ed25519.PublicKey
	public ed25519.PublicKey
	signer invite.DeviceSigner
	self   string
	grant  invite.Grant
	clock  func() time.Time
}
type authorizationResult struct {
	Status string `json:"status"`
	Phase  string `json:"phase"`
	Passed bool   `json:"passed"`
	Active bool   `json:"active"`
}

func newAuthorizationGate(p trustprofile.Profile, signer invite.DeviceSigner) *authorizationGate {
	issuer, _ := hex.DecodeString(p.Claims.Issuer)
	pub := append(ed25519.PublicKey(nil), signer.PublicKey()...)
	id := sha256.Sum256(pub)
	return &authorizationGate{issuer: issuer, public: pub, signer: signer, self: hex.EncodeToString(id[:])}
}

func (g *authorizationGate) validGrant(v invite.Grant, now time.Time) bool {
	c := v.Claims
	return canonical(v.Signature, 64) && canonical(c.ID, 16) && canonical(c.Target, 32) && canonical(c.Controller, 32) && c.Target != c.Controller && (g.self == c.Target || g.self == c.Controller) && invite.VerifyGrant(g.issuer, v, c.Target, c.Controller, now)
}

func (g *authorizationGate) request(ctx context.Context, client networkRequester, b invite.NetworkRequestBody) (authorizationResult, error) {
	result := authorizationResult{Status: "network_authorization"}
	fail := func() (authorizationResult, error) {
		g.clock = nil
		g.grant = invite.Grant{}
		return result, invite.ErrDenied
	}
	if ctx.Err() != nil || client == nil || g.signer == nil || !bytes.Equal(g.public, g.signer.PublicKey()) || b != (invite.NetworkRequestBody{Action: b.Action, Grant: b.Grant}) || !g.validGrant(b.Grant, time.Unix(b.Grant.Claims.IssuedAt, 0)) {
		return fail()
	}
	call, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	switch b.Action {
	case "grant_clock":
		result.Phase = "grant_clock"
		// Caller cannot supply challenge or clock. Count the entire RTT from
		// this monotonic start and require the issuer's online authorization.
		g.clock = nil
		g.grant = invite.Grant{}
		var challenge [32]byte
		if _, err := rand.Read(challenge[:]); err != nil {
			return fail()
		}
		b.Challenge = hex.EncodeToString(challenge[:])
		start := time.Now()
		reply, err := client.Request(call, b)
		if err != nil || call.Err() != nil || !bytes.Equal(g.public, g.signer.PublicKey()) || reply.Proof == nil || reply.Credential != nil || reply.State != nil || reply.TLS != nil || reply.Ticket != "" {
			return fail()
		}
		clock, err := invite.AuthorityClock(g.issuer, *reply.Proof, b.Grant.Claims.ID, b.Challenge, start)
		if err != nil || !g.validGrant(b.Grant, clock()) {
			return fail()
		}
		g.clock = clock
		g.grant = b.Grant
		result.Passed = true
	case "state":
		result.Phase = "state"
		if g.clock == nil || g.grant != b.Grant || !g.validGrant(b.Grant, g.clock()) {
			return fail()
		}
		reply, err := client.Request(call, b)
		if err != nil || call.Err() != nil || !bytes.Equal(g.public, g.signer.PublicKey()) || reply.State == nil || reply.Proof != nil || reply.Credential != nil || reply.TLS != nil || reply.Ticket != "" || reply.State.Claims.Grant != g.grant || !invite.VerifyNetworkState(g.issuer, *reply.State, g.clock()) || !g.validGrant(g.grant, g.clock()) {
			return fail()
		}
		result.Passed = true
		result.Active = reply.State.Claims.Active
		// Genuine inactive state is observable but terminal for this owner.
		if !result.Active {
			g.clock = nil
			g.grant = invite.Grant{}
			return result, invite.ErrDenied
		}
	default:
		return fail()
	}
	return result, nil
}
