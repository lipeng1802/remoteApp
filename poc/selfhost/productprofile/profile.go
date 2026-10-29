// Package productprofile imports public prerelease trust for offline TEST builds.
// It never reads an issuer/release private key or takes an endpoint from a peer.
package productprofile

import (
	"crypto/ed25519"
	"encoding/hex"
	"remoteapp.local/selfhost-poc/trustprofile"
	"time"
)

const releaseRoot = "5b74a1a3b2b6c477b81016d71c38275d9fdd17d01e6c33ec7847dcd892a5198f"

// Claims verifies the existing signed service profile against the operator's
// fixed release anchor BEFORE copying its exact public connectivity claims.
// TEST validity is shortened; this cannot extend official validity or change
// CA, issuer, endpoint, service or epoch. Clients still verify the TEST root.
func Claims(raw []byte, now time.Time) (trustprofile.Claims, error) {
	root, _ := hex.DecodeString(releaseRoot)
	return verified(raw, root, now)
}
func verified(raw []byte, root ed25519.PublicKey, now time.Time) (trustprofile.Claims, error) {
	p, e := trustprofile.Decode(raw)
	if e != nil {
		return trustprofile.Claims{}, e
	}
	if _, e = trustprofile.Verify(p, root, "remoteapp-prerelease", 1, now); e != nil {
		return trustprofile.Claims{}, e
	}
	if p.Claims.Endpoint != "https://127.0.0.1:8444" && p.Claims.Endpoint != "https://mk.fengmap.com:8444" {
		return trustprofile.Claims{}, trustprofile.ErrTrust
	}
	c := p.Claims
	if expires := now.Add(20 * time.Minute).Unix(); expires < c.ExpiresAt {
		c.ExpiresAt = expires
	}
	c.IssuedAt = now.Unix()
	return c, nil
}
