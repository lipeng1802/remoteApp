package devicevalidation

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	"remoteapp.local/selfhost-poc/installeddevice"
	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/privatepipe"
	"remoteapp.local/selfhost-poc/trustinstall"
	"remoteapp.local/selfhost-poc/trustprofile"
)

type child struct {
	cmd     *exec.Cmd
	in      io.WriteCloser
	out     io.ReadCloser
	read    *privatepipe.Reader
	encode  *json.Encoder
	timer   *time.Timer
	expired atomic.Bool
	waited  bool
}

func launch(executable string) (*child, error) {
	c := &child{cmd: exec.Command(executable)}
	c.cmd.Stderr = io.Discard
	// Runtime ignores poisoned environment; it cannot supply profile/floor.
	c.cmd.Env = append(os.Environ(), "REMOTEAPP_RELEASE_ROOT=invalid", "REMOTEAPP_ENDPOINT=https://127.0.0.1:1", "REMOTEAPP_MINIMUM_EPOCH=0")
	var err error
	c.in, err = c.cmd.StdinPipe()
	if err != nil {
		return nil, ErrValidation
	}
	c.out, err = c.cmd.StdoutPipe()
	if err != nil {
		c.in.Close()
		return nil, ErrValidation
	}
	if c.cmd.Start() != nil {
		c.in.Close()
		c.out.Close()
		return nil, ErrValidation
	}
	c.read = privatepipe.NewReader(c.out)
	c.encode = json.NewEncoder(c.in)
	// Timeout closes only the dedicated input. Wait must observe actual exit;
	// no Process.Kill, canceled context or timeout counts as a successful join.
	c.timer = time.AfterFunc(30*time.Second, func() { c.expired.Store(true); c.in.Close() })
	return c, nil
}

func (c *child) receipt(status string) (map[string]any, error) {
	var value map[string]any
	if c.read.Read(&value) != nil || value["status"] != status {
		return nil, ErrValidation
	}
	return value, nil
}

func (c *child) wait(exit int) error {
	if c.waited {
		return ErrValidation
	}
	c.waited = true
	c.in.Close()
	err := c.cmd.Wait()
	c.timer.Stop()
	c.out.Close()
	if c.expired.Load() || c.cmd.ProcessState.ExitCode() != exit || (exit == 0 && err != nil) {
		return ErrValidation
	}
	return nil
}

func (c *child) cleanup() {
	if !c.waited {
		c.in.Close()
		c.wait(1)
	}
}

func (c *child) stop() error {
	if c.encode.Encode(installeddevice.Request{Op: "stop"}) != nil {
		return ErrValidation
	}
	// Stop is terminal. Half-close this dedicated command pipe before waiting
	// for closed, so the child's inherited-stdin reader can actually join.
	// EOF does not stand in for closed, native signer join or actual exit0.
	c.in.Close()
	value, err := c.receipt("closed")
	if err != nil || len(value) != 1 || c.wait(0) != nil {
		return ErrValidation
	}
	return nil
}

func pinned(path, digest string) error {
	real, err := filepath.EvalSymlinks(path)
	if err != nil || real != path {
		return ErrValidation
	}
	b, err := os.ReadFile(path)
	if err != nil || len(b) == 0 || len(b) > 100<<20 {
		return ErrValidation
	}
	h := sha256.Sum256(b)
	if hex.EncodeToString(h[:]) != digest {
		return ErrValidation
	}
	return nil
}

