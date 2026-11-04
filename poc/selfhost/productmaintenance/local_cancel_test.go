package productmaintenance

import (
	"context"
	"remoteapp.local/selfhost-poc/installeddevice"
	"testing"
	"time"
)

func TestLocalCancellationDoesNotRequireOrClaimRemoteRevocation(t *testing.T) {
	c, s, _ := controllerFixture(t, "fault-race")
	c.prepared = false
	c.provisioned = true
	rev := &revokeFixture{unknown: true}
	c.revocation = rev
	if c.CancelLocalFault(context.Background()) != nil || !c.child.JoinedFault() || !c.localCancelJoined || c.faultJoined || s.closes.Load() != 1 || rev.revokes != 0 {
		t.Fatal("local join confused with revocation")
	}
	if c.CancelLocalFault(context.Background()) != nil || s.closes.Load() != 1 {
		t.Fatal("duplicate cancellation")
	}
	if _, e := c.Forward(context.Background(), installeddevice.Request{Op: "quiesce_session"}); e == nil {
		t.Fatal("local cancellation rearmed success")
	}
}
func TestLocalCancellationRetainsSameChildOnTimeout(t *testing.T) {
	c, s, _ := controllerFixture(t, "fault-slow")
	c.prepared = false
	c.provisioned = true
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	if c.CancelLocalFault(ctx) == nil || s.closes.Load() != 0 || !c.broken {
		t.Fatal("timeout released signer")
	}
	if c.ReleaseFault(context.Background()) == nil {
		t.Fatal("cancelled path became remote fault proof")
	}
	if c.CancelLocalFault(context.Background()) != nil || s.closes.Load() != 1 || !c.localCancelJoined {
		t.Fatal("same child could not join")
	}
}
func TestLocalCancellationRejectsGenericFailedOrNormalReceipt(t *testing.T) {
	for _, mode := range []string{"normal", "fault-generic", "fault-extra", "fault-zero"} {
		t.Run(mode, func(t *testing.T) {
			c, s, _ := controllerFixture(t, mode)
			c.prepared = false
			if c.CancelLocalFault(context.Background()) == nil || s.closes.Load() != 0 || c.localCancelJoined {
				t.Fatal("unproven nested cleanup accepted")
			}
		})
	}
}
