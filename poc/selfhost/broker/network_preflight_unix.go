//go:build linux || darwin

package broker

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"remoteapp.local/selfhost-poc/trustprofile"
	"remoteapp.local/selfhost-poc/verifier"
)

// CheckNetworkPreparation validates an idle deployment snapshot without opening
// the Store, taking locks, starting an overlay or executing control-plane CLI.
// It is NOT readiness, issuer matching, policy-mode or network-denial evidence.
// Startup must revalidate everything and obtain its own exclusive ownership.
func CheckNetworkPreparation(c DaemonConfig) error {
	n, err := loadDaemonNetworkConfig(c)
	if err != nil || !privateOwnedPath(c.StateDirectory, true) || !privateOwnedPath(filepath.Dir(c.Profile), true) || !privateOwnedPath(c.Profile, false) {
		return ErrBroker
	}
	if !filepath.IsAbs(c.Socket) || filepath.Clean(c.Socket) != c.Socket {
		return ErrBroker
	}
	for dir, gid := range map[string]uint32{filepath.Dir(c.StateDirectory): uint32(os.Getegid()), filepath.Dir(c.Socket): c.GroupID} {
		real, err := filepath.EvalSymlinks(dir)
		if err != nil || real != dir || !safeDirectory(dir, gid) {
			return ErrBroker
		}
	}
	root, err := hex.DecodeString(c.ReleaseRoot)
	if err != nil || len(root) != 32 || hex.EncodeToString(root) != c.ReleaseRoot {
		return ErrBroker
	}
	b, err := os.ReadFile(c.Profile)
	if err != nil {
		return ErrBroker
	}
	p, err := trustprofile.Decode(b)
	if err != nil {
		return ErrBroker
	}
	if _, err = trustprofile.Verify(p, root, c.Service, c.MinimumEpoch, time.Now()); err != nil {
		return ErrBroker
	}
	// No automatic lock removal, even if its process appears to have exited.
	for _, lock := range []string{filepath.Join(c.StateDirectory, "writer.lock"), filepath.Join(n.VerifierDirectory, "owner.lock")} {
		if _, err := os.Lstat(lock); !os.IsNotExist(err) {
			return ErrBroker
		}
	}
	if !privateOwnedPath(filepath.Join(n.VerifierDirectory, "node"), true) {
		return ErrBroker
	}
	for _, file := range []string{filepath.Join(c.StateDirectory, "state.json"), filepath.Join(n.VerifierDirectory, "journal.json"), filepath.Join(n.VerifierDirectory, "node", "tailscaled.state")} {
		actual, err := filepath.EvalSymlinks(file)
		i, se := os.Lstat(file)
		if err != nil || se != nil || actual != file || !i.Mode().IsRegular() || i.Mode().Perm() != 0600 || i.Size() == 0 {
			return ErrBroker
		}
		st, ok := i.Sys().(*syscall.Stat_t)
		if !ok || st.Uid != uint32(os.Geteuid()) {
			return ErrBroker
		}
	}
	if verifier.CheckResumeJournal(n.VerifierDirectory) != nil {
		return ErrBroker
	}
	return nil
}
