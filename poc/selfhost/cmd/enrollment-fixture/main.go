// Privileged private fixture. Its stdout contains bootstrap secrets: pipe only.
package main

import (
	"bufio"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"remoteapp.local/selfhost-poc/headscale"
	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/nodeidentity"
	"remoteapp.local/selfhost-poc/nodeproof"
	"remoteapp.local/selfhost-poc/privatepipe"
	"tailscale.com/tsnet"
)

func random(n int) string {
	b := make([]byte, n)
	if _, e := rand.Read(b); e != nil {
		panic("random unavailable")
	}
	return hex.EncodeToString(b)
}
func hexDeviceID(pub ed25519.PublicKey) string {
	h := sha256.Sum256(pub)
	return hex.EncodeToString(h[:])
}
func main() {
	if run() != nil {
		os.Exit(1)
	}
}
func run() (result error) {
	nodeidentity.ConfigureOnline()
	dir, e := os.MkdirTemp("/var/lib/remoteapp-poc", "enrollment-")
	if e != nil {
		return e
	}
	retainFixture := false
	defer func() {
		if !retainFixture {
			os.RemoveAll(dir)
		}
	}()
	state := filepath.Join(dir, "authority")
	s, e := invite.Open(state, time.Now)
	if e != nil {
		return e
	}
	defer func() {
		if s != nil && !retainFixture {
			s.Close()
		}
	}()
	supervised := len(os.Args) == 2 && os.Args[1] == "--native-supervised"
	native := supervised || len(os.Args) == 2 && os.Args[1] == "--native"
	deviceFlow := native || len(os.Args) == 2 && os.Args[1] == "--device-flow"
	if len(os.Args) > 1 && !deviceFlow {
		return invite.ErrDenied
	}
	var target, controller ed25519.PrivateKey
	var targetPub, controllerPub ed25519.PublicKey
	if native {
		var bootstrap struct{ Target, Controller string }
		if privatepipe.NewReader(os.Stdin).Read(&bootstrap) != nil {
			return invite.ErrDenied
		}
		targetPub, controllerPub, e = nativePublics(bootstrap.Target, bootstrap.Controller)
		if e != nil {
			return e
		}
	} else {
		targetPub, target, _ = ed25519.GenerateKey(rand.Reader)
		controllerPub, controller, _ = ed25519.GenerateKey(rand.Reader)
	}
	execute := func(k ed25519.PrivateKey, b invite.Body) (invite.Result, error) {
		if native || len(k) != ed25519.PrivateKeySize {
			return invite.Result{}, invite.ErrDenied
		}
		b.Nonce = random(16)
		b.IssuedAt = time.Now().Unix()
		return s.Execute(invite.Sign(k, b))
	}
	var t invite.Result
	if native {
		t.DeviceID = hexDeviceID(targetPub)
	} else {
		t, e = execute(target, invite.Body{Action: "register"})
		if e != nil {
			return e
		}
		if _, e = execute(controller, invite.Body{Action: "register"}); e != nil {
			return e
		}
	}
	var g invite.Grant
	if !deviceFlow {
		inv, e := execute(target, invite.Body{Action: "invite"})
		if e != nil {
			return e
		}
		req, e := execute(controller, invite.Body{Action: "claim", TargetCode: t.DeviceCode, Code: inv.Code})
		if e != nil {
			return e
		}
		approved, e := execute(target, invite.Body{Action: "approve", RequestID: req.RequestID, Allow: true})
		if e != nil {
			return e
		}
		g = *approved.Grant
	}
	var failPolicy atomic.Bool
	runner := headscale.LocalRunner()
	// Private fault injection: reject privileged policy commands, retain the
	// last actual ACL, and exercise the real driver's poisoned-state recovery.
	policyRunner := func(ctx context.Context, args ...string) ([]byte, error) {
		if failPolicy.Load() && len(args) > 0 && args[0] == "policy" {
			return nil, headscale.ErrDriver
		}
		return runner(ctx, args...)
	}
	d := &headscale.Driver{Run: policyRunner, PolicyDir: dir, PolicyMode: "database"}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var proofStage atomic.Value
	proofStage.Store("not_called")
	var sf *supervisedFixture
	var vk string
	if supervised {
		sf, e = newSupervisedFixture(dir, s, policyRunner, func(stage string) { proofStage.Store(stage) })
		if e != nil {
			if sf != nil {
				retainFixture = true
			}
			return e
		}
		defer func() {
			if err := sf.Close(); err != nil {
				retainFixture = true
				result = err
			}
		}()
		vk = sf.runtime.Info().Key
	} else {
		verifierDir := filepath.Join(dir, "verifier")
		vk, e = nodeidentity.Prepare(verifierDir, "https://mk.fengmap.com:8443")
		if e != nil {
			return e
		}
		vr := invite.ProvisionRecord{ID: random(16), NodeKey: vk, Expires: time.Now().Unix() + 120}
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
			defer cancel()
			d.Cleanup(ctx, vr)
		}()
		credential, e := d.Mint(ctx, vr)
		if e != nil {
			return e
		}
		v := &tsnet.Server{Dir: verifierDir, Hostname: "poc-verifier", ControlURL: "https://mk.fengmap.com:8443", AuthKey: credential.Secret, Logf: func(string, ...any) {}, UserLogf: func(string, ...any) {}}
		defer v.Close()
		status, e := v.Up(ctx)
		if e != nil || status.Self == nil || status.Self.PublicKey.String() != vk {
			return errors.New("verifier identity mismatch")
		}
		for _, ip := range status.TailscaleIPs {
			if ip.Is4() {
				d.VerifierIP = ip.String()
			}
		}
		if d.VerifierIP == "" {
			return invite.ErrDenied
		}
		d.Proof = func(ctx context.Context, r invite.ProvisionRecord, o invite.ProvisionObservation) bool {
			pub := controllerPub
			if r.Device == g.Claims.Target {
				pub = targetPub
			}
			return nodeproof.VerifyWithReport(ctx, v, r, o, pub, func(stage string) { proofStage.Store(stage) })
		}
	}
	b := &invite.ProvisionBackend{Store: s, Driver: d}
	interval := time.Minute // early fixture phases use explicit reconcile/zero.
	if supervised {
		b, d, e = sf.Start(s, interval)
		if e != nil {
			return e
		}
	}
	var maintenance *invite.Maintenance
	stopMaintenance := func(ctx context.Context) error {
		if supervised {
			return sf.Stop(ctx)
		}
		if maintenance == nil {
			return nil
		}
		e := maintenance.Stop(ctx)
		if e == nil {
			maintenance = nil
		}
		return e
	}
	authorityOffline := false
	if !supervised && b.Reconcile(ctx) != nil {
		return invite.ErrDenied
	}
	defer func() {
		priorResult := result
		if supervised {
			// Any early return retains the scoped journal for operator recovery.
			// Do not erase identities/resources merely because parent stdin ended.
			retainFixture = true
			result = invite.ErrDenied
		}
		join, joinCancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer joinCancel()
		if stopMaintenance(join) != nil {
			retainFixture = true
			result = invite.ErrDenied
			return
		}
		failPolicy.Store(false)
		// Fault injection must not strand live registration resources if the
		// parent aborts before explicitly reopening the authority.
		if authorityOffline {
			var err error
			s, err = invite.Open(state, time.Now)
			if err != nil {
				return
			}
			b.Store = s
		}
		if native {
			// Privileged fixture teardown only: deny routes before reclaiming its
			// own journaled nodes. Never forge an OS device revocation signature.
			if d.ReplaceRules(join, nil) != nil {
				return
			}
			for _, r := range s.ProvisionRecords() {
				if r.Status == "cleaned" {
					continue
				}
				if s.ProvisionCleanup(r.ID, false) != nil || d.Cleanup(join, r) != nil {
					return
				}
				if s.ProvisionCleanup(r.ID, true) != nil {
					return
				}
			}
		} else {
			execute(target, invite.Body{Action: "revoke", GrantID: g.Claims.ID})
		}
		cleanup, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		if supervised {
			if (&invite.ProvisionBackend{Store: s, Driver: d}).Reconcile(cleanup) != nil {
				result = invite.ErrDenied
				return
			}
			retainFixture = false
			result = priorResult
		} else {
			b.Reconcile(cleanup)
		}
	}()
	out := json.NewEncoder(os.Stdout)
	var apiMu sync.RWMutex
	admission := invite.NewAdmission()
	apiHandler := admission.Wrap(invite.NetworkHandler(b))
	if supervised {
		apiHandler = admission.Wrap(sf.Handler())
	}
	api := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiMu.RLock()
		defer apiMu.RUnlock()
		apiHandler.ServeHTTP(w, r)
	}))
	api.Config.ReadHeaderTimeout = 2 * time.Second
	api.Config.ReadTimeout = 5 * time.Second
	api.Config.WriteTimeout = 15 * time.Second
	api.Config.IdleTimeout = 15 * time.Second
	api.Config.MaxHeaderBytes = 8192
	api.Config.ErrorLog = log.New(io.Discard, "", 0)
	_, apiKey, _ := ed25519.GenerateKey(rand.Reader)
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return err
	}
	template := &x509.Certificate{SerialNumber: serial, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}}
	der, err := x509.CreateCertificate(rand.Reader, template, template, apiKey.Public(), apiKey)
	if err != nil {
		return err
	}
	api.TLS = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: apiKey}}}
	api.StartTLS()
	defer api.Close()
	ready := map[string]any{"status": "ready", "verifier": vk, "grant_id": g.Claims.ID, "grant": g, "api_port": api.Listener.Addr().(*net.TCPAddr).Port, "api_certificate": base64.StdEncoding.EncodeToString(api.Certificate().Raw), "issuer": hex.EncodeToString(s.PublicKey()), "target": hex.EncodeToString(targetPub), "controller": hex.EncodeToString(controllerPub)}
	ready["supervised"] = supervised
	if !native {
		ready["target_key"] = hex.EncodeToString(target)
		ready["controller_key"] = hex.EncodeToString(controller)
	}
	out.Encode(ready)
	reader := bufio.NewReaderSize(os.Stdin, 8194)
	for {
		line, e := reader.ReadSlice('\n')
		if e == io.EOF && len(line) == 0 {
			return nil
		}
		if e != nil {
			return e
		}
		var c struct {
			Op, Role, NodeKey, Ticket, Challenge string
			Intent                               invite.ProvisionEnvelope
			Grant                                invite.Grant
		}
		if json.Unmarshal(line, &c) != nil {
			return invite.ErrDenied
		}
		callCtx, stop := context.WithTimeout(context.Background(), 25*time.Second)
		result := map[string]any{"status": c.Op, "passed": false}
		switch c.Op {
		case "adopt_grant":
			apiMu.Lock()
			if deviceFlow && g == (invite.Grant{}) && c.Grant.Claims.Target == t.DeviceID && c.Grant.Claims.Controller == hexDeviceID(controllerPub) && s.Authorized(c.Grant, c.Grant.Claims.Target, c.Grant.Claims.Controller) {
				g = c.Grant
				result["passed"] = true
			}
			apiMu.Unlock()
		case "proof_result":
			apiMu.Lock()
			result["proof_stage"] = proofStage.Load()
			for _, r := range s.ProvisionRecords() {
				if r.ID == c.Ticket {
					result["registration_expired"] = time.Now().Unix() >= r.Expires
				}
			}
			apiMu.Unlock()
		case "maintenance_start":
			if supervised {
				apiMu.Lock()
				interval = 500 * time.Millisecond
				b, d, e = sf.Start(s, interval)
				result["passed"] = e == nil
				apiMu.Unlock()
				break
			}
			if maintenance == nil {
				maintenance, e = b.StartMaintenance(context.Background(), 500*time.Millisecond, 2*time.Second)
				result["passed"] = e == nil
			}
		case "maintenance_count":
			if supervised {
				result["passed"] = true
				result["maintenance"] = sf.supervisor.MaintenanceStatus()
				result["network_ready"] = sf.supervisor.Ready()
				result["verifier_live"] = sf.runtime.Live()
				break
			}
			if maintenance != nil {
				result["passed"] = true
				result["maintenance"] = maintenance.Status()
			}
		case "maintenance_stop":
			result["passed"] = stopMaintenance(callCtx) == nil
		case "policy_unavailable":
			failPolicy.Store(true)
			result["passed"] = true
		case "policy_available":
			failPolicy.Store(false)
			result["passed"] = true
		case "enroll":
			if native {
				break
			}
			key := target
			if c.Role == "controller" {
				key = controller
			} else if c.Role != "target" {
				stop()
				return invite.ErrDenied
			}
			if c.Intent.PublicKey != hex.EncodeToString(key.Public().(ed25519.PublicKey)) {
				stop()
				return invite.ErrDenied
			}
			id, cred, e := b.Enroll(callCtx, g, c.Intent)
			result["passed"] = e == nil
			result["ticket"] = id
			result["credential"] = cred
		case "bind":
			proofStage.Store("not_called")
			result["passed"] = b.Bind(callCtx, c.Ticket) == nil
			result["proof_stage"] = proofStage.Load()
			for _, record := range s.ProvisionRecords() {
				if record.ID == c.Ticket {
					result["registration_expired"] = time.Now().Unix() >= record.Expires
				}
			}
		case "reconcile":
			if supervised && !sf.supervisor.Ready() {
				apiMu.Lock()
				b, d, e = sf.Start(s, interval)
				result["passed"] = e == nil
				apiMu.Unlock()
				break
			}
			result["passed"] = b.Reconcile(callCtx) == nil
		case "zero":
			if supervised && sf.Stop(callCtx) != nil {
				break
			}
			result["passed"] = d.ReplaceRules(callCtx, nil) == nil
		case "rules":
			rules, e := s.ProvisionRules()
			result["passed"] = e == nil
			result["rules"] = rules
		case "network_state":
			n, e := b.NetworkState(g)
			result["passed"] = e == nil && invite.VerifyNetworkState(s.PublicKey(), n, time.Now())
			if e == nil {
				result["network_state"] = n
			}
		case "clock_proof":
			proof, e := b.ClockProof(g, c.Challenge)
			result["passed"] = e == nil
			if e == nil {
				result["proof"] = proof
			}
		case "records":
			result["passed"] = true
			result["records"] = s.ProvisionRecords()
		case "revoke":
			if native {
				break
			}
			_, e := execute(target, invite.Body{Action: "revoke", GrantID: g.Claims.ID})
			result["passed"] = e == nil && b.Reconcile(callCtx) == nil
		case "revoke_only":
			if native {
				break
			}
			_, e := execute(target, invite.Body{Action: "revoke", GrantID: g.Claims.ID})
			result["passed"] = e == nil // keep nodes/ACL until explicit reconcile; test application gate, not route withdrawal
		case "restart":
			restartErr := func() error {
				apiMu.Lock()
				defer apiMu.Unlock()
				periodic := maintenance != nil
				if stopMaintenance(callCtx) != nil {
					stop()
					return invite.ErrDenied
				}
				if s.Close() != nil {
					stop()
					return invite.ErrStorage
				}
				s, e = invite.Open(state, time.Now)
				if e != nil {
					stop()
					return e
				}
				authorityOffline = false
				if supervised {
					b, d, e = sf.Start(s, interval)
					if e != nil {
						return e
					}
					apiHandler = admission.Wrap(sf.Handler())
					result["passed"] = true
					return nil
				}
				d = &headscale.Driver{Run: policyRunner, PolicyDir: dir, PolicyMode: "database", VerifierIP: d.VerifierIP, Proof: d.Proof}
				b = &invite.ProvisionBackend{Store: s, Driver: d}
				apiHandler = admission.Wrap(invite.NetworkHandler(b))
				if periodic {
					maintenance, e = b.StartMaintenance(context.Background(), 500*time.Millisecond, 2*time.Second)
					result["passed"] = e == nil
				} else {
					result["passed"] = b.Reconcile(callCtx) == nil
				}
				return nil
			}()
			if restartErr != nil {
				return restartErr
			}
		case "authority_unavailable":
			if supervised {
				failPolicy.Store(true)
			} // preserve old ACL during fault.
			if stopMaintenance(callCtx) != nil {
				stop()
				return invite.ErrDenied
			}
			result["passed"] = s.Close() == nil // actual closed durable authority, keep the policy and nodes unchanged
			authorityOffline = true
			if supervised {
				failPolicy.Store(false)
			}
		case "verifier_unavailable":
			if supervised {
				result["passed"] = sf.runtime.Stop(callCtx) == nil
			}
		case "verifier_resume":
			if supervised {
				apiMu.Lock()
				// The runtime outlives this private management command's deadline.
				if sf.ResumeVerifier(context.Background()) == nil {
					b, d, e = sf.Start(s, interval)
					result["passed"] = e == nil
				}
				apiMu.Unlock()
			}
		case "stop":
			stop()
			return nil
		default:
			stop()
			return invite.ErrDenied
		}
		stop()
		if out.Encode(result) != nil {
			return io.ErrClosedPipe
		}
	}
}
