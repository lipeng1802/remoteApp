package verifier

import (
	"context"
	"remoteapp.local/selfhost-poc/nodeidentity"
	"tailscale.com/ipn/ipnstate"
	"tailscale.com/tsnet"
)

type overlay struct{ s *tsnet.Server }

// NewOverlay must run in a dedicated process: ConfigureOnline changes only
// this process's inherited environment, not the installed system Tailscale.
func NewOverlay(dir, key string) Node {
	nodeidentity.ConfigureOnline()
	return &overlay{&tsnet.Server{Dir: dir, Hostname: "remoteapp-verifier", ControlURL: Control, Logf: func(string, ...any) {}, UserLogf: func(string, ...any) {}}}
}
func status(st *ipnstate.Status) Info {
	if st == nil || st.Self == nil {
		return Info{}
	}
	var ip string
	for _, a := range st.TailscaleIPs {
		if a.Is4() {
			if ip != "" {
				return Info{}
			}
			ip = a.String()
		}
	}
	return Info{st.Self.PublicKey.String(), ip, st.BackendState == "Running" && len(st.Health) == 0}
}
func (n *overlay) Up(ctx context.Context, secret string) (Info, error) {
	n.s.AuthKey = secret
	st, e := n.s.Up(ctx)
	n.s.AuthKey = ""
	if e != nil {
		return Info{}, ErrVerifier
	}
	return status(st), nil
}
func (n *overlay) Status(ctx context.Context) (Info, error) {
	lc, e := n.s.LocalClient()
	if e != nil {
		return Info{}, ErrVerifier
	}
	st, e := lc.StatusWithoutPeers(ctx)
	if e != nil {
		return Info{}, ErrVerifier
	}
	return status(st), nil
}
func (n *overlay) Close() error { return n.s.Close() }
