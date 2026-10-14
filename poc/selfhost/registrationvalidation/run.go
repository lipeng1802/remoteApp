// Package registrationvalidation composes real OS signers/journals with a
// signed loopback HTTPS authority and an explicitly offline node substitute.
package registrationvalidation

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"remoteapp.local/selfhost-poc/clientowner"
	"remoteapp.local/selfhost-poc/clientruntime"
	"remoteapp.local/selfhost-poc/devicevalidation"
	"remoteapp.local/selfhost-poc/enrollmentstore"
	"remoteapp.local/selfhost-poc/installeddevice"
	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/nativejournal"
	"remoteapp.local/selfhost-poc/privatepipe"
	"remoteapp.local/selfhost-poc/trustinstall"
	"remoteapp.local/selfhost-poc/trustprofile"
)

var ErrValidation = errors.New("native_registration_combination_failed")

func sign(k ed25519.PrivateKey, prefix string, v any) string {
	b, _ := json.Marshal(v)
	return hex.EncodeToString(ed25519.Sign(k, append([]byte(prefix), b...)))
}

type authority struct {
	mu            sync.Mutex
	issuer        ed25519.PrivateKey
	public        ed25519.PublicKey
	grant         invite.Grant
	secret        string
	unknown       bool
	nonces        map[string]bool
	clocks, mints int
}

func (a *authority) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.TLS == nil || r.Method != "POST" || r.URL.Path != "/v1/network" || r.URL.RawQuery != "" {
		w.WriteHeader(400)
		return
	}
	e, err := invite.DecodeNetworkRequest(http.MaxBytesReader(w, r.Body, 8192))
	if err != nil {
		w.WriteHeader(400)
		return
	}
	pub, _ := hex.DecodeString(e.PublicKey)
	sig, _ := hex.DecodeString(e.Signature)
	b, _ := json.Marshal(e.Body)
	a.mu.Lock()
	defer a.mu.Unlock()
	nonce, nerr := hex.DecodeString(e.Body.Nonce)
	if e.PublicKey != hex.EncodeToString(a.public) || !ed25519.Verify(pub, append([]byte("remoteapp-network-api-v1\nPOST /v1/network\n"), b...), sig) || e.Body.Grant != a.grant || nerr != nil || len(nonce) != 16 || hex.EncodeToString(nonce) != e.Body.Nonce || a.nonces[e.Body.Nonce] || e.Body.IssuedAt < time.Now().Unix()-30 || e.Body.IssuedAt > time.Now().Unix()+30 {
		w.WriteHeader(403)
		return
	}
	a.nonces[e.Body.Nonce] = true
	var reply invite.NetworkReply
	switch e.Body.Action {
	case "grant_clock":
		challenge, err := hex.DecodeString(e.Body.Challenge)
		if err != nil || len(challenge) != 32 || hex.EncodeToString(challenge) != e.Body.Challenge {
			w.WriteHeader(403)
			return
		}
		a.clocks++
		p := invite.ClockProof{Claims: invite.ClockClaims{Version: 1, GrantID: a.grant.Claims.ID, Nonce: e.Body.Challenge, IssuedAt: time.Now().UnixNano()}}
		p.Signature = sign(a.issuer, "remoteapp-clock-proof-v1\n", p.Claims)
		reply.Proof = &p
	case "enroll":
		i := e.Body.Intent
		raw, _ := json.Marshal(i.Body)
		signature, _ := hex.DecodeString(i.Signature)
		if i.PublicKey != e.PublicKey || i.Body.GrantID != a.grant.Claims.ID || len(i.Body.NodeKey) != 72 || !ed25519.Verify(a.public, append([]byte("remoteapp-provision-intent-v1\n"), raw...), signature) {
			w.WriteHeader(403)
			return
		}
		a.mints++
		if a.unknown {
			w.WriteHeader(503)
			return
		} // side effect fixture, no credential receipt
		reply.Ticket = strings.Repeat("d", 32)
		reply.Credential = &invite.RegistrationCredential{UserID: "fixture-user", KeyID: "fixture-key", Secret: a.secret}
	default:
		w.WriteHeader(403)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(reply)
}

