package installeddevice

import (
	"context"

	"remoteapp.local/selfhost-poc/clientruntime"
	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/nodeidentity"
)

// NativeEnrollmentValidationFactory is an explicitly TEST-only composition.
// It cannot open production storage and never calls Headscale/SDK Up. consume
// checks a one-time fixture credential inside this process, not parent IPC.
func NativeEnrollmentValidationFactory(scope string, consume func(context.Context, string) error) EnrollmentFactory {
	return protectedEnrollmentFactory(scope, true, func(dir string) EnrollmentFactory {
		return func(ctx context.Context) (EnrollmentNode, error) {
			if consume == nil || ctx.Err() != nil {
				return nil, invite.ErrDenied
			}
			pub, e := nodeidentity.Prepare(dir, clientruntime.ControlURL)
			if e != nil {
				return nil, invite.ErrDenied
			}
			return &validationEnrollmentNode{key: pub, consume: consume}, nil
		}
	})
}

type validationEnrollmentNode struct {
	key     string
	consume func(context.Context, string) error
	used    bool
}

func (n *validationEnrollmentNode) PublicKey() string { return n.key }
func (n *validationEnrollmentNode) Register(ctx context.Context, s string) (string, string, error) {
	if n.used || ctx.Err() != nil {
		return "", "", invite.ErrDenied
	}
	n.used = true
	if n.consume(ctx, s) != nil || ctx.Err() != nil {
		return "", "", invite.ErrDenied
	}
	return n.key, "100.120.0.7", nil // fixture observation, NOT live registration
}
func (n *validationEnrollmentNode) CloseJoined() error { return nil } // no SDK workers were started
