// Package autonomousvalidation is desktop native isolated acceptance, not GUI/installer.
// Client has NO SSH credential supplier. All device/network calls go to the
// compiled signed HTTPS profile and actual server approval/provision backend.
package autonomousvalidation

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"remoteapp.local/selfhost-poc/clientowner"
	"remoteapp.local/selfhost-poc/clientruntime"
	"remoteapp.local/selfhost-poc/enrollmentstore"
	"remoteapp.local/selfhost-poc/installeddevice"
	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/privatepipe"
	"remoteapp.local/selfhost-poc/trustinstall"
	"remoteapp.local/selfhost-poc/trustprofile"
)

var ErrValidation = errors.New("autonomous_native_validation_unavailable")

func Run(release trustinstall.Release, digest, dir string, pass func(string)) (result error) {
	stage := "profile_prepare"
	defer func() {
		if result != nil && pass != nil {
			pass("diagnostic_" + stage)
		}
	}()
	if (runtime.GOOS != "darwin" && runtime.GOOS != "windows") || !release.Specification().Test || pass == nil {
		return ErrValidation
	}
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	vault := filepath.Join(dir, "identity-vault"+suffix)
	journal := filepath.Join(dir, "trust-journal"+suffix)
	real, e := filepath.EvalSymlinks(vault)
	if e != nil || real != vault {
		return ErrValidation
	}
	b, e := os.ReadFile(vault)
	if e != nil {
		return ErrValidation
	}
	sum := sha256.Sum256(b)
	if hex.EncodeToString(sum[:]) != digest {
		return ErrValidation
	}
	factory, e := release.NativeFactory(journal)
	if e != nil {
		return ErrValidation
	}
	j, e := factory(true)
	if e != nil {
		return ErrValidation
	}
	if _, e = trustprofile.InitializeInstallation(release.Anchor(), release.Specification().Bundled, j, time.Now); e != nil {
		j.Close()
		return ErrValidation
	}
	if j.Close() != nil {
		return ErrValidation
	}
	var raw [16]byte
	if _, e = rand.Read(raw[:]); e != nil {
		return ErrValidation
	}
	vaultScope := hex.EncodeToString(raw[:])
	creator, e := privatepipe.NewSigner(vault, vaultScope, "", true)
	if e != nil {
		return ErrValidation
	}
	// Identity creator survives the borrowed signing process; delete only once
	// owner/network/proof/signer joined. No cleanup on uncertain runner join.
	identityJoined := false
	defer func() {
		if identityJoined && creator.DeleteCreatedIdentity() != nil {
			result = ErrValidation
		}
	}()
	pub := hex.EncodeToString(creator.PublicKey())
	scope, e := enrollmentstore.Scope(release.Scope(), vaultScope, pub)
	if e != nil {
		return ErrValidation
	}
	meta, _ := json.Marshal(struct {
		Version                                             int
		ProfileScope, VaultScope, ExpectedPublic, NodeScope string
	}{1, release.Scope(), vaultScope, pub, scope})
	metaPath := filepath.Join(dir, "public-recovery.json")
	f, e := os.OpenFile(metaPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return ErrValidation
	}
	_, writeErr := f.Write(meta)
	syncErr := f.Sync()
	closeErr := f.Close()
	if writeErr != nil || syncErr != nil || closeErr != nil {
		return ErrValidation
	}
	state, e := enrollmentstore.Initialize(scope, true)
	if e != nil {
		return ErrValidation
	}
	if state.Close() != nil {
		return ErrValidation
	}
	pass("native_profile_identity_and_node_initialized")
	owner, e := clientowner.Open(release, factory, time.Now)
	if e != nil {
		return ErrValidation
	}
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
	defer cancel()
	commands := make(chan installeddevice.Request, 1)
	outputs := make(chan any, 16)
	h, e := owner.Start(ctx, installeddevice.BoundEnrollmentRunner(release, clientruntime.NativeSigner(vault, digest, vaultScope, pub), commands, func(v any) error {
		select {
		case outputs <- v:
			return nil
		case <-ctx.Done():
			return ErrValidation
		}
	}, installeddevice.NativeEnrollmentFactory(scope, true)))
	if e != nil {
		return ErrValidation
	}
	joined := false
	defer func() {
		if !joined {
			cancel()
			h.Wait(context.Background())
			var e error
			if h.State().Fault {
				e = owner.RetireFault(context.Background())
			} else {
				e = owner.Close(context.Background())
			}
			if e != nil {
				result = ErrValidation
			}
		}
	}()
	read := func() (map[string]json.RawMessage, error) {
		select {
		case v := <-outputs:
			b, _ := json.Marshal(v)
			var m map[string]json.RawMessage
			if json.Unmarshal(b, &m) != nil {
				return nil, ErrValidation
			}
			return m, nil
		case <-ctx.Done():
			return nil, ErrValidation
		case <-h.Done():
			return nil, ErrValidation
		}
	}
	m, e := read()
	if e != nil || string(m["status"]) != `"ready"` {
		return ErrValidation
	}
	device := func(body invite.Body) (invite.Result, error) {
		select {
		case commands <- installeddevice.Request{Op: "device", Device: body}:
		case <-ctx.Done():
			return invite.Result{}, ErrValidation
		}
		m, e := read()
		if e != nil || string(m["status"]) != `"device_result"` || string(m["passed"]) != "true" {
			return invite.Result{}, ErrValidation
		}
		var r invite.Result
		if json.Unmarshal(m["device"], &r) != nil {
			return invite.Result{}, ErrValidation
		}
		return r, nil
	}
	stage = "signed_device_approval"
	target, e := device(invite.Body{Action: "register"})
	if e != nil {
		return ErrValidation
	}
	inv, e := device(invite.Body{Action: "invite"})
	if e != nil {
		return ErrValidation
	}
	// Only the claimant role is a local memory-signing fixture. It never creates
	// a node or payload rule. The target is an actual native OS signer/SDK.
	_, peerKey, e := ed25519.GenerateKey(rand.Reader)
	if e != nil {
		return ErrValidation
	}
	peerSigner, e := invite.NewMemorySigner(peerKey)
	if e != nil {
		return ErrValidation
	}
	a := release.Anchor()
	peer, e := invite.NewProfileNetworkClient(release.Specification().Bundled, a.Root[:], a.Service, a.MinimumEpoch, peerSigner)
	if e != nil {
		return ErrValidation
	}
	defer peer.Close()
	if _, e = peer.DeviceRequest(ctx, invite.Body{Action: "register"}); e != nil {
		return ErrValidation
	}
	claimed, e := peer.DeviceRequest(ctx, invite.Body{Action: "claim", TargetCode: target.DeviceCode, Code: inv.Code})
	if e != nil {
		return ErrValidation
	}
	approved, e := device(invite.Body{Action: "approve", RequestID: claimed.RequestID, Allow: true})
	if e != nil || approved.Grant == nil {
		return ErrValidation
	}
	pass("actual_store_signed_device_approval")
	stage = "signed_clock"
	commands <- installeddevice.Request{Op: "network", Network: invite.NetworkRequestBody{Action: "grant_clock", Grant: *approved.Grant}}
	m, e = read()
	if e != nil || string(m["status"]) != `"network_authorization"` || string(m["passed"]) != "true" {
		return ErrValidation
	}
	stage = "real_enrollment_and_binding"
	commands <- installeddevice.Request{Op: "provision", Network: invite.NetworkRequestBody{Grant: *approved.Grant}}
	m, e = read()
	if e != nil || string(m["status"]) != `"network_binding"` || string(m["passed"]) != "true" || string(m["active"]) != "false" {
		return ErrValidation
	}
	pass("native_signed_https_real_sdk_verifier_permit_and_bind")
	commands <- installeddevice.Request{Op: "stop"}
	close(commands)
	<-h.Done()
	if h.Wait(ctx) != nil || !h.State().Joined || h.State().Fault || owner.Close(ctx) != nil {
		return ErrValidation
	}
	joined = true
	identityJoined = true
	stage = "native_join_and_cleanup"
	state, e = enrollmentstore.Open(scope, true)
	if e != nil {
		return ErrValidation
	}
	if state.State().Phase != "registered" || state.Begin() == nil {
		state.Close()
		return ErrValidation
	}
	if state.CleanupTestAfterJoin() != nil {
		state.Close()
		return ErrValidation
	}
	if trustprofile.DeleteTestDesktopJournal(journal, release.Scope()) != nil {
		return ErrValidation
	}
	pass("proof_sdk_signer_profile_joined_and_exact_native_test_cleanup")
	return nil
}
