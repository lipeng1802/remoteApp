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
	Reason          string `json:"reason,omitempty"`
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
		reason := "installation_rejected"
		if errors.Is(err, trustprofile.ErrDesktopLogon) {
			reason = "interactive_logon_required"
		}
		_ = json.NewEncoder(os.Stdout).Encode(reply{Status: "failed", Reason: reason})
		os.Exit(1)
	}
	_ = json.NewEncoder(os.Stdout).Encode(value)
}

func installedRelease(release trustinstall.Release, dir, suffix string) error {
	factory, err := release.NativeFactory(filepath.Join(dir, "trust-journal"+suffix))
	if err != nil {
		return err
	}
	owner, err := trustinstall.NewOfflineOwner(release, factory, time.Now)
	if err != nil {
		return err
	}
	_, err = owner.Inspect()
	return err
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

func initialize(release trustinstall.Release, vault string, records trustinstall.RecordFactory) (reply, error) {
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
	var raw [16]byte
	if _, err = rand.Read(raw[:]); err != nil {
		return reply{}, err
	}
	vaultScope := hex.EncodeToString(raw[:])
	identity, err := clientruntime.CreateNativeIdentity(vault, embeddedSignerDigest, vaultScope)
	if err != nil {
		return reply{}, err
	}
	public := hex.EncodeToString(identity.PublicKey())
	enrollmentScope, err := enrollmentstore.Scope(release.Scope(), vaultScope, public)
	if err != nil {
		_ = identity.DeleteCreatedIdentity()
		return reply{}, err
	}
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
	binding, err := reservation.Commit(vaultScope, public)
	if err != nil {
		_ = identity.CloseJoined()
		return reply{}, err
	}
	reserved = false
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

func run() (reply, error) {
	if len(os.Args) != 2 || (os.Args[1] != "initialize" && os.Args[1] != "inspect" && os.Args[1] != "cleanup-test") {
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
	if err = installedRelease(release, dir, suffix); err != nil {
		return reply{}, err
	}
	records, err := release.NativeRecordFactory(journalPath)
	if err != nil {
		return reply{}, err
	}
	if os.Args[1] == "inspect" {
		return inspect(release, records)
	}
	if os.Args[1] == "cleanup-test" {
		return cleanupTest(release, records, journalPath, vaultPath)
	}
	return initialize(release, vaultPath, records)
}
