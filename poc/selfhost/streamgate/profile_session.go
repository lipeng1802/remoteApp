package streamgate

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"

	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/session"
	"remoteapp.local/selfhost-poc/trustinstall"
	"remoteapp.local/selfhost-poc/trustprofile"
)

// ProfileSession is the live Runner adapter for clientowner. The installation
// supplies p; the compiled release supplies root/service/floor. There is no
// raw endpoint/CA/issuer fallback. work runs the actual Gate.ServeTLS or
// Gate.ConnectTLS (or fixed-payload diagnostics) and MUST join its nested node,
// listener and stream workers before returning. Signer is borrowed until return.
func ProfileSession(ctx context.Context, release trustinstall.Release, p trustprofile.Profile, signer invite.DeviceSigner, target, controller ed25519.PublicKey, grant invite.Grant, binding invite.NetworkBinding, server bool, work func(context.Context, *Gate) error) error {
	if ctx == nil || work == nil {
		return session.ErrDenied
	}
	if ctx.Err() != nil {
		return nil // cancelled before startup: no workers/resources to join
	}
	a := release.Anchor()
	client, e := invite.NewProfileNetworkClient(p, a.Root[:], a.Service, a.MinimumEpoch, signer)
	if e != nil {
		if ctx.Err() != nil {
			return nil
		}
		return e
	}
	defer client.Close() // only after the watch AND stream worker join
	issuer, e := hex.DecodeString(p.Claims.Issuer)
	if e != nil {
		return session.ErrDenied
	}
	clock, e := Calibrate(ctx, client, issuer, grant)
	if e != nil {
		if ctx.Err() != nil {
			return nil
		}
		return e
	}
	life, cancel := context.WithCancel(ctx)
	defer cancel()
	m, e := StartManaged(life, client, issuer, target, controller, signer, grant, binding, server, clock)
	if e != nil {
		if ctx.Err() != nil {
			return nil
		} // StartManaged joined its failed watch
		return e
	}
	done := make(chan error, 1)
	go func() { done <- work(life, m.Gate()) }()
	var workerErr error
	ended := false
	fault := false
	select {
	case workerErr = <-done:
		ended = true
	case <-ctx.Done():
	case <-m.Done():
		fault = ctx.Err() == nil
	}
	// Deny before cancellation/join. Never publish a new profile just because
	// cancellation was requested, even if work is stuck despite its context.
	m.Gate().Deny()
	cancel()
	m.Stop()
	if !ended {
		workerErr = <-done
	}
	if fault {
		return session.ErrDenied
	}
	if ctx.Err() != nil {
		return nil
	} // joined cancellation is normal shutdown
	return workerErr
}
