package clientruntime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/processowner"
	"remoteapp.local/selfhost-poc/session"
	"remoteapp.local/selfhost-poc/trustinstall"
)

// Scope comes from the compiled random TEST release and already verified grant,
// never a peer/profile-selected certificate namespace or production identity.
func TestCertificateScope(r trustinstall.Release, g invite.Grant) (string, error) {
	if !r.Specification().Test {
		return "", session.ErrDenied
	}
	b, e := hex.DecodeString(g.Claims.ID)
	if e != nil || len(b) != 16 || hex.EncodeToString(b) != g.Claims.ID {
		return "", session.ErrDenied
	}
	h := sha256.Sum256([]byte("remoteapp-test-product-certificate-v1\n" + r.Scope() + "\n" + g.Claims.ID))
	return hex.EncodeToString(h[:]), nil
}
func certificateSpec(s processowner.Spec, scope string, reopen bool) (processowner.Spec, error) {
	if len(s.Args) != 0 {
		return s, session.ErrDenied
	}
	b, e := hex.DecodeString(scope)
	if e != nil || len(b) != 32 || hex.EncodeToString(b) != scope {
		return s, session.ErrDenied
	}
	mode := "--create-test-certificate"
	if reopen {
		mode = "--reopen-test-certificate"
	}
	s.Args = []string{mode, scope}
	return s, nil
}
func certificateMaintenance(ctx context.Context, s processowner.Spec, mode, scope string) error {
	if len(s.Args) != 0 {
		return session.ErrDenied
	}
	if mode == "--cleanup-test-certificate" {
		checked, e := certificateSpec(s, scope, false)
		if e != nil {
			return e
		}
		s = checked
		s.Args[0] = mode
	} else if mode == "--test-certificate-selftest" && scope == "" {
		s.Args = []string{mode}
	} else {
		return session.ErrDenied
	}
	child, e := processowner.Start(s)
	if e != nil {
		return e
	}
	raw, e := child.Exchange(ctx, nil)
	var ready map[string]any
	if e != nil || json.Unmarshal(raw, &ready) != nil || len(ready) != 1 || ready["status"] != "ready" {
		closed := child.Stop(context.Background())
		if closed != nil {
			return closed
		}
		return session.ErrDenied
	}
	// Stop requires the child's exact closed receipt AND actual zero exit/join.
	// This local owner must not lose a child handle on a context deadline.
	// Native maintenance is bounded; completion still requires actual join.
	return child.Stop(context.Background())
}
func ProbeTestCertificateStore(ctx context.Context, s processowner.Spec) error {
	return certificateMaintenance(ctx, s, "--test-certificate-selftest", "")
}
func CleanupTestCertificate(ctx context.Context, s processowner.Spec, scope string) error {
	return certificateMaintenance(ctx, s, "--cleanup-test-certificate", scope)
}
