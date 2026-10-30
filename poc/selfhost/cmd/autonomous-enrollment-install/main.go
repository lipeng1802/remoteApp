// Offline autonomous identity/node bootstrap. It must run in the logged-in
// user's installer context; it never contacts the authority or starts tsnet.
package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"remoteapp.local/selfhost-poc/autonomousbinding"
	"remoteapp.local/selfhost-poc/clientruntime"
	"remoteapp.local/selfhost-poc/enrollmentstore"
	"remoteapp.local/selfhost-poc/trustinstall"
	"remoteapp.local/selfhost-poc/trustprofile"
)

var embeddedRelease, embeddedSignerDigest string

type reply struct {
	Status          string `json:"status"`
	PublicKey       string `json:"public_key,omitempty"`
	EnrollmentState string `json:"enrollment_state,omitempty"`
	Stage           string `json:"stage,omitempty"`
	Diagnostic      int    `json:"diagnostic_version,omitempty"`
	TrustEpoch      uint64 `json:"trust_epoch,omitempty"`
	Reason          string `json:"reason,omitempty"`
}

type identityProbe func(autonomousbinding.Record) error

func maintenance(stage string) reply {
	return reply{Status: "maintenance_required", Stage: stage}
}

func diagnostic(epoch uint64, value reply) reply {
	value.Diagnostic = 1
	value.TrustEpoch = epoch
	value.PublicKey = ""
	return value
}

func diagnose(releaseScope string, epoch uint64, test bool, records trustinstall.RecordFactory, states trustinstall.RecordState, identity identityProbe) (reply, error) {
	if epoch == 0 || epoch > 1<<53-1 || records == nil || states == nil || identity == nil {
		return reply{}, trustprofile.ErrTrust
	}
	bindingScope, err := autonomousbinding.Scope(releaseScope)
	if err != nil {
		return reply{}, err
	}
	state, err := states(bindingScope, test)
	if err != nil {
		return reply{}, err
	}
	switch state {
	case trustprofile.JournalAbsent:
		return diagnostic(epoch, reply{Status: "not_installed"}), nil
	case trustprofile.JournalPending:
		return diagnostic(epoch, maintenance("binding_pending")), nil
	case trustprofile.JournalUnusable:
		return diagnostic(epoch, maintenance("binding_unusable")), nil
	case trustprofile.JournalCommitted:
	default:
		return reply{}, trustprofile.ErrTrust
	}
	binding, err := autonomousbinding.OpenWithJournal(releaseScope, test, bindingRecords(records))
	if err != nil {
		return diagnostic(epoch, maintenance("binding_unusable")), nil
	}
	record := binding.Record()
	checkErr, closeErr := binding.Check(), binding.Close()
	if checkErr != nil || closeErr != nil {
		return diagnostic(epoch, maintenance("binding_unusable")), nil
	}
	state, err = states(record.EnrollmentScope, test)
	if err != nil {
		return reply{}, err
	}
	switch state {
	case trustprofile.JournalAbsent:
		return diagnostic(epoch, maintenance("enrollment_missing")), nil
	case trustprofile.JournalPending:
		return diagnostic(epoch, maintenance("enrollment_pending")), nil
	case trustprofile.JournalUnusable:
		return diagnostic(epoch, maintenance("enrollment_unusable")), nil
	case trustprofile.JournalCommitted:
	default:
		return reply{}, trustprofile.ErrTrust
	}
	enrollment, err := enrollmentstore.OpenWithJournal(record.EnrollmentScope, test, enrollmentRecords(records))
	if err != nil {
		return diagnostic(epoch, maintenance("enrollment_unusable")), nil
	}
	phase := enrollment.State().Phase
	checkErr, closeErr = enrollment.Check(), enrollment.Close()
	if checkErr != nil || closeErr != nil {
		return diagnostic(epoch, maintenance("enrollment_unusable")), nil
	}
	if identity(record) != nil {
		return diagnostic(epoch, maintenance("identity_unavailable")), nil
	}
	return diagnostic(epoch, reply{Status: "usable", EnrollmentState: phase}), nil
}

