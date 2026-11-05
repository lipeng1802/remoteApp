// Package productprofile imports public prerelease trust for offline TEST builds.
// It never reads an issuer/release private key or takes an endpoint from a peer.
package productprofile

import (
	"crypto/ed25519"
	"encoding/hex"
	"remoteapp.local/selfhost-poc/trustprofile"
	"time"
)

// DesktopService is the separately anchored service used by desktop TEST builds.
const (
	DesktopService    = "remoteapp-desktop-test"
	DesktopEndpoint   = "https://mk.fengmap.com:8444"
	DesktopControlURL = "https://mk.fengmap.com:19443"
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

// DesktopClaims requires an explicit trusted build anchor from the isolated
// service bootstrap. The signed HTTPS name is preserved through the TEST tunnel.
func DesktopClaims(raw []byte, anchor string, now time.Time) (trustprofile.Claims, error) {
	root, err := hex.DecodeString(anchor)
	if err != nil || len(root) != ed25519.PublicKeySize || hex.EncodeToString(root) != anchor {
		return trustprofile.Claims{}, trustprofile.ErrTrust
	}
	p, err := trustprofile.Decode(raw)
	if err != nil {
		return trustprofile.Claims{}, err
	}
	if _, err = trustprofile.Verify(p, root, DesktopService, 1, now); err != nil {
		return trustprofile.Claims{}, err
	}
	if p.Claims.Endpoint != DesktopEndpoint {
		return trustprofile.Claims{}, trustprofile.ErrTrust
	}
	return shortClaims(p, now), nil
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
	return shortClaims(p, now), nil
}

func shortClaims(p trustprofile.Profile, now time.Time) trustprofile.Claims {
	c := p.Claims
	if expires := now.Add(20 * time.Minute).Unix(); expires < c.ExpiresAt {
		c.ExpiresAt = expires
	}
	c.IssuedAt = now.Unix()
	return c
}
