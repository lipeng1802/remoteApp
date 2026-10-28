// Package programremoval coordinates explicit TEST-only program-file removal.
// It has no production uninstall, reset, identity deletion or startup API.
package programremoval

import (
	"context"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"remoteapp.local/selfhost-poc/clientowner"
	"remoteapp.local/selfhost-poc/installrelease"
	"remoteapp.local/selfhost-poc/trustinstall"
	"remoteapp.local/selfhost-poc/trustprofile"
)

// RunTest reopens an existing native lease without creating or repairing state.
// root and release must be compiled/trusted caller inputs. Helpers and this
// process must live outside root. This API is deliberately unavailable in a
// production namespace. Failure leaves maintenance latched for explicit retry.
func RunTest(ctx context.Context, r trustinstall.Release, factory trustinstall.Factory, root string) (state trustprofile.MaintenanceState, err error) {
	return runTest(ctx, r, factory, root, nil)
}

// RunExpectedTest explicitly continues the exact transaction observed after
// live-owner join. It never accepts a merely matching epoch from another owner.
func RunExpectedTest(ctx context.Context, r trustinstall.Release, factory trustinstall.Factory, root string, expected trustprofile.MaintenanceState) (trustprofile.MaintenanceState, error) {
	return runTest(ctx, r, factory, root, &expected)
}

func runTest(ctx context.Context, r trustinstall.Release, factory trustinstall.Factory, root string, expected *trustprofile.MaintenanceState) (state trustprofile.MaintenanceState, err error) {
	if ctx == nil || ctx.Err() != nil || !r.Specification().Test || factory == nil || validateRemovalRoot(root) != nil {
		return state, trustprofile.ErrTrust
	}
	j, e := factory(false)
	if e != nil {
		return state, e
	}
	defer func() {
		if c := j.Close(); err == nil {
			err = c
		}
	}()
	tx, e := trustprofile.OpenRemovalTransaction(r.Anchor(), j, time.Now)
	if e != nil {
		return state, e
	}
	if expected != nil {
		actual, e := tx.State()
		id, e2 := hex.DecodeString(expected.Transaction)
		if e != nil || e2 != nil || len(id) != 16 || hex.EncodeToString(id) != expected.Transaction || actual.Transaction != expected.Transaction || actual.Epoch != expected.Epoch || (expected.PlanSHA256 != "" && actual.PlanSHA256 != expected.PlanSHA256) {
			return state, trustprofile.ErrTrust
		}
	}
	return continueTest(ctx, r, root, tx)
}

func validateRemovalRoot(root string) error {
	if !filepath.IsAbs(root) || filepath.Clean(root) != root {
		return trustprofile.ErrTrust
	}
	exe, e := os.Executable()
	if e != nil {
		return trustprofile.ErrTrust
	}
	exe, e = filepath.EvalSymlinks(exe)
	if e != nil {
		return trustprofile.ErrTrust
	}
	realRoot, e := filepath.EvalSymlinks(root)
	if e != nil {
		return trustprofile.ErrTrust
	}
	rel, e := filepath.Rel(realRoot, exe)
	if e == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return trustprofile.ErrTrust
	}
	return nil
}

// QuiesceAndRemoveTest joins an existing trusted in-process owner, observes its
// durable transaction while leased, closes only after join, then reacquires a
// new lease with that exact expectation. On join/close error no removal occurs.
// ErrRemovalReceipt requires explicit RunExpectedTest, never a second live join.
// An independent uninstaller still needs authenticated IPC to this live owner.
func QuiesceAndRemoveTest(ctx context.Context, r trustinstall.Release, factory trustinstall.Factory, root string, owner *clientowner.Owner) (expected, result trustprofile.MaintenanceState, err error) {
	if ctx == nil || ctx.Err() != nil || !r.Specification().Test || factory == nil || owner == nil || validateRemovalRoot(root) != nil {
		return expected, result, trustprofile.ErrTrust
	}
	if _, err = installrelease.ValidateProtectedBootstrapLayout(root, true); err != nil {
		return expected, result, err
	}
	err = owner.MaintainDurableState(ctx, r, func(_ context.Context, state trustprofile.MaintenanceState) error { expected = state; return nil })
	if err != nil {
		return expected, result, err
	}
	if err = owner.Close(ctx); err != nil {
		return expected, result, err
	}
	result, err = RunExpectedTest(ctx, r, factory, root, expected)
	return expected, result, err
}

func continueTest(ctx context.Context, r trustinstall.Release, root string, tx *trustprofile.RemovalTransaction) (trustprofile.MaintenanceState, error) {
	s, err := tx.State()
	if err != nil {
		return s, err
	}
	spec := r.Specification()
	public, _ := hex.DecodeString(spec.Root)
	p, digest, loadErr := installrelease.LoadRemovalTest(root, s.PlanSHA256, true)
	if s.State == "pending" {
		if loadErr != nil {
			// Only an absent checkpoint permits capture. Corrupt/torn/existing
			// plans cannot be overwritten or silently recaptured.
			if _, e := os.Lstat(filepath.Join(root, installrelease.RemovalPlanName)); !os.IsNotExist(e) {
				return s, installrelease.ErrRelease
			}
			p, err = installrelease.CaptureRemovalTest(root, public, spec.Channel, runtime.GOOS, s.Transaction, s.Epoch, true)
			if err != nil {
				return s, err
			}
			if err = installrelease.PersistRemovalTest(root, p, true); err != nil {
				return s, err
			}
			p, digest, err = installrelease.LoadRemovalTest(root, "", true)
			if err != nil {
				return s, err
			}
		}
		if p.Transaction != s.Transaction || p.Epoch != s.Epoch || installrelease.ValidateRemovalTest(root, p, public, spec.Channel, runtime.GOOS, false, false, true) != nil {
			return s, installrelease.ErrRelease
		}
		if ctx.Err() != nil {
			return s, ctx.Err()
		}
		if err = tx.BindRemovalPlan(digest); err != nil {
			return s, err
		}
		s, err = tx.State()
		if err != nil {
			return s, err
		}
	} else if loadErr != nil {
		return s, loadErr
	}
	if p.Transaction != s.Transaction || p.Epoch != s.Epoch || digest != s.PlanSHA256 {
		return s, installrelease.ErrRelease
	}
	if s.State == "files_removed" {
		return s, installrelease.ValidateRemovalTest(root, p, public, spec.Channel, runtime.GOOS, true, true, true)
	}
	if s.State != "deleting" {
		return s, trustprofile.ErrTrust
	}
	done, err := installrelease.RemoveProgramBatchTest(ctx, root, p, public, spec.Channel, runtime.GOOS, 256, true)
	if err != nil {
		return s, err
	}
	if !done {
		return s, installrelease.ErrRelease
	}
	if err = tx.MarkFilesRemoved(digest); err != nil {
		return s, err
	}
	return tx.State()
}
