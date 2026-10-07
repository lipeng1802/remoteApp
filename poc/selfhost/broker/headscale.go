package broker

import (
	"context"
	"remoteapp.local/selfhost-poc/headscale"
	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/verifier"
	"time"
)

// StartHeadscaleNetworkSupervisor is privileged process-local assembly, NOT
// exposed by the device-only daemon or frontend. Runtime must already be live
// and caller must Stop supervisor BEFORE stopping runtime or closing Store.
// The fixed local CLI and runtime-owned overlay never use installed Tailscale.
func StartHeadscaleNetworkSupervisor(ctx context.Context, st *invite.Store, v *verifier.Runtime, interval time.Duration) (*NetworkSupervisor, error) {
	if st == nil || v == nil || !v.Live() {
		return nil, ErrBroker
	}
	d := &headscale.Driver{Run: headscale.LocalRunner(), VerifierIP: v.Info().IP}
	d.Proof = func(ctx context.Context, r invite.ProvisionRecord, o invite.ProvisionObservation) bool {
		pub, e := st.DevicePublicKey(r.Device)
		return e == nil && v.Verify(ctx, r, o, pub)
	}
	return StartNetworkSupervisor(ctx, &invite.ProvisionBackend{Store: st, Driver: d}, v, interval)
}
