package invite

import (
	"crypto/ed25519"
	"time"
)

// GrantPolicy is immutable trusted local composition, never selected from a
// grant, peer frame or JSON. Its zero value preserves fixed-payload-only callers.
// A desktop build must choose its exact scope independently before verifying
// any authority reply. This policy proves neither revocation nor local consent.
type GrantPolicy struct{ desktop DesktopPermission }

func DesktopGrantPolicy(expected DesktopPermission) (GrantPolicy, error) {
	if !expected.valid() {
		return GrantPolicy{}, ErrDenied
	}
	return GrantPolicy{desktop: expected}, nil
}

func (p GrantPolicy) Permission() string { return grantPermission(p.desktop) }
func (p GrantPolicy) IsDesktop() bool    { return p.desktop.valid() }

func (p GrantPolicy) Verify(pub ed25519.PublicKey, g Grant, target, controller string, now time.Time) bool {
	if p.desktop == "" {
		return VerifyGrant(pub, g, target, controller, now)
	}
	return VerifyDesktopGrant(pub, g, target, controller, p.desktop, now)
}

// A policy can cross an in-process trusted function boundary, not an IPC one.
func (GrantPolicy) MarshalJSON() ([]byte, error) { return nil, ErrDenied }
func (*GrantPolicy) UnmarshalJSON([]byte) error  { return ErrDenied }