func bindingRecords(records trustinstall.RecordFactory) autonomousbinding.JournalFactory {
	return func(scope string, create, test bool) (autonomousbinding.Journal, error) {
		return records(scope, create, test)
	}
}

func enrollmentRecords(records trustinstall.RecordFactory) enrollmentstore.JournalFactory {
	return func(scope string, create, test bool) (enrollmentstore.Journal, error) {
		return records(scope, create, test)
	}
}

func main() {
	value, err := run()
	if err != nil {
		_ = json.NewEncoder(os.Stdout).Encode(failureReply(err))
		os.Exit(1)
	}
	_ = json.NewEncoder(os.Stdout).Encode(value)
}

func installedRelease(release trustinstall.Release, dir, suffix string) (trustprofile.Profile, error) {
	factory, err := release.NativeFactory(filepath.Join(dir, "trust-journal"+suffix))
	if err != nil {
		return trustprofile.Profile{}, err
	}
	owner, err := trustinstall.NewOfflineOwner(release, factory, time.Now)
	if err != nil {
		return trustprofile.Profile{}, err
	}
	return owner.Inspect()
}

func inspect(release trustinstall.Release, records trustinstall.RecordFactory) (reply, error) {
	binding, err := autonomousbinding.OpenWithJournal(release.Scope(), release.Specification().Test, bindingRecords(records))
	if err != nil {
		return reply{}, err
	}
	record := binding.Record()
	checkErr := binding.Check()
	closeErr := binding.Close()
	if checkErr != nil || closeErr != nil {
		return reply{}, autonomousbinding.ErrBinding
	}
	state, err := enrollmentstore.OpenWithJournal(record.EnrollmentScope, release.Specification().Test, enrollmentRecords(records))
	if err != nil {
		return reply{}, err
	}
	phase := state.State().Phase
	checkErr = state.Check()
	closeErr = state.Close()
	if checkErr != nil || closeErr != nil {
		return reply{}, enrollmentstore.ErrStorage
	}
	return reply{Status: "usable", PublicKey: record.PublicKey, EnrollmentState: phase}, nil
}

func initialize(release trustinstall.Release, vault string, records trustinstall.RecordFactory) (value reply, result error) {
	stage := "identity_reservation"
	defer func() {
		if result != nil {
			result = &initializationFailure{stage: stage, cause: result}
		}
	}()
	reservation, err := autonomousbinding.BeginWithJournal(release.Scope(), release.Specification().Test, bindingRecords(records))
	if err != nil {
		return reply{}, err
	}
	reserved := true
	defer func() {
		if reserved {
			_ = reservation.Close()
		}
	}()
	stage = "identity_scope"
	var raw [16]byte
	if _, err = rand.Read(raw[:]); err != nil {
		return reply{}, err
	}
	vaultScope := hex.EncodeToString(raw[:])
	stage = "identity_intent"
	if err = reservation.ReserveIdentity(vaultScope); err != nil {
		return reply{}, err
	}
	stage = "identity_create"
	identity, err := clientruntime.CreateNativeIdentity(vault, embeddedSignerDigest, vaultScope)
	if err != nil {
		return reply{}, err
	}
	public := hex.EncodeToString(identity.PublicKey())
	stage = "enrollment_scope"
	enrollmentScope, err := enrollmentstore.Scope(release.Scope(), vaultScope, public)
	if err != nil {
		_ = identity.DeleteCreatedIdentity()
		return reply{}, err
	}
	stage = "enrollment_initialize"
	state, err := enrollmentstore.InitializeWithJournal(enrollmentScope, release.Specification().Test, enrollmentRecords(records))
	if err != nil {
		_ = identity.DeleteCreatedIdentity()
		return reply{}, err
	}
	checkErr := state.Check()
	phase := state.State().Phase
	closeErr := state.Close()
	if checkErr != nil || phase != "idle" || closeErr != nil {
		_ = identity.CloseJoined()
		return reply{}, enrollmentstore.ErrStorage
	}
	stage = "binding_commit"
	binding, err := reservation.Commit(vaultScope, public)
	if err != nil {
		_ = identity.CloseJoined()
		return reply{}, err
	}
	reserved = false
	stage = "identity_join"
	checkErr = binding.Check()
	recordedScope := binding.Record().EnrollmentScope
	closeErr = binding.Close()
	identityErr := identity.CloseJoined()
	if checkErr != nil || recordedScope != enrollmentScope || closeErr != nil || identityErr != nil {
		return reply{}, autonomousbinding.ErrBinding
	}
	return reply{Status: "usable", PublicKey: public, EnrollmentState: "idle"}, nil
}

