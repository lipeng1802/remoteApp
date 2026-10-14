package installeddevice

import (
	"context"
	"os"
	"path/filepath"
	"runtime"

	"remoteapp.local/selfhost-poc/clientruntime"
	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/nodeidentity"
	"remoteapp.local/selfhost-poc/nodeproof"
	"tailscale.com/tsnet"
)

// TSNetEnrollmentFactory is trusted composition only. Caller must hold an
// exclusive native owner lease covering dir and provision its OS ACL BEFORE
// calling. It must not derive dir from IPC, a peer, or an environment override.
// This adapter does not claim that a POSIX mode check provisions Windows ACLs.
// State is retained on stop/unknown outcomes; server reconciliation must precede
// any explicit fresh authorization. The compiled device entry does not enable it.
func TSNetEnrollmentFactory(dir string) EnrollmentFactory {
	return tsNetEnrollmentFactory(dir, clientruntime.ControlURL)
}

func tsNetEnrollmentFactory(dir, controlURL string) EnrollmentFactory {
	return func(ctx context.Context) (EnrollmentNode, error) {
		if ctx.Err() != nil || !filepath.IsAbs(dir) || filepath.Clean(dir) != dir {
			return nil, invite.ErrDenied
		}
		st, err := os.Lstat(dir)
		if err != nil || !st.IsDir() || st.Mode()&os.ModeSymlink != 0 || (runtime.GOOS != "windows" && st.Mode().Perm()&0077 != 0) {
			return nil, invite.ErrDenied
		}
		// Fresh-only adapter: never reuse a logged-in node that could make Up
		// ignore the new auth key. Any retained material (including an unknown
		// first outcome) blocks this factory until explicit trusted recovery.
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) != 0 {
			return nil, invite.ErrDenied
		}
		pub, err := nodeidentity.Prepare(dir, controlURL)
		if err != nil || ctx.Err() != nil {
			return nil, invite.ErrDenied
		}
		return &tsnetEnrollment{key: pub, node: &tsnet.Server{Dir: dir, Hostname: "remoteapp-enrollment", ControlURL: controlURL, Logf: func(string, ...any) {}, UserLogf: func(string, ...any) {}}}, nil
	}
}

type tsnetEnrollment struct {
	key     string
	node    *tsnet.Server
	started bool
	proof   *nodeproof.ProofOwner
}

func (n *tsnetEnrollment) PublicKey() string { return n.key }

func (n *tsnetEnrollment) Register(ctx context.Context, secret string) (string, string, error) {
	if n.started || ctx.Err() != nil || secret == "" || len(secret) > 256 {
		return "", "", invite.ErrDenied
	}
	n.started = true
	nodeidentity.ConfigureOnline() // helper-only; never in the GUI process
	n.node.AuthKey = secret
	defer func() { n.node.AuthKey = "" }()
	st, err := n.node.Up(ctx)
	if err != nil || ctx.Err() != nil || st == nil || st.Self == nil || st.Self.PublicKey.String() != n.key {
		return "", "", invite.ErrDenied
	}
	address := ""
	for _, ip := range st.TailscaleIPs {
		if ip.Is4() {
			if address != "" || !overlayOK(ip.String()) {
				return "", "", invite.ErrDenied
			}
			address = ip.String()
		}
	}
	if address == "" {
		return "", "", invite.ErrDenied
	}
	return n.key, address, nil
}

func (n *tsnetEnrollment) CloseJoined() error {
	var proofErr error
	if n.proof != nil {
		proofErr = n.proof.CloseJoined()
	}
	if !n.started {
		return nil
	}
	// Synchronous SDK close is completion, not merely a cancellation request.
	err := n.node.Close()
	n.node.AuthKey = ""
	if proofErr != nil {
		return proofErr
	}
	return err
}

func (n *tsnetEnrollment) StartProof(ctx context.Context, s invite.DeviceSigner, p invite.VerifierPermit) error {
	if !n.started || n.proof != nil || p.Claims.NodeKey != n.key {
		return invite.ErrDenied
	}
	var err error
	n.proof, err = nodeproof.StartPermitted(ctx, n.node, s, p.Claims)
	return err
}
func (n *tsnetEnrollment) ProofReady() bool { return n.proof != nil && n.proof.Ready() }
