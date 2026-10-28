package clientowner

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/trustinstall"
	"remoteapp.local/selfhost-poc/trustprofile"
)

func durableNativeFixture(t *testing.T) (trustinstall.Release, trustprofile.Profile, trustinstall.Factory) {
	t.Helper()
	helper := os.Getenv("REMOTEAPP_JOURNAL_TEST_EXECUTABLE")
	if helper == "" {
		t.Skip("native helper required")
	}
	helper, err := filepath.EvalSymlinks(helper)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(helper)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(raw)
	release, next := fixture(t, hex.EncodeToString(digest[:]))
	factory, err := release.NativeFactory(helper)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := trustprofile.DeleteTestDesktopJournal(helper, release.Scope()); err != nil {
			t.Error("native TEST cleanup", err)
		}
	})
	offline, err := trustinstall.NewOfflineOwner(release, factory, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = offline.Initialize(); err != nil {
		t.Fatal(err)
	}
	if _, err = offline.Upgrade(context.Background(), next); err != nil {
		t.Fatal(err)
	}
	return release, next, factory
}

func TestNativeDurableMaintenanceSurvivesClose(t *testing.T) {
	release, next, factory := durableNativeFixture(t)
	o, err := Open(release, factory, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close(context.Background())
	h := mustStart(t, o, func(ctx context.Context, _ trustprofile.Profile) error { <-ctx.Done(); return nil })
	var recorded trustprofile.MaintenanceState
	if err = o.MaintainDurable(context.Background(), func(context.Context, uint64) error {
		if !h.State().Joined {
			t.Fatal("marker before join")
		}
		var e error
		recorded, e = trustprofile.ReadMaintenance(release.Anchor(), o.journal, time.Now)
		return e
	}); err != nil {
		t.Fatal(err)
	}
	if err = o.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertNativePending(t, release, next, factory, recorded.Transaction)
	t.Log("CHECK joined durable pending survives explicit close; fresh owner/init/upgrade denied; epoch 2 retained PASS")
}

func TestNativeDurableMaintenanceUncertainReceipt(t *testing.T) {
	release, next, factory := durableNativeFixture(t)
	wrapped := func(create bool) (trustinstall.Journal, error) {
		j, e := factory(create)
		if e != nil {
			return nil, e
		}
		return &uncertainNativeCommit{Journal: j, armed: true}, nil
	}
	o, err := Open(release, wrapped, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close(context.Background())
	if o.MaintainDurable(context.Background(), func(context.Context, uint64) error { t.Fatal("action after lost receipt"); return nil }) == nil {
		t.Fatal("uncertain commit accepted")
	}
	if competing, e := factory(false); e == nil {
		competing.Close()
		t.Fatal("lease released on uncertain receipt")
	}
	if err = o.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertNativePending(t, release, next, factory, "")
	t.Log("CHECK real native pending commit + simulated lost acknowledgement denies action and preserves blocking state PASS")
}

type crashMaintenanceReply struct {
	Status      string
	Transaction string
	Epoch       uint64
}

// TEST-only subprocess fixture. Release data is public and must identify a
// separate TEST namespace. Exit(77) deliberately bypasses all owner defers.
func TestCrashOwnerFixture(t *testing.T) {
	encoded := os.Getenv("REMOTEAPP_TEST_CRASH_RELEASE")
	if encoded == "" {
		return
	}
	release, err := trustinstall.DecodeCompiled(encoded)
	if err != nil || !release.Specification().Test {
		t.Fatal("invalid TEST release")
	}
	factory, err := release.NativeFactory(os.Getenv("REMOTEAPP_JOURNAL_TEST_EXECUTABLE"))
	if err != nil {
		t.Fatal(err)
	}
	o, err := Open(release, factory, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close(context.Background())
	// Crash from inside the cancelled runner, before it has joined. This
	// proves intent must be durable before join, not merely before ready.
	_ = mustStart(t, o, func(ctx context.Context, _ trustprofile.Profile) error {
		<-ctx.Done()
		state, e := trustprofile.ReadMaintenance(release.Anchor(), o.journal, time.Now)
		if e != nil {
			return e
		}
		if e = json.NewEncoder(os.Stdout).Encode(crashMaintenanceReply{"maintenance_committed", state.Transaction, state.Epoch}); e != nil {
			return e
		}
		os.Exit(77)
		return nil
	})
	err = o.MaintainDurable(context.Background(), func(context.Context, uint64) error { t.Fatal("action ran before crashed join"); return nil })
	t.Fatal("crash fixture returned", err)
}

func TestNativeDurableMaintenanceCrashAndReopen(t *testing.T) {
	release, next, factory := durableNativeFixture(t)
	encoded, err := trustinstall.EncodeForTrustedBuild(release.Specification(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestCrashOwnerFixture$")
	cmd.Env = append(os.Environ(), "REMOTEAPP_TEST_CRASH_RELEASE="+encoded)
	cmd.Stderr = io.Discard
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	line, readErr := bufio.NewReader(out).ReadBytes('\n')
	exitErr := cmd.Wait()
	var exited *exec.ExitError
	if readErr != nil || !errors.As(exitErr, &exited) || exited.ExitCode() != 77 {
		t.Fatal("did not observe actual fixture crash", exitErr)
	}
	var reply crashMaintenanceReply
	if json.Unmarshal(line, &reply) != nil || reply.Status != "maintenance_committed" || reply.Epoch != 2 || len(reply.Transaction) != 32 {
		t.Fatal("invalid crash checkpoint")
	}
	// Wait only for the orphan native helper to observe pipe EOF and release
	// its lease. These probes never create/reset/commit any native record.
	deadline := time.Now().Add(5 * time.Second)
	for {
		j, e := factory(false)
		if e == nil {
			if e = j.Close(); e != nil {
				t.Fatal(e)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("orphan native helper did not release lease")
		}
		time.Sleep(20 * time.Millisecond)
	}
	assertNativePending(t, release, next, factory, reply.Transaction)
	t.Log("CHECK actual exit 77 without owner defers + helper EOF + native pending survives process crash and fences new owner PASS")
}

func assertNativePending(t *testing.T, release trustinstall.Release, next trustprofile.Profile, factory trustinstall.Factory, transaction string) {
	t.Helper()
	j, err := factory(false)
	if err != nil {
		t.Fatal(err)
	}
	before, err := j.Load()
	if err != nil {
		j.Close()
		t.Fatal(err)
	}
	state, err := trustprofile.ReadMaintenance(release.Anchor(), j, time.Now)
	closeErr := j.Close()
	if err != nil || closeErr != nil || state.State != "pending" || state.Epoch != 2 || len(state.Transaction) != 32 || (transaction != "" && transaction != state.Transaction) {
		t.Fatal("native pending not preserved", err, closeErr)
	}
	if fresh, e := Open(release, factory, time.Now); e == nil {
		fresh.Close(context.Background())
		t.Fatal("new owner bypassed native pending")
	} else if !errors.Is(e, trustprofile.ErrMaintenance) {
		t.Fatal("wrong startup rejection", e)
	}
	offline, err := trustinstall.NewOfflineOwner(release, factory, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = offline.Inspect(); !errors.Is(err, trustprofile.ErrMaintenance) {
		t.Fatal("offline inspection emitted usable profile", err)
	}
	if _, err = offline.Initialize(); err == nil {
		t.Fatal("reinstall erased pending")
	}
	if _, err = offline.Upgrade(context.Background(), next); err == nil {
		t.Fatal("upgrade bypassed pending")
	}
	j, err = factory(false)
	if err != nil {
		t.Fatal(err)
	}
	after, err := j.Load()
	closeErr = j.Close()
	if err != nil || closeErr != nil || !bytes.Equal(before, after) {
		t.Fatal(fmt.Sprint("rejected startup mutated pending: ", err, closeErr))
	}
}
