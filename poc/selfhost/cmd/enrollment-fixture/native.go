package main

import (
	"crypto/ed25519"
	"encoding/hex"
	"remoteapp.local/selfhost-poc/invite"
)

// Native mode accepts only canonical, distinct public identities, never keys.
func nativePublics(target, controller string) (ed25519.PublicKey, ed25519.PublicKey, error) {
	a, e := hex.DecodeString(target)
	b, f := hex.DecodeString(controller)
	if e != nil || f != nil || len(a) != 32 || len(b) != 32 || hex.EncodeToString(a) != target || hex.EncodeToString(b) != controller || target == controller {
		return nil, nil, invite.ErrDenied
	}
	return a, b, nil
}
