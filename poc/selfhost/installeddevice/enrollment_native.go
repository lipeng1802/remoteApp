package installeddevice

import (
	"context"

	"remoteapp.local/selfhost-poc/enrollmentstore"
	"remoteapp.local/selfhost-poc/invite"
)

// NativeEnrollmentFactory only reopens an explicitly provisioned fixed OS
// namespace. Scope comes from enrollmentstore.Scope(trusted channel, local
// identity), never a grant/path/epoch. Call and Close on the Runner goroutine.
// The compiled entry is still gated on dedicated native process acceptance.
func NativeEnrollmentFactory(scope string, test bool) EnrollmentFactory {
	return func(ctx context.Context) (EnrollmentNode, error) {
		if ctx.Err() != nil {
			return nil, invite.ErrDenied
		}
		s, err := enrollmentstore.Open(scope, test)
		if err != nil {
			return nil, invite.ErrDenied
		}
		if s.Begin() != nil || ctx.Err() != nil {
			s.Close()
			return nil, invite.ErrDenied
		}
		n, err := TSNetEnrollmentFactory(s.NodeDir())(ctx)
		if err != nil || n == nil {
			if n != nil {
				return &nativeEnrollment{node: n, store: s, key: n.PublicKey()}, invite.ErrDenied
			}
			s.Close()
			return nil, invite.ErrDenied
		}
		return &nativeEnrollment{node: n, store: s, key: n.PublicKey()}, nil
	}
}

type nativeEnrollment struct {
	node  EnrollmentNode
	store *enrollmentstore.Store
	key   string
}

func (n *nativeEnrollment) PublicKey() string { return n.node.PublicKey() }
func (n *nativeEnrollment) Register(ctx context.Context, secret string) (string, string, error) {
	if n.store.Check() != nil || n.node.PublicKey() != n.key {
		return "", "", invite.ErrDenied
	}
	k, ip, err := n.node.Register(ctx, secret)
	if err != nil || ctx.Err() != nil || k != n.key || n.node.PublicKey() != n.key || !nodeKeyOK(k) || !overlayOK(ip) || n.store.Registered(k, ip) != nil {
		return "", "", invite.ErrDenied
	}
	return k, ip, nil
}
func (n *nativeEnrollment) CloseJoined() error {
	if n.node.CloseJoined() != nil {
		return invite.ErrDenied
	} // retain native lease on uncertain node join
	return n.store.Close()
}