func cleanupTest(release trustinstall.Release, records trustinstall.RecordFactory, journalPath, vaultPath string) (reply, error) {
	if !release.Specification().Test {
		return reply{}, trustprofile.ErrTrust
	}
	binding, err := autonomousbinding.OpenWithJournal(release.Scope(), true, bindingRecords(records))
	if err != nil {
		return reply{}, err
	}
	record := binding.Record()
	checkErr := binding.Check()
	closeErr := binding.Close()
	if checkErr != nil || closeErr != nil {
		return reply{}, autonomousbinding.ErrBinding
	}
	state, err := enrollmentstore.OpenWithJournal(record.EnrollmentScope, true, enrollmentRecords(records))
	if err != nil || state.State().Phase != "idle" {
		if state != nil {
			_ = state.Close()
		}
		return reply{}, enrollmentstore.ErrStorage
	}
	cleanup := func(scope string) error { return trustprofile.DeleteTestDesktopJournal(journalPath, scope) }
	if state.CleanupTestAfterJoinWith(cleanup) != nil || clientruntime.DeleteNativeTestIdentity(vaultPath, embeddedSignerDigest, record.VaultScope, record.PublicKey) != nil || autonomousbinding.CleanupTestWith(release.Scope(), cleanup) != nil {
		return reply{}, trustprofile.ErrTrust
	}
	return reply{Status: "cleaned"}, nil
}

// reserveTest simulates interruption at the earliest durable install boundary.
// It is compiled only into an explicitly TEST release and never creates an OS
// identity or enrollment record.
func reserveTest(release trustinstall.Release, records trustinstall.RecordFactory) (reply, error) {
	if !release.Specification().Test {
		return reply{}, trustprofile.ErrTrust
	}
	reservation, err := autonomousbinding.BeginWithJournal(release.Scope(), true, bindingRecords(records))
	if err != nil {
		return reply{}, err
	}
	if err = reservation.Close(); err != nil {
		return reply{}, err
	}
	return maintenance("binding_pending"), nil
}

// cleanupReservationTest removes only the pending marker created by
// reserveTest. Committed, corrupt and production records remain fail-closed.
func cleanupReservationTest(release trustinstall.Release, states trustinstall.RecordState, journalPath string) (reply, error) {
	if !release.Specification().Test || states == nil {
		return reply{}, trustprofile.ErrTrust
	}
	bindingScope, err := autonomousbinding.Scope(release.Scope())
	if err != nil {
		return reply{}, err
	}
	state, err := states(bindingScope, true)
	if err != nil || state != trustprofile.JournalPending {
		return reply{}, trustprofile.ErrTrust
	}
	cleanup := func(scope string) error { return trustprofile.DeleteTestDesktopJournal(journalPath, scope) }
	if err = autonomousbinding.CleanupTestWith(release.Scope(), cleanup); err != nil {
		return reply{}, err
	}
	return reply{Status: "cleaned"}, nil
}

