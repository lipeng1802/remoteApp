package programremoval

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"time"

	"remoteapp.local/selfhost-poc/autonomousbinding"
	"remoteapp.local/selfhost-poc/installrelease"
	"remoteapp.local/selfhost-poc/localmaintenance"
	"remoteapp.local/selfhost-poc/trustinstall"
	"remoteapp.local/selfhost-poc/trustprofile"
)

var ErrArmedCancelled = errors.New("armed_removal_cancelled")

type BoundSigner interface {
	localmaintenance.Signer
	CloseJoined() error
}
type OpenBoundSigner func(autonomousbinding.Record) (BoundSigner, error)

// AuthenticatedBoundTest never initializes a journal or accepts an IPC-selected
// identity. root/address/release are compiled TEST inputs. The existing native
// binding alone selects the signer, and its inspection lease closes before the
// authenticated request can join the live owner. No deletion precedes actual
// signer join and the independently reacquired exact maintenance transaction.
func AuthenticatedBoundTest(ctx context.Context, r trustinstall.Release, factory trustinstall.Factory, records autonomousbinding.JournalFactory, root, address string, open OpenBoundSigner) (result trustprofile.MaintenanceState, err error) {
	return authenticatedBoundTest(ctx, r, factory, records, root, address, open, func(context.Context) error { return nil }, false)
}

// ArmedAuthenticatedBoundTest selects the immutable native binding and opens
// its dedicated signer BEFORE the payload takes the binding lease. await is
// trusted private-parent wiring. It grants no authority and caches no signed
// permission; authentication uses a fresh challenge only after await succeeds.
func ArmedAuthenticatedBoundTest(ctx context.Context, r trustinstall.Release, factory trustinstall.Factory, records autonomousbinding.JournalFactory, root, address string, open OpenBoundSigner, await func(context.Context) error) (trustprofile.MaintenanceState, error) {
	if await == nil {
		return trustprofile.MaintenanceState{}, trustprofile.ErrTrust
	}
	return authenticatedBoundTest(ctx, r, factory, records, root, address, open, await, false)
}

// ArmedVerifiedRemovalTest additionally proves native identity/profile/floor
// and activation preservation before returning files_removed.
func ArmedVerifiedRemovalTest(ctx context.Context, r trustinstall.Release, factory trustinstall.Factory, records autonomousbinding.JournalFactory, root, address string, open OpenBoundSigner, await func(context.Context) error) (trustprofile.MaintenanceState, error) {
	if await == nil {
		return trustprofile.MaintenanceState{}, trustprofile.ErrTrust
	}
	return authenticatedBoundTest(ctx, r, factory, records, root, address, open, await, true)
}
func authenticatedBoundTest(ctx context.Context, r trustinstall.Release, factory trustinstall.Factory, records autonomousbinding.JournalFactory, root, address string, open OpenBoundSigner, await func(context.Context) error, verified bool) (result trustprofile.MaintenanceState, err error) {
	if ctx == nil || ctx.Err() != nil || !r.Specification().Test || factory == nil || records == nil || open == nil {
		return result, trustprofile.ErrTrust
	}
	life, cancel := context.WithDeadline(ctx, time.Unix(r.Specification().Bundled.Claims.ExpiresAt, 0))
	defer cancel()
	ctx = life
	anchor := r.Anchor()
	fresh := func() error {
		_, e := trustprofile.Verify(r.Specification().Bundled, anchor.Root[:], r.Specification().Service, r.Specification().MinimumEpoch, time.Now())
		return e
	}
	if fresh() != nil || validateRemovalRoot(root) != nil {
		return result, trustprofile.ErrTrust
	}
	if _, e := installrelease.ValidateProtectedBootstrapLayout(root, true); e != nil {
		return result, e
	}
	var snapshot preservationSnapshot
	if verified {
		var e error
		snapshot, e = capturePreservation(r, factory, records, false)
		if e != nil {
			return result, e
		}
	}
	var original autonomousbinding.Record
	boundOpen := func(record autonomousbinding.Record) (BoundSigner, error) { original = record; return open(record) }
	signer, e := openBoundRemovalSigner(r, records, boundOpen)
	if e != nil {
		return result, e
	}
	defer func() {
		if signer != nil {
			if e := signer.CloseJoined(); e != nil {
				err = trustprofile.ErrTrust
			}
		}
	}()
	awaited := await(ctx)
	if awaited == ErrArmedCancelled && ctx.Err() == nil && fresh() == nil {
		return result, ErrArmedCancelled
	}
	if awaited != nil || ctx.Err() != nil || fresh() != nil {
		return result, trustprofile.ErrTrust
	}
	public := append([]byte(nil), signer.PublicKey()...)
	expected, e := localmaintenance.RequestTest(ctx, r, address, public, signer)
	if e != nil {
		return result, e
	}
	closeSigner := signer
	signer = nil // an uncertain join must not be retried as success
	if closeSigner.CloseJoined() != nil || fresh() != nil || ctx.Err() != nil {
		return expected, trustprofile.ErrTrust
	}
	result, err = RunExpectedTest(ctx, r, factory, root, expected)
	if err != nil || !verified {
		return result, err
	}
	after, e := capturePreservation(r, factory, records, true)
	if e != nil || after != snapshot {
		return result, trustprofile.ErrTrust
	}
	probe, e := openBoundRemovalSigner(r, records, func(record autonomousbinding.Record) (BoundSigner, error) {
		if record != original {
			return nil, trustprofile.ErrTrust
		}
		return open(record)
	})
	if e != nil {
		return result, e
	}
	if probe.CloseJoined() != nil {
		return result, trustprofile.ErrTrust
	}
	return result, nil
}

func openBoundRemovalSigner(r trustinstall.Release, records autonomousbinding.JournalFactory, open OpenBoundSigner) (BoundSigner, error) {
	if !r.Specification().Test || records == nil || open == nil {
		return nil, trustprofile.ErrTrust
	}
	binding, e := autonomousbinding.OpenWithJournal(r.Scope(), true, records)
	if e != nil {
		return nil, e
	}
	if e = binding.Check(); e != nil {
		binding.Close()
		return nil, e
	}
	record := binding.Record()
	if e = binding.Close(); e != nil {
		return nil, e
	}
	signer, e := open(record)
	if e != nil || signer == nil {
		if signer != nil {
			signer.CloseJoined()
		}
		return nil, trustprofile.ErrTrust
	}
	public, e := hex.DecodeString(record.PublicKey)
	if e != nil || len(public) != 32 || !bytes.Equal(public, signer.PublicKey()) {
		signer.CloseJoined()
		return nil, trustprofile.ErrTrust
	}
	return signer, nil
}
