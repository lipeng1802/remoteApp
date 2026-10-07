// Privileged private fixture. Its stdout contains bootstrap secrets: pipe only.
package main

import (
	"bufio"
	"context"
	"crypto/ed25519"
	"crypto/rand"
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
	"tailscale.com/tsnet"
)

func random(n int) string {
	b := make([]byte, n)
	if _, e := rand.Read(b); e != nil {
		panic("random unavailable")
	}
	return hex.EncodeToString(b)
}
func main() {
	if run() != nil {
		os.Exit(1)
	}
}
func run() error {
	nodeidentity.ConfigureOnline()
	dir, e := os.MkdirTemp("/var/lib/remoteapp-poc", "enrollment-")
	if e != nil {
		return e
	}
	defer os.RemoveAll(dir)
	state := filepath.Join(dir, "authority")
	s, e := invite.Open(state, time.Now)
	if e != nil {
		return e
	}
	defer func() {
		if s != nil {
			s.Close()
		}
	}()
	_, target, _ := ed25519.GenerateKey(rand.Reader)
	_, controller, _ := ed25519.GenerateKey(rand.Reader)
	execute := func(k ed25519.PrivateKey, b invite.Body) (invite.Result, error) {
		b.Nonce = random(16)
		b.IssuedAt = time.Now().Unix()
		return s.Execute(invite.Sign(k, b))
	}
	t, e := execute(target, invite.Body{Action: "register"})
	if e != nil {
		return e
	}
	if _, e = execute(controller, invite.Body{Action: "register"}); e != nil {
		return e
	}
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
	g := *approved.Grant
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
	verifierDir := filepath.Join(dir, "verifier")
	vk, e := nodeidentity.Prepare(verifierDir, "https://mk.fengmap.com:8443")
	if e != nil {
		return e
	}
	vr := invite.ProvisionRecord{ID: random(16), NodeKey: vk, Expires: time.Now().Unix() + 120}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		d.Cleanup(ctx, vr)
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
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
	proofStage := "not_called"
	d.Proof = func(ctx context.Context, r invite.ProvisionRecord, o invite.ProvisionObservation) bool {
		pub := controller.Public().(ed25519.PublicKey)
		if r.Device == g.Claims.Target {
			pub = target.Public().(ed25519.PublicKey)
		}
		return nodeproof.VerifyWithReport(ctx, v, r, o, pub, func(stage string) { proofStage = stage })
	}
	b := &invite.ProvisionBackend{Store: s, Driver: d}
	var maintenance *invite.Maintenance
	stopMaintenance := func(ctx context.Context) error {
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
	if b.Reconcile(ctx) != nil {
		return invite.ErrDenied
	}
	defer func() {
		join, joinCancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer joinCancel()
		if stopMaintenance(join) != nil {
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
		execute(target, invite.Body{Action: "revoke", GrantID: g.Claims.ID})
		cleanup, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		b.Reconcile(cleanup)
	}()
	out := json.NewEncoder(os.Stdout)
	var apiMu sync.RWMutex
	apiHandler := invite.NetworkHandler(b)
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
	out.Encode(map[string]any{"status": "ready", "verifier": vk, "target_key": hex.EncodeToString(target), "controller_key": hex.EncodeToString(controller), "grant_id": g.Claims.ID, "grant": g, "api_port": api.Listener.Addr().(*net.TCPAddr).Port, "api_certificate": base64.StdEncoding.EncodeToString(api.Certificate().Raw), "issuer": hex.EncodeToString(s.PublicKey()), "target": hex.EncodeToString(target.Public().(ed25519.PublicKey)), "controller": hex.EncodeToString(controller.Public().(ed25519.PublicKey))})
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
		}
		if json.Unmarshal(line, &c) != nil {
			return invite.ErrDenied
		}
		callCtx, stop := context.WithTimeout(context.Background(), 25*time.Second)
		result := map[string]any{"status": c.Op, "passed": false}
		switch c.Op {
		case "proof_result":
			apiMu.Lock()
			result["proof_stage"] = proofStage
			for _, r := range s.ProvisionRecords() {
				if r.ID == c.Ticket {
					result["registration_expired"] = time.Now().Unix() >= r.Expires
				}
			}
			apiMu.Unlock()
		case "maintenance_start":
			if maintenance == nil {
				maintenance, e = b.StartMaintenance(context.Background(), 500*time.Millisecond, 2*time.Second)
				result["passed"] = e == nil
			}
		case "maintenance_count":
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
			proofStage = "not_called"
			result["passed"] = b.Bind(callCtx, c.Ticket) == nil
			result["proof_stage"] = proofStage
			for _, record := range s.ProvisionRecords() {
				if record.ID == c.Ticket {
					result["registration_expired"] = time.Now().Unix() >= record.Expires
				}
			}
		case "reconcile":
			result["passed"] = b.Reconcile(callCtx) == nil
		case "zero":
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
			_, e := execute(target, invite.Body{Action: "revoke", GrantID: g.Claims.ID})
			result["passed"] = e == nil && b.Reconcile(callCtx) == nil
		case "revoke_only":
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
				d = &headscale.Driver{Run: policyRunner, PolicyDir: dir, PolicyMode: "database", VerifierIP: d.VerifierIP, Proof: d.Proof}
				b = &invite.ProvisionBackend{Store: s, Driver: d}
				apiHandler = invite.NetworkHandler(b)
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
			if stopMaintenance(callCtx) != nil {
				stop()
				return invite.ErrDenied
			}
			result["passed"] = s.Close() == nil // actual closed durable authority, keep the policy and nodes unchanged
			authorityOffline = true
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
