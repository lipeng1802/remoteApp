// Package broker defines a fixed signed-request server-local authority boundary.
// It is not a generic command runner or a public network service.
package broker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"remoteapp.local/selfhost-poc/invite"
)

var ErrBroker = errors.New("local_authority_unavailable")

type Request struct {
	Operation string                 `json:"operation"`
	Device    *invite.Envelope       `json:"device,omitempty"`
	Network   *invite.NetworkRequest `json:"network,omitempty"`
}
type Reply struct {
	Status  int                  `json:"status"`
	Issuer  string               `json:"issuer,omitempty"`
	Device  *invite.Result       `json:"device,omitempty"`
	Network *invite.NetworkReply `json:"network,omitempty"`
}
type Executor interface {
	Execute(context.Context, Request) Reply
}
type Authority struct {
	Store   *invite.Store
	Backend *invite.ProvisionBackend
	// NetworkReady is a trusted, live fail-closed predicate. A journal/IPC
	// flag cannot satisfy it. Future supervisor MUST cancel/fence backend on
	// verifier failure; this component alone is not that lifecycle wiring.
	NetworkReady func() bool
}

func (a *Authority) Execute(ctx context.Context, r Request) Reply {
	if a == nil || a.Store == nil || !a.Store.Available() || ctx.Err() != nil {
		return Reply{Status: 503}
	}
	switch r.Operation {
	case "identity":
		if r.Device != nil || r.Network != nil {
			return Reply{Status: 403}
		}
		return Reply{Status: 200, Issuer: hex.EncodeToString(a.Store.PublicKey())}
	case "device":
		if r.Device == nil || r.Network != nil {
			return Reply{Status: 403}
		}
		result, e := a.Store.Execute(*r.Device)
		if e == invite.ErrStorage {
			return Reply{Status: 503}
		}
		if e != nil {
			return Reply{Status: 403}
		}
		return Reply{Status: 200, Issuer: hex.EncodeToString(a.Store.PublicKey()), Device: &result}
	case "network":
		if r.Network == nil || r.Device != nil {
			return Reply{Status: 403}
		}
		if a.Backend == nil || a.Backend.Store != a.Store || a.NetworkReady == nil || !a.NetworkReady() {
			return Reply{Status: 503}
		}
		pub, e := hex.DecodeString(r.Network.PublicKey)
		if e != nil || len(pub) != 32 {
			return Reply{Status: 403}
		}
		id := sha256.Sum256(pub)
		bound, e := a.Store.DevicePublicKey(hex.EncodeToString(id[:]))
		if e != nil || hex.EncodeToString(bound) != r.Network.PublicKey {
			return Reply{Status: 403}
		}
		// A persisted, signature-bound device key is mandatory for nodeproof.
		// ExecuteNetworkRequest still rechecks signature/nonce/grant/permission.
		result, e := a.Backend.ExecuteNetworkRequest(ctx, *r.Network)
		if e == invite.ErrStorage {
			return Reply{Status: 503}
		}
		if e != nil {
			return Reply{Status: 403}
		}
		return Reply{Status: 200, Issuer: hex.EncodeToString(a.Store.PublicKey()), Network: &result}
	default:
		return Reply{Status: 403}
	}
}

type Config struct {
	Socket           string
	PeerUID, GroupID uint32
	Executor         Executor
}
type Client struct {
	Socket    string
	ServerUID uint32
}