func run() (reply, error) {
	if len(os.Args) != 2 || (os.Args[1] != "initialize" && os.Args[1] != "inspect" && os.Args[1] != "status" && os.Args[1] != "cleanup-test" && os.Args[1] != "reserve-test" && os.Args[1] != "cleanup-reservation-test" && os.Args[1] != "inspect-maintenance-test") {
		return reply{}, trustprofile.ErrTrust
	}
	release, err := trustinstall.DecodeCompiled(embeddedRelease)
	if err != nil {
		return reply{}, err
	}
	exe, err := os.Executable()
	if err != nil {
		return reply{}, err
	}
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	dir := filepath.Dir(exe)
	journalPath := filepath.Join(dir, "trust-journal"+suffix)
	vaultPath := filepath.Join(dir, "identity-vault"+suffix)
	installed, err := installedRelease(release, dir, suffix)
	if err != nil {
		// Exact TEST inspection/teardown remains available for a validated
		// pending marker. Acquiring its journal still rejects a live owner.
		if release.Specification().Test && (os.Args[1] == "cleanup-test" || os.Args[1] == "inspect-maintenance-test") {
			factory, e := release.NativeFactory(journalPath)
			if e != nil {
				return reply{}, e
			}
			offline, e := trustinstall.NewOfflineOwner(release, factory, time.Now)
			if e != nil {
				return reply{}, e
			}
			if _, e = offline.MaintenanceState(); e != nil {
				return reply{}, e
			}
			records, e := release.NativeRecordFactory(journalPath)
			if e != nil {
				return reply{}, e
			}
			if os.Args[1] == "cleanup-test" {
				return cleanupTest(release, records, journalPath, vaultPath)
			}
			value, e := inspect(release, records)
			value.Status = "maintenance_required"
			return value, e
		}
		return reply{}, err
	}
	if os.Args[1] == "inspect-maintenance-test" {
		return reply{}, trustprofile.ErrTrust
	}
	records, err := release.NativeRecordFactory(journalPath)
	if err != nil {
		return reply{}, err
	}
	if os.Args[1] == "inspect" {
		return inspect(release, records)
	}
	if os.Args[1] == "reserve-test" {
		return reserveTest(release, records)
	}
	if os.Args[1] == "status" || os.Args[1] == "cleanup-reservation-test" {
		states, err := release.NativeRecordState(journalPath)
		if err != nil {
			return reply{}, err
		}
		if os.Args[1] == "cleanup-reservation-test" {
			return cleanupReservationTest(release, states, journalPath)
		}
		probe := func(record autonomousbinding.Record) error {
			signer, err := clientruntime.NativeSigner(vaultPath, embeddedSignerDigest, record.VaultScope, record.PublicKey)()
			if err != nil {
				return err
			}
			joined, ok := signer.(interface{ CloseJoined() error })
			if !ok {
				signer.Close()
				return trustprofile.ErrTrust
			}
			return joined.CloseJoined()
		}
		return diagnose(release.Scope(), installed.Claims.Epoch, release.Specification().Test, records, states, probe)
	}
	if os.Args[1] == "cleanup-test" {
		return cleanupTest(release, records, journalPath, vaultPath)
	}
	return initialize(release, vaultPath, records)
}

// CLI diagnostics preserve only fixed local stages, never the wrapped error.
type initializationFailure struct {
	stage string
	cause error
}

func (e *initializationFailure) Error() string { return "identity_initialization_failed" }
func (e *initializationFailure) Unwrap() error { return e.cause }
func failureReply(err error) reply {
	v := reply{Status: "failed", Reason: "installation_rejected"}
	if errors.Is(err, trustprofile.ErrDesktopLogon) {
		v.Reason = "interactive_logon_required"
	}
	var failed *initializationFailure
	if errors.As(err, &failed) {
		switch failed.stage {
		case "identity_intent", "identity_reservation", "identity_scope", "identity_create", "enrollment_scope", "enrollment_initialize", "binding_commit", "identity_join":
			v.Stage = failed.stage
		}
	}
	return v
}
