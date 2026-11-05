//go:build darwin || linux

package broker

import (
	"context"
	"time"

	"remoteapp.local/selfhost-poc/headscale"
	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/verifier"
)

// StartDesktopTestHeadscaleNetworkSupervisor binds desktop TEST policy and
// provisioning to the separately prepared instance and verifier.
func StartDesktopTestHeadscaleNetworkSupervisor(ctx context.Context, st *invite.Store, v *verifier.Runtime, instance *headscale.DesktopTestInstance, interval time.Duration) (*NetworkSupervisor, error) {
	if instance == nil || !v.DesktopTest() {
		return nil, ErrBroker
	}
	return startHeadscaleNetworkSupervisor(ctx, st, v, instance.Run, instance.PolicyDirectory(), interval, true)
}
