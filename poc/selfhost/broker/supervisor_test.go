package broker

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"path/filepath"
	"remoteapp.local/selfhost-poc/invite"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type liveDependency struct {
	live atomic.Bool
	done chan struct{}
}

func (v *liveDependency) Live() bool            { return v.live.Load() }
func (v *liveDependency) Done() <-chan struct{} { return v.done }

type supervisorDriver struct {
	fail        atomic.Bool
	mintStarted chan struct{}
	release     chan struct{}
	canceled    atomic.Bool
	zero        atomic.Int32
}

func (d *supervisorDriver) Mint(ctx context.Context, r invite.ProvisionRecord) (invite.RegistrationCredential, error) {
	if d.mintStarted != nil {
		close(d.mintStarted)
		<-ctx.Done()
		d.canceled.Store(true)
		<-d.release
	}
	return invite.RegistrationCredential{UserID: "1", KeyID: "1", Secret: "must-not-escape"}, nil
}
func (d *supervisorDriver) Observe(context.Context, invite.ProvisionRecord) (invite.ProvisionObservation, error) {
	return invite.ProvisionObservation{}, invite.ErrDenied
}
func (d *supervisorDriver) Verify(context.Context, invite.ProvisionRecord, invite.ProvisionObservation) bool {
	return false
}
func (d *supervisorDriver) Cleanup(context.Context, invite.ProvisionRecord) error { return nil }
func (d *supervisorDriver) ReplaceRules(ctx context.Context, r []invite.NetworkRule) error {
	if ctx.Err() != nil || d.fail.Load() {
		return invite.ErrDenied
	}
	if len(r) == 0 {
		d.zero.Add(1)
	}
	return nil
}
func supervisorFixture(t *testing.T, d *supervisorDriver) (*NetworkSupervisor, *invite.Store, *liveDependency) {
	t.Helper()
	path, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	st, e := invite.Open(filepath.Join(path, "state"), time.Now)
	if e != nil {
		t.Fatal(e)
	}
	v := &liveDependency{done: make(chan struct{})}
	v.live.Store(true)
	s, e := StartNetworkSupervisor(context.Background(), &invite.ProvisionBackend{Store: st, Driver: d}, v, 100*time.Millisecond)
	if e != nil {
		st.Close()
		t.Fatal(e)
	}
	t.Cleanup(func() {
		ctx, c := context.WithTimeout(context.Background(), 3*time.Second)
		defer c()
		if s.Stop(ctx) != nil {
			t.Error("stop timeout")
		}
		st.Close()
	})
	waitSupervisor(t, func() bool { return s.Ready() })
	return s, st, v
}
func waitSupervisor(t *testing.T, f func() bool) {
	t.Helper()
	end := time.Now().Add(3 * time.Second)
	for !f() {
		if time.Now().After(end) {
			t.Fatal("supervisor timeout")
		}
		time.Sleep(5 * time.Millisecond)
	}
}
func TestSupervisorPolicyFailureLatchesNoRecovery(t *testing.T) {
	d := &supervisorDriver{}
	s, _, _ := supervisorFixture(t, d)
	if _, e := StartNetworkSupervisor(context.Background(), s.backend, s.verifier, 100*time.Millisecond); e == nil {
		t.Fatal("backend acquired by second supervisor")
	}
	d.fail.Store(true)
	waitSupervisor(t, func() bool { return s.closed.Load() })
	d.fail.Store(false)
	ctx, c := context.WithTimeout(context.Background(), 3*time.Second)
	defer c()
	if s.Stop(ctx) != nil {
		t.Fatal("join")
	}
	if s.Ready() {
		t.Fatal("automatic recovery")
	}
	r := s.Authority().Execute(context.Background(), Request{Operation: "network", Network: &invite.NetworkRequest{}})
	if r.Status != 503 || r.Network != nil {
		t.Fatal("network gate reopened")
	}
}
func TestSupervisorVerifierFaultCancelsMintNoCredentialAndJoins(t *testing.T) {
	d := &supervisorDriver{mintStarted: make(chan struct{}), release: make(chan struct{})}
	s, st, v := supervisorFixture(t, d)
	_, target, _ := ed25519.GenerateKey(rand.Reader)
	_, controller, _ := ed25519.GenerateKey(rand.Reader)
	nonce := func() string { b := make([]byte, 16); rand.Read(b); return hex.EncodeToString(b) }
	call := func(k ed25519.PrivateKey, b invite.Body) invite.Result {
		t.Helper()
		b.Nonce = nonce()
		b.IssuedAt = time.Now().Unix()
		r, e := st.Execute(invite.Sign(k, b))
		if e != nil {
			t.Fatal(e)
		}
		return r
	}
	targetDevice := call(target, invite.Body{Action: "register"})
	call(controller, invite.Body{Action: "register"})
	inv := call(target, invite.Body{Action: "invite"})
	claim := call(controller, invite.Body{Action: "claim", TargetCode: targetDevice.DeviceCode, Code: inv.Code})
	approved := call(target, invite.Body{Action: "approve", RequestID: claim.RequestID, Allow: true})
	if approved.Grant == nil {
		t.Fatal("grant")
	}
	g := *approved.Grant
	intent := invite.SignProvision(target, invite.ProvisionBody{GrantID: g.Claims.ID, NodeKey: "nodekey:" + strings.Repeat("a", 64), Nonce: nonce(), IssuedAt: time.Now().Unix()})
	req := invite.SignNetworkRequest(target, invite.NetworkRequestBody{Action: "enroll", Grant: g, Nonce: nonce(), IssuedAt: time.Now().Unix(), Intent: intent})
	result := make(chan Reply, 1)
	go func() {
		result <- s.Authority().Execute(context.Background(), Request{Operation: "network", Network: &req})
	}()
	select {
	case <-d.mintStarted:
	case r := <-result:
		t.Fatal("mint not reached", r.Status)
	case <-time.After(3 * time.Second):
		t.Fatal("mint timeout")
	}
	v.live.Store(false)
	close(v.done)
	waitSupervisor(t, func() bool { return d.canceled.Load() })
	timeout, c := context.WithTimeout(context.Background(), 20*time.Millisecond)
	if s.Stop(timeout) == nil {
		t.Fatal("blocked driver falsely joined")
	}
	c()
	if s.Ready() {
		t.Fatal("ready after verifier fault")
	}
	close(d.release)
	select {
	case r := <-result:
		if r.Status == 200 || r.Network != nil {
			t.Fatal("credential escaped")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("request join")
	}
	ctx, c := context.WithTimeout(context.Background(), 3*time.Second)
	defer c()
	if s.Stop(ctx) != nil {
		t.Fatal("stop")
	}
	if d.zero.Load() < 3 {
		t.Fatal("no final explicit zero rules")
	}
	if !s.DenyAllConfirmed() {
		t.Fatal("final zero projection not confirmed")
	}
}

func TestSupervisorFailedWithdrawalNeverReportedAsACLSuccess(t *testing.T) {
	d := &supervisorDriver{}
	s, _, v := supervisorFixture(t, d)
	d.fail.Store(true)
	v.live.Store(false)
	close(v.done)
	ctx, c := context.WithTimeout(context.Background(), 3*time.Second)
	defer c()
	if s.Stop(ctx) != nil {
		t.Fatal("join")
	}
	if s.DenyAllConfirmed() || s.Ready() {
		t.Fatal("failed policy withdrawal reported as success")
	}
}