func Run(release trustinstall.Release, fixture devicevalidation.TLSFixture, signerDigest, dir string, pass func(string)) (result error) {
	stage := "profile_and_fixture"
	defer func() {
		if result != nil && pass != nil {
			pass("diagnostic_setup_" + stage)
		}
	}()
	spec := release.Specification()
	if !spec.Test || !strings.HasPrefix(spec.Channel, "device-validation-") || pass == nil {
		return ErrValidation
	}
	pair, err := tls.X509KeyPair([]byte(fixture.Certificate), []byte(fixture.Key))
	if err != nil {
		return ErrValidation
	}
	issuer, ok := pair.PrivateKey.(ed25519.PrivateKey)
	if !ok || hex.EncodeToString(issuer.Public().(ed25519.PublicKey)) != spec.Bundled.Claims.Issuer {
		return ErrValidation
	}
	anchor := release.Anchor()
	ca, err := trustprofile.Verify(spec.Bundled, anchor.Root[:], anchor.Service, anchor.MinimumEpoch, time.Now())
	if err != nil {
		return ErrValidation
	}
	cert, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		return ErrValidation
	}
	root, err := x509.ParseCertificate(ca)
	if err != nil {
		return ErrValidation
	}
	roots := x509.NewCertPool()
	roots.AddCert(root)
	if _, err := cert.Verify(x509.VerifyOptions{Roots: roots, DNSName: "127.0.0.1"}); err != nil {
		return ErrValidation
	}
	u, _ := url.Parse(spec.Bundled.Claims.Endpoint)
	if u.Hostname() != "127.0.0.1" || u.Port() == "" || u.Port() == "443" {
		return ErrValidation
	}
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	journalPath := filepath.Join(dir, "trust-journal"+suffix)
	vault := filepath.Join(dir, "identity-vault"+suffix)
	real, e := filepath.EvalSymlinks(vault)
	if e != nil || real != vault {
		return ErrValidation
	}
	b, e := os.ReadFile(vault)
	if e != nil || len(b) == 0 || len(b) > 100<<20 {
		return ErrValidation
	}
	sum := sha256.Sum256(b)
	if hex.EncodeToString(sum[:]) != signerDigest {
		return ErrValidation
	}
	factory, err := release.NativeFactory(journalPath)
	if err != nil {
		return ErrValidation
	}
	offline, err := trustinstall.NewOfflineOwner(release, factory, time.Now)
	if err != nil {
		return ErrValidation
	}
	stage = "native_profile_create"
	profileJournal, err := factory(true)
	if err != nil {
		return ErrValidation
	}
	stage = "native_profile_commit"
	installed, err := trustprofile.InitializeInstallation(anchor, spec.Bundled, profileJournal, time.Now)
	if err != nil {
		profileJournal.Close()
		return ErrValidation
	}
	if _, _, err = installed.Current(); err != nil {
		profileJournal.Close()
		return ErrValidation
	}
	stage = "native_profile_close"
	if profileJournal.Close() != nil {
		return ErrValidation
	}
	defer func() {
		if trustprofile.DeleteTestDesktopJournal(journalPath, release.Scope()) != nil {
			result = ErrValidation
		}
		if _, e := offline.Inspect(); e == nil {
			pass("diagnostic_cleanup_profile_still_readable")
			result = ErrValidation
		}
		runtime.LockOSThread()
		empty, e := nativejournal.TestEmpty(release.Scope())
		runtime.UnlockOSThread()
		if e != nil || !empty {
			pass("diagnostic_cleanup_profile_namespace_not_empty")
			result = ErrValidation
		}
	}()
	for _, unknown := range []bool{false, true} {
		stage = "native_case"
		if runCase(release, factory, vault, signerDigest, pair, u.Host, issuer, unknown, pass) != nil {
			return ErrValidation
		}
	}
	return nil
}

