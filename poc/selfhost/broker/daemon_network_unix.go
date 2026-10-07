//go:build linux || darwin

package broker

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"remoteapp.local/selfhost-poc/headscale"
	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/privatepipe"
	"remoteapp.local/selfhost-poc/verifier"
)

// Trusted process-local configuration, never accepted through HTTP or IPC.
// The CLI, control URL, database policy mode and maintenance budget are fixed.
type DaemonNetworkConfig struct {
	Version           int    `json:"version"`
	VerifierDirectory string `json:"verifier_directory"`
	PolicyDirectory   string `json:"policy_directory"`
}

func privateOwnedPath(path string, directory bool) bool {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return false
	}
	real, e := filepath.EvalSymlinks(path)
	info, se := os.Lstat(path)
	if e != nil || real != path || se != nil {
		return false
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || st.Uid != uint32(os.Geteuid()) {
		return false
	}
	if directory {
		return info.IsDir() && info.Mode().Perm() == 0700
	}
	return info.Mode().IsRegular() && info.Mode().Perm() == 0600 && info.Size() <= 8191
}

func loadDaemonNetworkConfig(c DaemonConfig) (DaemonNetworkConfig, error) {
	var n DaemonNetworkConfig
	if !privateOwnedPath(c.NetworkConfig, false) || !privateOwnedPath(filepath.Dir(c.NetworkConfig), true) {
		return n, ErrBroker
	}
	f, e := os.Open(c.NetworkConfig)
	if e != nil {
		return n, ErrBroker
	}
	b, e := io.ReadAll(io.LimitReader(f, 8192))
	ce := f.Close()
	if e != nil || ce != nil || len(b) > 8191 {
		return n, ErrBroker
	}
	r := privatepipe.NewReader(bytes.NewReader(append(b, '\n')))
	var extra struct{}
	if r.Read(&n) != nil || r.Read(&extra) != io.EOF || n.Version != 1 {
		return n, ErrBroker
	}
	if !privateOwnedPath(n.VerifierDirectory, true) || !privateOwnedPath(n.PolicyDirectory, true) {
		return n, ErrBroker
	}
	// Separate ownership domains: policy writes must not overwrite Store,
	// verifier journal/state, configuration, profile or broker socket parent.
	paths := []string{n.VerifierDirectory, n.PolicyDirectory, c.StateDirectory, filepath.Dir(c.NetworkConfig), filepath.Dir(c.Profile), filepath.Dir(c.Socket)}
	for i, a := range paths {
		for j, b := range paths {
			if i >= j {
				continue
			}
			// Profile and network config may share their trusted config directory.
			if i == 3 && j == 4 && a == b {
				continue
			}
			if a == b || strings.HasPrefix(a, b+string(os.PathSeparator)) || strings.HasPrefix(b, a+string(os.PathSeparator)) {
				return n, ErrBroker
			}
		}
	}
	return n, nil
}

type daemonNetworkOwner interface {
	Authority() *Authority
	Done() <-chan struct{}
	Stop(context.Context) error
}
type networkStarter func(context.Context, *invite.Store, DaemonNetworkConfig) (daemonNetworkOwner, error)
type daemonNetwork struct {
	runtime    *verifier.Runtime
	supervisor *NetworkSupervisor
}

func startDaemonNetwork(ctx context.Context, st *invite.Store, c DaemonNetworkConfig) (daemonNetworkOwner, error) {
	// No creation, registration credential or rekey fallback in service startup.
	// Parent shutdown must first join supervisor/broker workers. Runtime owns
	// its own health cancellation; Open's existing 30s startup budget remains.
	v, e := verifier.Open(context.WithoutCancel(ctx), c.VerifierDirectory, false, &headscale.Driver{Run: headscale.LocalRunner()}, verifier.NewOverlay)
	if e != nil {
		return nil, ErrBroker
	}
	n := &daemonNetwork{runtime: v}
	if ctx.Err() != nil {
		return n, ErrBroker
	}
	s, e := StartHeadscaleNetworkSupervisor(ctx, st, v, c.PolicyDirectory, time.Second)
	if e != nil {
		return n, ErrBroker
	}
	n.supervisor = s
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()
	tick := time.NewTicker(25 * time.Millisecond)
	defer tick.Stop()
	for !s.Ready() {
		select {
		case <-ctx.Done():
			return n, ErrBroker
		case <-s.Done():
			return n, ErrBroker
		case <-timer.C:
			return n, ErrBroker
		case <-tick.C:
		}
	}
	return n, nil
}
func (n *daemonNetwork) Authority() *Authority { return n.supervisor.Authority() }
func (n *daemonNetwork) Done() <-chan struct{} { return n.supervisor.Done() }
func (n *daemonNetwork) Stop(ctx context.Context) error {
	withdrawn := true
	if n.supervisor != nil {
		if n.supervisor.Stop(ctx) != nil {
			return ErrBroker
		}
		withdrawn = n.supervisor.DenyAllConfirmed()
	}
	// Stop, not Retire: keep the exact journal and identity for explicit reopen.
	if n.runtime.Stop(ctx) != nil || !withdrawn {
		return ErrBroker
	}
	return nil
}
