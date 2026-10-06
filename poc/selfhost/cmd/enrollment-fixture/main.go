// Privileged private fixture. Its stdout contains bootstrap secrets: pipe only.
package main

import (
	"bufio"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
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
	d := &headscale.Driver{Run: headscale.LocalRunner(), PolicyDir: dir, PolicyMode: "database"}
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
	if b.Reconcile(ctx) != nil {
		return invite.ErrDenied
	}
	defer func() {
		execute(target, invite.Body{Action: "revoke", GrantID: g.Claims.ID})
		cleanup, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		b.Reconcile(cleanup)
	}()
	out := json.NewEncoder(os.Stdout)
	out.Encode(map[string]string{"status": "ready", "verifier": vk, "target_key": hex.EncodeToString(target), "controller_key": hex.EncodeToString(controller), "grant_id": g.Claims.ID})
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
			Op, Role, NodeKey, Ticket string
			Intent                    invite.ProvisionEnvelope
		}
		if json.Unmarshal(line, &c) != nil {
			return invite.ErrDenied
		}
		callCtx, stop := context.WithTimeout(context.Background(), 25*time.Second)
		result := map[string]any{"status": c.Op, "passed": false}
		switch c.Op {
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
		case "records":
			result["passed"] = true
			result["records"] = s.ProvisionRecords()
		case "revoke":
			_, e := execute(target, invite.Body{Action: "revoke", GrantID: g.Claims.ID})
			result["passed"] = e == nil && b.Reconcile(callCtx) == nil
		case "restart":
			if s.Close() != nil {
				stop()
				return invite.ErrStorage
			}
			s, e = invite.Open(state, time.Now)
			if e != nil {
				stop()
				return e
			}
			d = &headscale.Driver{Run: headscale.LocalRunner(), PolicyDir: dir, PolicyMode: "database", VerifierIP: d.VerifierIP, Proof: d.Proof}
			b = &invite.ProvisionBackend{Store: s, Driver: d}
			result["passed"] = b.Reconcile(callCtx) == nil
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