func runCase(release trustinstall.Release, factory trustinstall.Factory, vault, digest string, pair tls.Certificate, host string, issuer ed25519.PrivateKey, unknown bool, pass func(string)) (result error) {
	stage := "identity_create"
	defer func() {
		if result != nil {
			pass("diagnostic_case_" + stage)
		}
	}()
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return ErrValidation
	}
	vaultScope := hex.EncodeToString(random[:])
	creator, err := privatepipe.NewSigner(vault, vaultScope, "", true)
	if err != nil {
		return ErrValidation
	}
	defer func() {
		if creator.DeleteCreatedIdentity() != nil {
			result = ErrValidation
		}
	}()
	pub := hex.EncodeToString(creator.PublicKey())
	scope, err := enrollmentstore.Scope(release.Scope(), vaultScope, pub)
	if err != nil {
		return ErrValidation
	}
	s, err := enrollmentstore.Initialize(scope, true)
	if err != nil {
		return ErrValidation
	}
	stage = "storage_initialized"
	if s.Close() != nil {
		return ErrValidation
	}
	// Cleanup is allowed only after each Runner below has actually joined.
	defer func() {
		s, e := enrollmentstore.Open(scope, true)
		if e != nil {
			result = ErrValidation
			return
		}
		if s.CleanupTestAfterJoin() != nil {
			s.Close()
			result = ErrValidation
		}
	}()
	var seed [32]byte
	if _, err = rand.Read(seed[:]); err != nil {
		return ErrValidation
	}
	secret := hex.EncodeToString(seed[:])
	expected := sha256.Sum256([]byte(secret))
	now := time.Now().Unix()
	id := sha256.Sum256(creator.PublicKey())
	grant := invite.Grant{Claims: invite.Claims{Version: 1, ID: hex.EncodeToString(random[:]), Target: hex.EncodeToString(id[:]), Controller: strings.Repeat("b", 64), Permission: "fixed-payload", IssuedAt: now - 1, ExpiresAt: now + 299}}
	grant.Signature = sign(issuer, "remoteapp-invite-grant-v1\n", grant.Claims)
	a := &authority{issuer: issuer, public: creator.PublicKey(), grant: grant, secret: secret, unknown: unknown, nonces: map[string]bool{}}
	l, err := net.Listen("tcp", host)
	if err != nil {
		return ErrValidation
	}
	server := &http.Server{Handler: a, ReadHeaderTimeout: time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second, ErrorLog: log.New(io.Discard, "", 0)}
	done := make(chan error, 1)
	go func() {
		done <- server.Serve(tls.NewListener(l, &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{pair}}))
	}()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if server.Shutdown(ctx) != nil {
			server.Close()
			result = ErrValidation
		}
		if <-done != http.ErrServerClosed {
			result = ErrValidation
		}
	}()
	var consumed atomic.Int32
	consume := func(ctx context.Context, value string) error {
		if ctx.Err() != nil || sha256.Sum256([]byte(value)) != expected || !consumed.CompareAndSwap(0, 1) {
			return ErrValidation
		}
		return nil
	}
	for round := 0; round < 2; round++ {
		stage = "runner"
		if execute(release, factory, vault, digest, vaultScope, pub, scope, grant, consume, round == 0 && !unknown, pass) != nil {
			return ErrValidation
		}
		state, e := enrollmentstore.Open(scope, true)
		if e != nil {
			return ErrValidation
		}
		want := "registered"
		if unknown {
			want = "attempted"
		}
		if state.State().Phase != want || state.Begin() == nil {
			state.Close()
			return ErrValidation
		}
		if state.Close() != nil {
			return ErrValidation
		}
	}
	a.mu.Lock()
	mints, clocks := a.mints, a.clocks
	a.mu.Unlock()
	wantConsumed := int32(1)
	if unknown {
		wantConsumed = 0
	}
	if mints != 1 || clocks != 2 || consumed.Load() != wantConsumed {
		return ErrValidation
	}
	if unknown {
		pass("unknown_https_enrollment_receipt_preserved_no_consume_no_second_mint")
	} else {
		pass("native_signed_https_credential_consumed_and_registered_reopen_rejects_second_mint")
	}
	return nil
}

func execute(release trustinstall.Release, factory trustinstall.Factory, vault, digest, vaultScope, pub, scope string, grant invite.Grant, consume func(context.Context, string) error, success bool, pass func(string)) (result error) {
	stage := "owner_open"
	defer func() {
		if result != nil {
			pass("diagnostic_runner_" + stage)
		}
	}()
	owner, e := clientowner.Open(release, factory, time.Now)
	if e != nil {
		return ErrValidation
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	commands := make(chan installeddevice.Request, 1)
	outputs := make(chan any, 4)
	runner := installeddevice.EnrollmentRunner(release, clientruntime.NativeSigner(vault, digest, vaultScope, pub), commands, func(v any) error { outputs <- v; return nil }, installeddevice.NativeEnrollmentValidationFactory(scope, consume))
	h, e := owner.Start(ctx, runner)
	if e != nil {
		owner.Close(context.Background())
		return ErrValidation
	}
	stage = "ready"
	defer func() {
		cancel()
		h.Wait(context.Background())
		var closeErr error
		if h.State().Fault {
			closeErr = owner.RetireFault(context.Background())
		} else {
			closeErr = owner.Close(context.Background())
		}
		if closeErr != nil {
			result = ErrValidation
		}
	}()
	read := func() (map[string]any, error) {
		select {
		case v := <-outputs:
			b, _ := json.Marshal(v)
			var m map[string]any
			if json.Unmarshal(b, &m) != nil {
				return nil, ErrValidation
			}
			return m, nil
		case <-ctx.Done():
			return nil, ErrValidation
		}
	}
	if v, e := read(); e != nil || v["status"] != "ready" || len(v) != 1 {
		return ErrValidation
	}
	commands <- installeddevice.Request{Op: "network", Network: invite.NetworkRequestBody{Action: "grant_clock", Grant: grant}}
	stage = "grant_clock"
	if v, e := read(); e != nil || v["passed"] != true || v["active"] != false || v["phase"] != "grant_clock" || len(v) != 4 {
		return ErrValidation
	}
	commands <- installeddevice.Request{Op: "provision", Network: invite.NetworkRequestBody{Grant: grant}}
	stage = "provision"
	if v, e := read(); e != nil || v["status"] != "network_enrollment" || v["passed"] != success || v["active"] != false || len(v) != 3 {
		return ErrValidation
	}
	if success {
		commands <- installeddevice.Request{Op: "stop"}
	}
	e = h.Wait(ctx)
	if (e == nil) != success || !h.State().Joined || h.State().Fault == success {
		return ErrValidation
	}
	return nil
}
