package streamgate

import (
	"remoteapp.local/selfhost-poc/nodeproof"
	"remoteapp.local/selfhost-poc/startuptrace"
	"testing"
)

func TestDeadlineRouteSnapshotDoesNotConflateReadinessAndEngine(t *testing.T) {
	for _, v := range []struct {
		r     nodeproof.RouteStatus
		stage startuptrace.Stage
	}{
		{nodeproof.RouteStatus{}, startuptrace.DeadlineRouteQueryFailed},
		{nodeproof.RouteStatus{QueryOK: true}, startuptrace.DeadlineRoutePeerMissing},
		{nodeproof.RouteStatus{QueryOK: true, PeerKnown: true, IPMatches: true}, startuptrace.DeadlineRouteNotReady},
		{nodeproof.RouteStatus{QueryOK: true, PeerKnown: true, IPMatches: true, Ready: true}, startuptrace.DeadlineRouteEnginePending},
		{nodeproof.RouteStatus{QueryOK: true, PeerKnown: true, IPMatches: true, Ready: true, InEngine: true}, startuptrace.DeadlineSelfRelayUnknown},
		{nodeproof.RouteStatus{QueryOK: true, PeerKnown: true, IPMatches: true, Ready: true, InEngine: true, SelfRelayKnown: true}, startuptrace.DeadlinePeerRelayUnknown},
		{nodeproof.RouteStatus{QueryOK: true, PeerKnown: true, IPMatches: true, Ready: true, InEngine: true, SelfRelayKnown: true, PeerRelayKnown: true}, startuptrace.DeadlineHandshakeUnseen},
		{nodeproof.RouteStatus{QueryOK: true, PeerKnown: true, IPMatches: true, Ready: true, InEngine: true, SelfRelayKnown: true, PeerRelayKnown: true, HandshakeSeen: true}, startuptrace.DeadlineRouteEnginePresent},
	} {
		if got := deadlineRouteStage(v.r); got != v.stage || !startuptrace.Valid(string(got)) {
			t.Fatal("snapshot misclassified")
		}
	}
}
