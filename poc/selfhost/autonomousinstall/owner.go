// Package autonomousinstall is an offline, trusted installation boundary.
// It never enrolls, resets a failed store, or starts product/network workers.
package autonomousinstall

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"time"

	"remoteapp.local/selfhost-poc/enrollmentstore"
	"remoteapp.local/selfhost-poc/trustinstall"
	"remoteapp.local/selfhost-poc/trustprofile"
)

type Identity interface {
	PublicKey() ed25519.PublicKey
	CloseJoined() error
}
type IdentityFactory func() (Identity, error)
type node interface {
	Check() error
	Close() error
}
type nodeFactory func(bool) (node, error)

type Owner struct {
	release  trustinstall.Release
	journal  trustinstall.Factory
	identity IdentityFactory
	public   string
	node     nodeFactory
	clock    func() time.Time
}

// New requires a previously explicitly provisioned OS identity, pinned by the
// trusted installer. Neither a failed Inspect nor Upgrade falls back to create.
// Same-goroutine calls/close preserve Windows native mutex thread ownership.
func New(r trustinstall.Release, journal trustinstall.Factory, identity IdentityFactory,
	vaultScope, public string, clock func() time.Time) (*Owner, error) {
	if journal == nil || identity == nil || clock == nil {
		return nil, trustprofile.ErrTrust
	}
	encoded, err := trustinstall.EncodeForTrustedBuild(r.Specification(), clock())
	if err != nil {
		return nil, err
	}
	r, err = trustinstall.DecodeCompiled(encoded)
	if err != nil {
		return nil, err
	}
	scope, err := enrollmentstore.Scope(r.Scope(), vaultScope, public)
	if err != nil {
		return nil, err
	}
	o := &Owner{release: r, journal: journal, identity: identity, public: public, clock: clock}
	o.node = func(create bool) (node, error) {
		if create {
			return enrollmentstore.Initialize(scope, r.Specification().Test)
		}
		return enrollmentstore.Open(scope, r.Specification().Test)
	}
	return o, nil
}

func (o *Owner) Initialize(ctx context.Context) error { return o.run(ctx, "initialize") }
func (o *Owner) Upgrade(ctx context.Context) error    { return o.run(ctx, "upgrade") }
func (o *Owner) Inspect(ctx context.Context) error    { return o.run(ctx, "inspect") }

func (o *Owner) run(ctx context.Context, op string) (result error) {
	if ctx == nil || ctx.Err() != nil {
		return trustprofile.ErrTrust
	}
	signer, err := o.identity()
	if err != nil {
		return err
	}
	if signer == nil {
		return trustprofile.ErrTrust
	}
	defer func() {
		if err := signer.CloseJoined(); err != nil {
			result = err
		}
	}()
	if hex.EncodeToString(signer.PublicKey()) != o.public {
		return trustprofile.ErrTrust
	}
	// Acquire trust BEFORE node, matching the installed client's lock order.
	j, err := o.journal(op == "initialize")
	if err != nil {
		return err
	}
	defer func() {
		if err := j.Close(); err != nil {
			result = err
		}
	}()
	var installed *trustprofile.Installation
	if op != "initialize" {
		installed, err = trustprofile.OpenInstallation(o.release.Anchor(), j, o.clock)
		if err != nil {
			return err
		}
	}
	n, err := o.node(op == "initialize")
	if err != nil {
		return err
	}
	defer func() {
		if err := n.Close(); err != nil {
			result = err
		}
	}()
	if n.Check() != nil || ctx.Err() != nil {
		return trustprofile.ErrTrust
	}
	if op == "initialize" {
		// Trust is committed LAST. Partial first installs cannot start a client.
		// Preserve all partial evidence; retry never deletes/recreates the node.
		installed, err = trustprofile.InitializeInstallation(o.release.Anchor(), o.release.Specification().Bundled, j, o.clock)
	} else if op == "upgrade" {
		// Both exclusive leases are held; there can be no live installed session.
		err = installed.Accept(ctx, o.release.Specification().Bundled, offline{})
	}
	if err != nil {
		return err
	}
	p, _, err := installed.Current()
	if err != nil || p != o.release.Specification().Bundled || n.Check() != nil {
		return trustprofile.ErrTrust
	}
	return nil
}

type offline struct{}

func (offline) Quiesce(ctx context.Context) error { return ctx.Err() }