// Run uses isolated compiled TEST state only. The authority is a loopback
// signature fixture, not a real admission Store, grant or network backend.
func Run(release trustinstall.Release, fixture TLSFixture, signerDigest, clientDigest, dir string, pass func(string)) (result error) {
	pair, err := fixturePair(release, fixture)
	if err != nil {
		return err
	}
	u, _ := url.Parse(release.Specification().Bundled.Claims.Endpoint)
	if u.Hostname() != "127.0.0.1" || u.Port() == "" || u.Port() == "443" {
		return ErrValidation
	}
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	journal := filepath.Join(dir, "trust-journal"+suffix)
	vault := filepath.Join(dir, "identity-vault"+suffix)
	executable := filepath.Join(dir, "installed-device-client"+suffix)
	if pinned(vault, signerDigest) != nil || pinned(executable, clientDigest) != nil {
		return ErrValidation
	}
	factory, err := release.NativeFactory(journal)
	if err != nil {
		return ErrValidation
	}
	offline, err := trustinstall.NewOfflineOwner(release, factory, time.Now)
	if err != nil {
		return ErrValidation
	}
	var raw [16]byte
	if _, err = rand.Read(raw[:]); err != nil {
		return ErrValidation
	}
	scope := hex.EncodeToString(raw[:])
	creator, err := privatepipe.NewSigner(vault, scope, "", true)
	if err != nil {
		return ErrValidation
	}
	defer func() {
		if creator.DeleteCreatedIdentity() != nil {
			result = ErrValidation
		} else {
			pass("exact_random_os_test_identity_deleted_and_joined")
		}
	}()
	public := hex.EncodeToString(creator.PublicKey())
	open := installeddevice.Request{Op: "open", VaultScope: scope, ExpectedPublic: public}
	// Missing installation is not automatically initialized by the new entry.
	missing, err := launch(executable)
	if err != nil {
		return err
	}
	defer missing.cleanup()
	// Missing journal may reject before reading stdin; an early closed pipe
	// is allowed only when the strict failed frame and actual exit1 follow.
	missing.encode.Encode(open)
	value, err := missing.receipt("failed")
	if err != nil || len(value) != 1 || missing.wait(1) != nil {
		return ErrValidation
	}
	pass("missing_native_installation_rejected_without_repair")
	if _, err = offline.Initialize(); err != nil {
		return ErrValidation
	}
	defer func() {
		if trustprofile.DeleteTestDesktopJournal(journal, release.Scope()) != nil {
			result = ErrValidation
		} else {
			pass("exact_isolated_native_test_journal_deleted_and_joined")
		}
	}()
	bad, err := launch(executable)
	if err != nil {
		return err
	}
	defer bad.cleanup()
	if bad.encode.Encode(map[string]any{"Op": "open", "VaultScope": scope, "ExpectedPublic": public, "Endpoint": "https://127.0.0.1:1"}) != nil {
		return ErrValidation
	}
	value, err = bad.receipt("failed")
	if err != nil || len(value) != 1 || bad.wait(1) != nil {
		return ErrValidation
	}
	if p, err := offline.Inspect(); err != nil || p.Claims.Epoch != 1 {
		return ErrValidation
	}
	pass("runtime_endpoint_injection_rejected_native_epoch_preserved")
	v := &verifier{public: creator.PublicKey(), nonces: make(map[string]bool)}
	l, err := net.Listen("tcp", u.Host)
	if err != nil {
		return ErrValidation
	}
	server := &http.Server{Handler: invite.NewAdmission().Wrap(v), ReadHeaderTimeout: 2 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second, IdleTimeout: 5 * time.Second, ErrorLog: log.New(io.Discard, "", 0), TLSConfig: &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{pair}, NextProtos: []string{"http/1.1"}}}
	server.SetKeepAlivesEnabled(false)
	done := make(chan error, 1)
	go func() { done <- server.Serve(tls.NewListener(l, server.TLSConfig)) }()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if server.Shutdown(ctx) != nil {
			server.Close()
			result = ErrValidation
		}
		if err := <-done; err != http.ErrServerClosed {
			result = ErrValidation
		}
	}()
	var firstID, firstCode string
	for round := 0; round < 2; round++ {
		c, err := launch(executable)
		if err != nil {
			return err
		}
		defer c.cleanup()
		if c.encode.Encode(open) != nil {
			return ErrValidation
		}
		value, err = c.receipt("ready")
		if err != nil || len(value) != 1 {
			return ErrValidation
		}
		if _, err = offline.Inspect(); err == nil {
			return ErrValidation
		}
		pass("actual_installed_client_holds_exclusive_native_lease")
		if c.encode.Encode(installeddevice.Request{Op: "device", Device: invite.Body{Action: "register"}}) != nil {
			return ErrValidation
		}
		value, err = c.receipt("device_result")
		if err != nil || value["passed"] != true || len(value) != 3 {
			v.mu.Lock()
			stage := v.last
			v.mu.Unlock()
			if stage == "" {
				stage = "no_http_request"
			}
			pass("diagnostic_registration_" + stage)
			return ErrValidation
		}
		device, ok := value["device"].(map[string]any)
		if !ok {
			return ErrValidation
		}
		id, ok := device["device_id"].(string)
		if !ok || id == "" {
			return ErrValidation
		}
		code, ok := device["device_code"].(string)
		if !ok || code == "" {
			return ErrValidation
		}
		pass("actual_native_registration_signature_verified")
		if round == 0 {
			firstID, firstCode = id, code
			if c.encode.Encode(installeddevice.Request{Op: "device", Device: invite.Body{Action: "invite"}}) != nil {
				return ErrValidation
			}
			value, err = c.receipt("device_result")
			if err != nil || value["passed"] != true {
				v.mu.Lock()
				stage := v.last
				v.mu.Unlock()
				pass("diagnostic_invitation_" + stage)
				return ErrValidation
			}
			device, ok = value["device"].(map[string]any)
			if !ok || device["code"] != "native-fixture-invite" {
				pass("diagnostic_invitation_result_shape")
				return ErrValidation
			}
			pass("actual_native_invitation_signature_verified")
		} else if id != firstID || code != firstCode {
			return ErrValidation
		}
		if c.stop() != nil {
			if c.expired.Load() {
				pass("diagnostic_client_deadline_elapsed")
			} else {
				pass("diagnostic_client_closed_or_actual_exit_unconfirmed")
			}
			return ErrValidation
		}
		if p, err := offline.Inspect(); err != nil || p.Claims.Epoch != 1 {
			return ErrValidation
		}
		pass("actual_client_closed_zero_exit_signer_join_and_native_reopen")
	}
	v.mu.Lock()
	registers, invites := v.registers, v.invites
	v.mu.Unlock()
	if registers != 2 || invites != 1 {
		return ErrValidation
	}
	pass("actual_os_signature_https_registration_invitation_and_stable_restart")
	// A structurally valid local network command with an invalid issuer grant
	// must retire this real native client, not fall back to device retry behavior.
	// It intentionally does not exercise online clock/state proof acceptance.
	fault, err := launch(executable)
	if err != nil {
		return err
	}
	defer fault.cleanup()
	if fault.encode.Encode(open) != nil {
		return ErrValidation
	}
	if v, e := fault.receipt("ready"); e != nil || len(v) != 1 {
		return ErrValidation
	}
	if _, e := offline.Inspect(); e == nil {
		return ErrValidation
	}
	id := sha256.Sum256(creator.PublicKey())
	grant := invite.Grant{Claims: invite.Claims{Version: 1, ID: strings.Repeat("a", 32), Target: hex.EncodeToString(id[:]), Controller: strings.Repeat("b", 64), Permission: "fixed-payload", IssuedAt: time.Now().Unix() - 1, ExpiresAt: time.Now().Unix() + 299}, Signature: strings.Repeat("0", 128)}
	if fault.encode.Encode(installeddevice.Request{Op: "network", Network: invite.NetworkRequestBody{Action: "grant_clock", Grant: grant}}) != nil {
		return ErrValidation
	}
	value, err = fault.receipt("network_authorization")
	if err != nil || len(value) != 4 || value["passed"] != false || value["active"] != false {
		return ErrValidation
	}
	fault.in.Close() // terminal result -> half-close FIRST, then wait final/exit
	value, err = fault.receipt("failed")
	if err != nil || len(value) != 1 || fault.wait(1) != nil {
		return ErrValidation
	}
	if p, e := offline.Inspect(); e != nil || p.Claims.Epoch != 1 {
		return ErrValidation
	}
	pass("actual_native_network_denial_input_eof_failed_exit_and_lease_reopen")
	return nil
}
