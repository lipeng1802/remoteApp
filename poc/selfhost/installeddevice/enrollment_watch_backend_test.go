package installeddevice

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/trustprofile"
)

type watchPolicyDriver struct {
	mu    sync.Mutex
	rules []invite.NetworkRule
}

func (d *watchPolicyDriver) Mint(context.Context, invite.ProvisionRecord) (invite.RegistrationCredential, error) {
	return invite.RegistrationCredential{UserID: "fixture-user", KeyID: "fixture-key", Secret: "fixture-secret"}, nil
}
func (d *watchPolicyDriver) Observe(_ context.Context, r invite.ProvisionRecord) (invite.ProvisionObservation, error) {
	ip := "100.120.0.42"
	if r.Device == r.Grant.Claims.Controller {
		ip = "100.120.0.93"
	}
	return invite.ProvisionObservation{UserID: r.UserID, NodeID: "fixture-node-" + r.Device[:8], NodeKey: r.NodeKey, IP: ip}, nil
}
func (d *watchPolicyDriver) Verify(_ context.Context, r invite.ProvisionRecord, o invite.ProvisionObservation) bool {
	return o.UserID == r.UserID && o.NodeKey == r.NodeKey && o.NodeID != "" && o.IP != ""
}
func (d *watchPolicyDriver) Cleanup(context.Context, invite.ProvisionRecord) error { return nil }
func (d *watchPolicyDriver) ReplaceRules(ctx context.Context, rules []invite.NetworkRule) error {
	if ctx.Err() != nil {
		return invite.ErrDenied
	}
	d.mu.Lock()
	d.rules = append([]invite.NetworkRule(nil), rules...)
	d.mu.Unlock()
	return nil
}
func (d *watchPolicyDriver) Healthy() bool { return true }
func (d *watchPolicyDriver) snapshot() []invite.NetworkRule {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]invite.NetworkRule(nil), d.rules...)
}

func signedDeviceCall(t *testing.T, s *invite.Store, key ed25519.PrivateKey, body invite.Body, nonce byte) invite.Result {
	t.Helper()
	body.Nonce = strings.Repeat(string([]byte{nonce}), 32)
	body.IssuedAt = time.Now().Unix()
	r, err := s.Execute(invite.Sign(key, body))
	if err != nil {
		t.Fatal("signed device call", err)
	}
	return r
}

func TestTwoEnrollmentWatchesJoinAfterRevocationAndZeroProjection(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	store, err := invite.Open(filepath.Join(dir, "authority"), time.Now)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	_, targetKey, _ := ed25519.GenerateKey(rand.Reader)
	_, controllerKey, _ := ed25519.GenerateKey(rand.Reader)
	target := signedDeviceCall(t, store, targetKey, invite.Body{Action: "register"}, '1')
	signedDeviceCall(t, store, controllerKey, invite.Body{Action: "register"}, '2')
	invitation := signedDeviceCall(t, store, targetKey, invite.Body{Action: "invite"}, '3')
	claim := signedDeviceCall(t, store, controllerKey, invite.Body{Action: "claim", TargetCode: target.DeviceCode, Code: invitation.Code}, '4')
	approved := signedDeviceCall(t, store, targetKey, invite.Body{Action: "approve", RequestID: claim.RequestID, Allow: true}, '5')
	if approved.Grant == nil {
		t.Fatal("grant")
	}
	grant := *approved.Grant
	driver := &watchPolicyDriver{}
	backend := &invite.ProvisionBackend{Store: store, Driver: driver}
	if err = backend.Reconcile(context.Background()); err != nil {
		t.Fatal("initial zero projection")
	}
	type side struct {
		key    ed25519.PrivateKey
		node   *proofNodeFixture
		ticket string
		gate   *authorizationGate
		e      *enrollment
	}
	sides := []side{
		{key: targetKey, node: &proofNodeFixture{enrollmentNodeFixture: &enrollmentNodeFixture{key: "nodekey:" + strings.Repeat("a", 64), address: "100.120.0.42"}, ready: true}},
		{key: controllerKey, node: &proofNodeFixture{enrollmentNodeFixture: &enrollmentNodeFixture{key: "nodekey:" + strings.Repeat("b", 64), address: "100.120.0.93"}, ready: true}},
	}
	for i := range sides {
		intent := invite.SignProvision(sides[i].key, invite.ProvisionBody{GrantID: grant.Claims.ID, NodeKey: sides[i].node.key, Nonce: strings.Repeat(string([]byte{'6' + byte(i)}), 32), IssuedAt: time.Now().Unix()})
		ticket, credential, enrollErr := backend.Enroll(context.Background(), grant, intent)
		if enrollErr != nil || credential.Secret == "" || backend.Bind(context.Background(), ticket) != nil {
			t.Fatal("verified pair bind", i, enrollErr)
		}
		credential = invite.RegistrationCredential{}
		signer := &testSigner{key: sides[i].key, joined: make(chan struct{})}
		sides[i].gate = newAuthorizationGate(trustprofile.Profile{Claims: trustprofile.Claims{Issuer: hex.EncodeToString(store.PublicKey())}}, signer)
		sides[i].gate.clock = time.Now
		sides[i].gate.grant = grant
		sides[i].ticket = ticket
		sides[i].e = &enrollment{node: sides[i].node, ticket: ticket, key: sides[i].node.key, address: sides[i].node.address, attempted: true, proofCancel: func() {}}
	}
	if err = backend.Reconcile(context.Background()); err != nil || len(driver.snapshot()) != 1 {
		t.Fatal("active policy projection")
	}
	request := requestFunc(func(ctx context.Context, body invite.NetworkRequestBody) (invite.NetworkReply, error) {
		if body.Action != "state" || body.Grant != grant {
			return invite.NetworkReply{}, invite.ErrDenied
		}
		state, stateErr := backend.NetworkState(grant)
		if stateErr != nil {
			return invite.NetworkReply{}, stateErr
		}
		return invite.NetworkReply{State: &state}, nil
	})
	for i := range sides {
		if r, watchErr := sides[i].e.startWatch(context.Background(), sides[i].gate, request); watchErr != nil || !r.Passed || !r.Active {
			t.Fatal("active watch", i, watchErr)
		}
	}
	signedDeviceCall(t, store, targetKey, invite.Body{Action: "revoke", GrantID: grant.Claims.ID}, '8')
	if err = backend.Reconcile(context.Background()); err != nil || len(driver.snapshot()) != 0 {
		t.Fatal("revocation did not project zero rules")
	}
	for i := range sides {
		select {
		case <-sides[i].e.watch.Done():
			if !sides[i].e.watch.Faulted() {
				t.Fatal("revoked watch did not fault", i)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("revoked watch remained live", i)
		}
		if sides[i].e.closeJoined() != nil || sides[i].node.closes != 1 {
			t.Fatal("revoked owner resources not joined", i)
		}
	}
}
