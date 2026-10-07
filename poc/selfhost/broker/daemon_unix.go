//go:build linux || darwin

package broker

import (
	"context"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/trustprofile"
	"time"
)

type DaemonConfig struct {
	Profile, ReleaseRoot, Service, StateDirectory, Socket string
	MinimumEpoch                                          uint64
	PeerUID, GroupID                                      uint32
	// Empty retains the deployed device-only behavior. Trusted local path only.
	NetworkConfig string
}

// RunDaemon owns the sole existing Store. It has NO TLS listener or generic
// privileged operations. Network is opt-in via trusted local configuration.
func RunDaemon(ctx context.Context, c DaemonConfig, ready func()) (result error) {
	return runDaemon(ctx, c, ready, startDaemonNetwork)
}

func runDaemon(ctx context.Context, c DaemonConfig, ready func(), start networkStarter) (result error) {
	root, e := hex.DecodeString(c.ReleaseRoot)
	if e != nil || len(root) != 32 || hex.EncodeToString(root) != c.ReleaseRoot || ctx.Err() != nil {
		return ErrBroker
	}
	for _, path := range []string{c.Profile, c.StateDirectory} {
		if !filepath.IsAbs(path) || filepath.Clean(path) != path {
			return ErrBroker
		}
	}
	for _, dir := range []string{filepath.Dir(c.Profile), c.StateDirectory, filepath.Dir(c.StateDirectory)} {
		real, e := filepath.EvalSymlinks(dir)
		if e != nil || real != dir || !safeDirectory(dir, uint32(os.Getegid())) {
			return ErrBroker
		}
	}
	info, e := os.Lstat(c.Profile)
	if e != nil || !info.Mode().IsRegular() || info.Size() > 8192 || info.Mode().Perm()&0022 != 0 {
		return ErrBroker
	}
	f, e := os.Open(c.Profile)
	if e != nil {
		return ErrBroker
	}
	b, e := io.ReadAll(io.LimitReader(f, 8193))
	ce := f.Close()
	if e != nil || ce != nil || len(b) > 8192 {
		return ErrBroker
	}
	p, e := trustprofile.Decode(b)
	if e != nil {
		return ErrBroker
	}
	if _, e = trustprofile.Verify(p, root, c.Service, c.MinimumEpoch, time.Now()); e != nil {
		return ErrBroker
	}
	var networkConfig DaemonNetworkConfig
	if c.NetworkConfig != "" {
		networkConfig, e = loadDaemonNetworkConfig(c)
		if e != nil {
			return ErrBroker
		}
	}
	// Missing state cannot turn into a fresh issuer. Open never creates the
	// authority directory, and no caller-controlled paths enter IPC messages.
	info, e = os.Lstat(filepath.Join(c.StateDirectory, "state.json"))
	if e != nil || !info.Mode().IsRegular() {
		return ErrBroker
	}
	s, e := invite.Open(c.StateDirectory, time.Now)
	if e != nil {
		return ErrBroker
	}
	closeStore := true
	defer func() {
		if closeStore && s.Close() != nil {
			result = ErrBroker
		}
	}()
	if hex.EncodeToString(s.PublicKey()) != p.Claims.Issuer {
		return ErrBroker
	}
	lifetime, cancel := context.WithDeadline(ctx, time.Now().Add(time.Until(time.Unix(p.Claims.ExpiresAt, 0))))
	defer cancel()
	a := &Authority{Store: s}
	var owner daemonNetworkOwner
	if c.NetworkConfig != "" {
		owner, e = start(lifetime, s, networkConfig)
		if owner != nil {
			defer func() {
				stopCtx, stopCancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer stopCancel()
				if owner.Stop(stopCtx) != nil {
					// Never release Store ownership under unjoined network workers.
					closeStore = false
					result = ErrBroker
				}
			}()
		}
		if e != nil || owner == nil || owner.Authority() == nil || owner.Authority().Store != s {
			return ErrBroker
		}
		a = owner.Authority()
		watchDone := make(chan struct{})
		defer func() { cancel(); <-watchDone }()
		go func() {
			defer close(watchDone)
			select {
			case <-owner.Done():
				cancel()
			case <-lifetime.Done():
			}
		}()
	}
	e = Serve(lifetime, Config{Socket: c.Socket, PeerUID: c.PeerUID, GroupID: c.GroupID, Executor: &timeBoundAuthority{a: a, issued: p.Claims.IssuedAt, expires: p.Claims.ExpiresAt}}, ready)
	if owner != nil && ctx.Err() == nil {
		return ErrBroker
	}
	return e
}

type timeBoundAuthority struct {
	a               *Authority
	issued, expires int64
}

func (t *timeBoundAuthority) Execute(ctx context.Context, r Request) Reply {
	now := time.Now().Unix()
	if now < t.issued || now >= t.expires {
		return Reply{Status: 503}
	}
	return t.a.Execute(ctx, r)
}
