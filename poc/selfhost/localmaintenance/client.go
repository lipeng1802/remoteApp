package localmaintenance

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"net"
	"time"

	"remoteapp.local/selfhost-poc/privatepipe"
	"remoteapp.local/selfhost-poc/trustinstall"
	"remoteapp.local/selfhost-poc/trustprofile"
)

// RequestTest never initializes an identity or selects one from peer metadata.
// Address/public/signer are trusted local wiring. A successful result is only
// an expectation: the uninstaller must acquire the native lease and verify the
// same transaction before any deletion. No profile or private key is returned.
func RequestTest(ctx context.Context, r trustinstall.Release, address string, public ed25519.PublicKey, signer Signer) (trustprofile.MaintenanceState, error) {
	if ctx == nil || ctx.Err() != nil || !r.Specification().Test || !localAddress(address) || len(public) != 32 || signer == nil || !bytes.Equal(public, signer.PublicKey()) {
		return trustprofile.MaintenanceState{}, ErrDenied
	}
	life, cancel := context.WithTimeout(ctx, 40*time.Second)
	defer cancel()
	c, e := (&net.Dialer{}).DialContext(life, "tcp", address)
	if e != nil {
		return trustprofile.MaintenanceState{}, ErrDenied
	}
	defer c.Close()
	stop := context.AfterFunc(life, func() { c.Close() })
	defer stop()
	deadline, _ := life.Deadline()
	if c.SetDeadline(deadline) != nil {
		return trustprofile.MaintenanceState{}, ErrDenied
	}
	return request(life, r, public, signer, c)
}

func request(ctx context.Context, r trustinstall.Release, public ed25519.PublicKey, s Signer, c net.Conn) (trustprofile.MaintenanceState, error) {
	read := privatepipe.NewBoundedReader(c, 4096)
	write := json.NewEncoder(c)
	nonce, e := random(32)
	if e != nil {
		return trustprofile.MaintenanceState{}, e
	}
	base := frame{Op: "hello", Version: 1, Scope: r.Scope(), Release: releaseDigest(r), ClientNonce: nonce}
	if write.Encode(base) != nil {
		return trustprofile.MaintenanceState{}, ErrDenied
	}
	var challenge frame
	if read.Read(&challenge) != nil || !canonical(challenge.Instance, 16) || !canonical(challenge.ServerNonce, 32) {
		return trustprofile.MaintenanceState{}, ErrDenied
	}
	base.Op = "challenge"
	base.Instance = challenge.Instance
	base.ServerNonce = challenge.ServerNonce
	if !verify(challenge, base, public) {
		return trustprofile.MaintenanceState{}, ErrDenied
	}
	base.Op = "authenticate"
	proof, e := sign(base, s)
	if e != nil || write.Encode(proof) != nil {
		return trustprofile.MaintenanceState{}, ErrDenied
	}
	var ready frame
	if read.Read(&ready) != nil || ready.Epoch < r.Specification().MinimumEpoch || !canonical(ready.Transaction, 16) {
		return trustprofile.MaintenanceState{}, ErrDenied
	}
	base.Op = "ready"
	base.Epoch = ready.Epoch
	base.Transaction = ready.Transaction
	if !verify(ready, base, public) {
		return trustprofile.MaintenanceState{}, ErrDenied
	}
	base.Op = "finish"
	finish, e := sign(base, s)
	if e != nil || write.Encode(finish) != nil {
		return trustprofile.MaintenanceState{}, ErrDenied
	}
	var closed frame
	base.Op = "closed"
	if read.Read(&closed) != nil || !verify(closed, base, public) || ctx.Err() != nil {
		return trustprofile.MaintenanceState{}, ErrDenied
	}
	return trustprofile.MaintenanceState{State: "pending", Epoch: base.Epoch, Transaction: base.Transaction}, nil
}
