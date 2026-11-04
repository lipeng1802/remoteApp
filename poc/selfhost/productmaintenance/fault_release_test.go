package productmaintenance

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

type failedSignerJoin struct{ testSigner }

func TestFaultReleaseCannotRaceWatchIntoOrdinaryZeroExit(t *testing.T) {
	c, signer, _ := controllerFixture(t, "fault-race")
	c.prepared = false
	c.provisioned = true
	c.revocation = &revokeFixture{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := c.ReleaseFault(ctx); err != nil || !c.child.JoinedFault() || signer.closes.Load() != 1 {
		t.Fatal("fault release sent an ordinary stop before watch observed revocation", err)
	}
}

func (s *failedSignerJoin) CloseJoined() error { s.closes.Add(1); return ErrProduct }

func TestFaultSignerJoinFailureCannotBecomeLateFaultProof(t *testing.T) {
	c, _, _ := controllerFixture(t, "fault")
	c.prepared = false
	c.provisioned = true
	c.revocation = &revokeFixture{}
	signer := &failedSignerJoin{}
	c.signer = signer
	if c.ReleaseFault(context.Background()) == nil || c.faultJoined || !c.faultReleaseFailed {
		t.Fatal("unknown signer join acknowledged")
	}
	if c.ReleaseFault(context.Background()) == nil || signer.closes.Load() != 1 {
		t.Fatal("failed signer join was rearmed")
	}
}

func TestFaultReleaseRequiresReadOnlyRevocationAndRealFailedChildJoin(t *testing.T) {
	c, s, _ := controllerFixture(t, "fault")
	c.prepared = false
	c.provisioned = true
	rev := &revokeFixture{unknown: true}
	c.revocation = rev
	if c.ReleaseFault(context.Background()) == nil || s.closes.Load() != 0 || c.released {
		t.Fatal("unknown revocation sacrificed signer")
	}
	select {
	case <-c.child.Exited():
		t.Fatal("unknown confirmation stopped product")
	default:
	}
	rev.unknown = false
	if e := c.ReleaseFault(context.Background()); e != nil {
		t.Fatal(e)
	}
	if !c.child.JoinedFault() || !c.faultJoined || s.closes.Load() != 1 || rev.closes != 1 || rev.revokes != 0 {
		t.Fatal("fault proof or signer join missing")
	}
	if c.ReleaseFault(context.Background()) != nil || s.closes.Load() != 1 {
		t.Fatal("confirmed close repeated")
	}
}

func TestFaultReceiptRejectsNormalExtraDuplicateAndZeroExit(t *testing.T) {
	for _, mode := range []string{"normal", "fault-extra", "fault-duplicate", "fault-zero", "fault-generic"} {
		t.Run(mode, func(t *testing.T) {
			c, s, _ := controllerFixture(t, mode)
			c.prepared = false
			c.provisioned = true
			c.revocation = &revokeFixture{}
			if c.ReleaseFault(context.Background()) == nil || c.faultJoined || s.closes.Load() != 0 {
				t.Fatal("invalid terminal became fault proof")
			}
		})
	}
}

func TestFaultStopTimeoutRetainsSameChildAndSigner(t *testing.T) {
	c, s, _ := controllerFixture(t, "fault-slow")
	c.prepared = false
	c.provisioned = true
	c.revocation = &revokeFixture{}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if c.ReleaseFault(ctx) == nil || s.closes.Load() != 0 {
		t.Fatal("timeout became joined")
	}
	if e := c.ReleaseFault(context.Background()); e != nil || s.closes.Load() != 1 {
		t.Fatal("explicit same-child join failed", e)
	}
}

func TestFaultTerminalConsumedByTelemetryStillRequiresActualExitJoin(t *testing.T) {
	c, s, _ := controllerFixture(t, "forward-fault-count")
	c.prepared = false
	c.provisioned = true
	c.revocation = &revokeFixture{}
	if _, e := c.Count(context.Background()); e == nil {
		t.Fatal("terminal failure was a count")
	}
	if e := c.ReleaseFault(context.Background()); e != nil || !c.child.JoinedFault() || s.closes.Load() != 1 {
		t.Fatal("in-flight terminal lost or join missing", e)
	}
}

func TestFaultReleaseReasonsAreFixedAndPreserveDenial(t *testing.T) {
	for _, item := range []struct {
		cause  error
		reason string
	}{
		{errFaultLock, "lock_unavailable"}, {errFaultState, "state_rejected"},
		{errFaultRevocation, "revocation_unconfirmed"}, {errFaultChild, "child_join_unconfirmed"},
		{errFaultSigner, "signer_join_unconfirmed"},
	} {
		err := errors.Join(ErrProduct, fmt.Errorf("PRIVATE: %w", item.cause))
		if FaultReleaseReason(err) != item.reason || !errors.Is(err, ErrProduct) {
			t.Fatal("fixed reason mismatch")
		}
	}
	if FaultReleaseReason(errors.New("revocation_unconfirmed PRIVATE")) != "unavailable" {
		t.Fatal("text treated as typed proof")
	}
	c, _, _ := controllerFixture(t, "fault")
	c.prepared = false
	c.provisioned = true
	c.revocation = &revokeFixture{unknown: true}
	if e := c.ReleaseFault(context.Background()); FaultReleaseReason(e) != "revocation_unconfirmed" || !errors.Is(e, ErrProduct) {
		t.Fatal("revocation failure misclassified")
	}
	c.revocation = &revokeFixture{}
	c.signer = &failedSignerJoin{}
	if e := c.ReleaseFault(context.Background()); FaultReleaseReason(e) != "signer_join_unconfirmed" || !c.faultReleaseFailed {
		t.Fatal("signer failure misclassified")
	}
	if e := c.ReleaseFault(context.Background()); FaultReleaseReason(e) != "state_rejected" {
		t.Fatal("failed join rearmed")
	}
}
