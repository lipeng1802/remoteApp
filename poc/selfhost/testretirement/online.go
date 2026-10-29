package testretirement

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"remoteapp.local/selfhost-poc/autonomousbinding"
	"remoteapp.local/selfhost-poc/clientruntime"
	"remoteapp.local/selfhost-poc/enrollmentstore"
	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/trustinstall"
	"remoteapp.local/selfhost-poc/trustprofile"
	"time"
)

var errOnlineRetirement = errors.New("node_retirement_unconfirmed")

func confirmNodeRetirement(ctx context.Context, r trustinstall.Release, p trustprofile.Profile, identity autonomousbinding.Record, node enrollmentstore.Record, vault, digest string, tunnel bool) (result error) {
	if ctx == nil || ctx.Err() != nil || !r.Specification().Test || node.Phase != "registered" {
		return errOnlineRetirement
	}
	signer, err := clientruntime.NativeSigner(vault, digest, identity.VaultScope, identity.PublicKey)()
	if err != nil {
		return errOnlineRetirement
	}
	joined, ok := signer.(interface{ CloseJoined() error })
	if !ok {
		signer.Close()
		return errOnlineRetirement
	}
	closed := false
	finish := func() error {
		if closed {
			return nil
		}
		closed = true
		return joined.CloseJoined()
	}
	defer func() {
		if finish() != nil {
			result = errOnlineRetirement
		}
	}()
	root, err := hex.DecodeString(r.Specification().Root)
	if err != nil {
		return errOnlineRetirement
	}
	create := invite.NewProfileNetworkClient
	if tunnel {
		create = invite.NewTestTunnelNetworkClient
	}
	client, err := create(p, root, r.Specification().Service, r.Specification().MinimumEpoch, signer)
	if err != nil {
		return errOnlineRetirement
	}
	defer client.Close()
	life, cancel := client.BoundLifetime(ctx)
	defer cancel()
	issuer, err := hex.DecodeString(p.Claims.Issuer)
	if err != nil {
		return errOnlineRetirement
	}
	subject := sha256.Sum256(signer.PublicKey())
	return confirmWithClient(life, client, issuer, hex.EncodeToString(subject[:]), node, time.Now, func() error {
		client.Close()
		if finish() != nil {
			return errOnlineRetirement
		}
		_, e := trustprofile.Verify(p, root, r.Specification().Service, r.Specification().MinimumEpoch, time.Now())
		return e
	})
}

type retirementAPI interface {
	DeviceRequest(context.Context, invite.Body) (invite.Result, error)
}

func confirmWithClient(ctx context.Context, client retirementAPI, issuer []byte, subject string, node enrollmentstore.Record, clock func() time.Time, after func() error) error {
	if ctx == nil || ctx.Err() != nil || node.Phase != "registered" {
		return errOnlineRetirement
	}
	var nonce [32]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return errOnlineRetirement
	}
	challenge := hex.EncodeToString(nonce[:])
	call, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	start := clock()
	r, err := client.DeviceRequest(call, invite.Body{Action: "node_retirement", NodeKey: node.NodeKey, Challenge: challenge})
	if after != nil && after() != nil {
		return errOnlineRetirement
	}
	if err != nil || call.Err() != nil || r.Retirement == nil || r != (invite.Result{Status: "node_retired", Retirement: r.Retirement}) {
		return errOnlineRetirement
	}
	// Fresh challenge binds this issuer reply to our one request. Charge the
	// ENTIRE monotonic round trip and signer join against its three-second life,
	// rather than assuming two machines' wall clocks agree. Profile validity and
	// the client's independent monotonic profile deadline remain unchanged.
	elapsed := clock().Sub(start)
	if elapsed < 0 || elapsed >= 3*time.Second || !invite.VerifyNodeRetirement(issuer, *r.Retirement, subject, node.NodeKey, node.Address, challenge, time.Unix(0, r.Retirement.Claims.IssuedAt).Add(elapsed)) {
		return errOnlineRetirement
	}
	return nil
}
