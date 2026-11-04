package installeddevice

import (
	"context"
	"remoteapp.local/selfhost-poc/trustprofile"
)

// pending is armed only by successful watched provision in compiled TEST wiring.
// No identity or initial stream gate is prepared until this one-shot start.
type controllerStart struct{ pending, attempted bool }

func (s *controllerStart) allowed(e *enrollment, g *authorizationGate, work SessionWork) bool {
	return s.pending && !s.attempted && work != nil && g != nil && g.self != "" && g.self == g.grant.Claims.Controller && e != nil && e.watch != nil && !e.watch.Faulted() && e.session == nil && e.peers == nil
}
func (s *controllerStart) run(ctx context.Context, e *enrollment, g *authorizationGate, client networkRequester, work SessionWork) error {
	if !s.allowed(e, g, work) {
		return trustprofile.ErrTrust
	}
	s.attempted = true // unknown/failed admission is never rearmed
	if err := e.prepareSessionPeers(ctx, g, client); err != nil {
		return err
	}
	return e.startSession(ctx, work)
}
