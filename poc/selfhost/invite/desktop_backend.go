package invite

import "time"

// NewDesktopProvisionBackend is trusted server composition, never an HTTP flag.
// It requires a Store explicitly created/reopened in desktop mode. The private
// pointer pins that same Store: assigning another public Store later cannot
// transfer the opt-in. The driver still manages only its own policy/resources.
func NewDesktopProvisionBackend(s *Store, driver ProvisionDriver) (*ProvisionBackend, error) {
	if s == nil || driver == nil {
		return nil, ErrDenied
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.broken || !s.state.DesktopAuthorization {
		return nil, ErrDenied
	}
	return &ProvisionBackend{Store: s, Driver: driver, desktopStore: s}, nil
}

func (b *ProvisionBackend) desktopNetwork() bool { return b.desktopStore != nil }

// Called with Store.mu held. Wrong-mode wrappers must not clear another
// wrapper's projection/readiness or invoke any privileged driver operation.
func (b *ProvisionBackend) networkModeLocked() bool {
	return b.Store != nil && b.Store.networkModeLocked(b.desktopNetwork()) &&
		(!b.desktopNetwork() || b.desktopStore == b.Store)
}
func (b *ProvisionBackend) networkMode() bool {
	if b.Store == nil {
		return false
	}
	b.Store.mu.Lock()
	defer b.Store.mu.Unlock()
	return b.networkModeLocked()
}
func (s *Store) networkModeLocked(desktop bool) bool {
	return s.state.DesktopAuthorization == desktop
}

// Scope comes from the authority's EXACT persisted request/grant, not the
// requester's permission string. Retain revoked grants for signed inactive
// state; positive authorization separately requires the approved ledger row.
func (s *Store) networkGrantPolicyLocked(g Grant, desktop bool) (GrantPolicy, bool) {
	if !s.networkModeLocked(desktop) {
		return GrantPolicy{}, false
	}
	if g.Claims.Permission == permission {
		return GrantPolicy{}, true
	}
	if !desktop {
		return GrantPolicy{}, false
	}
	for _, req := range s.state.Requests {
		if req.Grant != nil && *req.Grant == g && req.Owner == g.Claims.Target && req.Controller == g.Claims.Controller {
			policy, err := DesktopGrantPolicy(req.Permission)
			return policy, err == nil
		}
	}
	return GrantPolicy{}, false
}
func (s *Store) verifyNetworkGrantLocked(g Grant, desktop bool, now time.Time) bool {
	policy, ok := s.networkGrantPolicyLocked(g, desktop)
	return ok && policy.Verify(s.publicKeyLocked(), g, g.Claims.Target, g.Claims.Controller, now)
}
func (s *Store) authorizedNetworkLocked(g Grant, desktop bool) bool {
	if s.broken || s.clock().Unix() < s.state.LastTime || !s.verifyNetworkGrantLocked(g, desktop, s.clock()) {
		return false
	}
	for _, req := range s.state.Requests {
		if req.Status == "approved" && req.Grant != nil && *req.Grant == g && req.Owner == g.Claims.Target && req.Controller == g.Claims.Controller {
			return true
		}
	}
	return false
}
func (s *Store) authorizedNetwork(g Grant, desktop bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.authorizedNetworkLocked(g, desktop)
}
