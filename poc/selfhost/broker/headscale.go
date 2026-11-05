package broker

import (
	"context"
	"os"
	"path/filepath"
	"remoteapp.local/selfhost-poc/headscale"
	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/verifier"
	"time"
)

// StartHeadscaleNetworkSupervisor is privileged process-local assembly, NOT
// exposed to frontend callers. Runtime must already be live
// and caller must Stop supervisor BEFORE stopping runtime or closing Store.
// The fixed local CLI and runtime-owned overlay never use installed Tailscale.
func StartHeadscaleNetworkSupervisor(ctx context.Context, st *invite.Store, v *verifier.Runtime, policyDir string, interval time.Duration) (*NetworkSupervisor, error) {
	return startLocalHeadscaleNetworkSupervisor(ctx, st, v, policyDir, interval, false)
}

// StartDesktopHeadscaleNetworkSupervisor opts a new trusted server composition
// into its explicitly desktop-enabled Store. This is not a mode field in the
// existing daemon config or frontend IPC; it does not create/upgrade a Store or
// acquire another verifier's ownership. The caller owns the same Stop ordering.
func StartDesktopHeadscaleNetworkSupervisor(ctx context.Context, st *invite.Store, v *verifier.Runtime, policyDir string, interval time.Duration) (*NetworkSupervisor, error) {
	return startLocalHeadscaleNetworkSupervisor(ctx, st, v, policyDir, interval, true)
}

func startLocalHeadscaleNetworkSupervisor(ctx context.Context, st *invite.Store, v *verifier.Runtime, policyDir string, interval time.Duration, desktop bool) (*NetworkSupervisor, error) {
	if v.DesktopTest() {
		return nil, ErrBroker
	}
	return startHeadscaleNetworkSupervisor(ctx, st, v, headscale.LocalRunner(), policyDir, interval, desktop)
}

func startHeadscaleNetworkSupervisor(ctx context.Context, st *invite.Store, v *verifier.Runtime, run headscale.Run, policyDir string, interval time.Duration, desktop bool) (*NetworkSupervisor, error) {
	if ctx == nil || ctx.Err() != nil || st == nil || v == nil || !v.Live() || run == nil {
		return nil, ErrBroker
	}
	if !filepath.IsAbs(policyDir) || filepath.Clean(policyDir) != policyDir {
		return nil, ErrBroker
	}
	real, e := filepath.EvalSymlinks(policyDir)
	info, se := os.Lstat(policyDir)
	if e != nil || real != policyDir || se != nil || !info.IsDir() || info.Mode().Perm() != 0700 {
		return nil, ErrBroker
	}
	d := &headscale.Driver{Run: run, VerifierIP: v.Info().IP, PolicyDir: policyDir, PolicyMode: "database"}
	d.Proof = func(ctx context.Context, r invite.ProvisionRecord, o invite.ProvisionObservation) bool {
		pub, e := st.DevicePublicKey(r.Device)
		return e == nil && v.Verify(ctx, r, o, pub)
	}
	b, e := headscaleBackend(st, d, desktop)
	if e != nil {
		return nil, e
	}
	b.VerifierSource = func(call context.Context) (string, error) {
		info := v.Info()
		if call.Err() != nil || !info.Healthy || !v.Live() {
			return "", invite.ErrDenied
		}
		return info.Key, nil
	}
	return StartNetworkSupervisor(ctx, b, v, interval)
}

// Both public entry points share this exact selection. No backend can infer
// opt-in from a peer grant. Reconcile still validates the default Store mode
// before invoking the privileged driver or changing shared readiness.
func headscaleBackend(st *invite.Store, driver invite.ProvisionDriver, desktop bool) (*invite.ProvisionBackend, error) {
	if desktop {
		return invite.NewDesktopProvisionBackend(st, driver)
	}
	if st == nil || driver == nil {
		return nil, ErrBroker
	}
	return &invite.ProvisionBackend{Store: st, Driver: driver}, nil
}
