package streamgate

import (
	"remoteapp.local/selfhost-poc/nodeproof"
	"remoteapp.local/selfhost-poc/startuptrace"
)

// A post-failure snapshot is diagnosis only, never retry permission or proof
// that the same state held throughout the failed connection attempt.
func deadlineRouteStage(r nodeproof.RouteStatus) startuptrace.Stage {
	if !r.QueryOK {
		return startuptrace.DeadlineRouteQueryFailed
	}
	if !r.PeerKnown || !r.IPMatches {
		return startuptrace.DeadlineRoutePeerMissing
	}
	if !r.Ready {
		return startuptrace.DeadlineRouteNotReady
	}
	if !r.InEngine {
		return startuptrace.DeadlineRouteEnginePending
	}
	if !r.SelfRelayKnown {
		return startuptrace.DeadlineSelfRelayUnknown
	}
	if !r.PeerRelayKnown {
		return startuptrace.DeadlinePeerRelayUnknown
	}
	if !r.HandshakeSeen {
		return startuptrace.DeadlineHandshakeUnseen
	}
	return startuptrace.DeadlineRouteEnginePresent
}
