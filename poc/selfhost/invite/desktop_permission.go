package invite

import (
	"crypto/ed25519"
	"encoding/hex"
	"time"
)

// DesktopPermission is an explicit signed request scope, never inferred from a
// TLS/JPEG capability, a loopback address, local consent alone or fixed-payload.
// Clipboard and unattended access are deliberately absent.
type DesktopPermission string

const (
	DesktopView    DesktopPermission = "desktop-view"
	DesktopControl DesktopPermission = "desktop-control"
)

func (p DesktopPermission) valid() bool { return p == DesktopView || p == DesktopControl }

func grantPermission(p DesktopPermission) string {
	if p == "" {
		return permission
	}
	return string(p)
}

func validDesktopBody(b Body) bool {
	allowed := Body{Action: b.Action, Nonce: b.Nonce, IssuedAt: b.IssuedAt, Permission: b.Permission}
	switch b.Action {
	case "invite":
	case "claim":
		allowed.TargetCode, allowed.InviteID, allowed.Code = b.TargetCode, b.InviteID, b.Code
	case "approve":
		allowed.RequestID, allowed.Allow = b.RequestID, b.Allow
	default:
		return false
	}
	return b == allowed
}

// OpenDesktopAuthorization opts trusted composition into scoped DEVICE approval
// only. Current network/watch/TLS clients remain fixed-payload-only and reject
// these new grants. No deployed entry calls this constructor yet. It cannot
// upgrade an existing fixed-only Store or relax writer/recovery policy.
func OpenDesktopAuthorization(root string, clock func() time.Time) (*Store, error) {
	return openAuthorization(root, clock, true)
}

// VerifyDesktopGrant requires the locally selected exact scope. Control is not
// silently substituted for view, or vice versa. This is signature/expiry checking,
// NOT current revocation or proof of local GUI consent.
func VerifyDesktopGrant(pub ed25519.PublicKey, g Grant, target, controller string, expected DesktopPermission, now time.Time) bool {
	return expected.valid() && verifyGrantPermission(pub, g, target, controller, string(expected), now)
}

// AuthorizedDesktop is deliberately separate from Authorized/authorizedLocked:
// the existing network provisioning and fixed TLS path must NOT accept desktop
// grants until its own compiled permission-specific integration is complete.
func (s *Store) AuthorizedDesktop(g Grant, target, controller string, expected DesktopPermission) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.broken || !s.state.DesktopAuthorization || s.clock().Unix() < s.state.LastTime ||
		!VerifyDesktopGrant(s.publicKeyLocked(), g, target, controller, expected, s.clock()) {
		return false
	}
	for _, req := range s.state.Requests {
		if req.Status == "approved" && req.Permission == expected && req.Owner == target && req.Controller == controller &&
			req.Grant != nil && *req.Grant == g {
			return true
		}
	}
	return false
}

func validDesktopRecords(st snapshot) bool {
	valid := func(p DesktopPermission) bool { return p == "" || st.DesktopAuthorization && p.valid() }
	for _, inv := range st.Invites {
		if !valid(inv.Permission) {
			return false
		}
		if inv.RequestID != "" {
			req, ok := st.Requests[inv.RequestID]
			if (inv.Permission != "" || req.Permission != "") &&
				(!ok || inv.Owner != req.Owner || inv.Permission != req.Permission) {
				return false
			}
		}
	}
	raw, err := hex.DecodeString(st.PrivateKey)
	if err != nil || len(raw) != ed25519.PrivateKeySize {
		return false
	}
	key := ed25519.PrivateKey(raw).Public().(ed25519.PublicKey)
	for _, req := range st.Requests {
		if !valid(req.Permission) {
			return false
		}
		if req.Permission != "" && req.Grant != nil &&
			!VerifyDesktopGrant(key, *req.Grant, req.Owner, req.Controller, req.Permission, time.Unix(req.Grant.Claims.IssuedAt, 0)) {
			return false
		}
		if req.Permission == "" && req.Grant != nil && req.Grant.Claims.Permission != permission {
			return false
		}
	}
	// A scoped provision must still name that exact persisted signed request
	// after restart. Unknown/floating records cannot be treated as cleanup work
	// by a differently scoped backend or silently adopted as a live binding.
	for _, record := range st.Provisions {
		if record.Grant.Claims.Permission == permission {
			continue
		}
		known := false
		if st.DesktopAuthorization {
			for _, req := range st.Requests {
				if req.Permission.valid() && req.Grant != nil && *req.Grant == record.Grant {
					known = true
					break
				}
			}
		}
		if !known {
			return false
		}
	}
	return true
}
