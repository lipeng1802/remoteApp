package invite

import (
	"context"
	"crypto/ed25519"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type memoryDriver struct {
	resources                                                      map[string]ProvisionRecord
	rules                                                          []NetworkRule
	minted, cleaned                                                int
	failMint, failCleanup, failPolicy, rejectProof, badObservation bool
}

func (d *memoryDriver) Mint(ctx context.Context, r ProvisionRecord) (RegistrationCredential, error) {
	if ctx.Err() != nil {
		return RegistrationCredential{}, ErrDenied
	}
	d.minted++
	d.resources[r.ID] = r
	c := RegistrationCredential{"user-" + r.ID, "key-" + r.ID, "test-secret-" + r.ID}
	if d.failMint {
		return c, ErrDenied
	}
	return c, nil
}
func (d *memoryDriver) Observe(ctx context.Context, r ProvisionRecord) (ProvisionObservation, error) {
	ip := "100.120.0.1"
	if r.Device == r.Grant.Claims.Controller {
		ip = "100.120.0.2"
	}
	o := ProvisionObservation{r.UserID, "node-" + r.ID, r.NodeKey, ip}
	if d.badObservation {
		o.UserID = "other"
	}
	return o, nil
}
func (d *memoryDriver) Verify(context.Context, ProvisionRecord, ProvisionObservation) bool {
	return !d.rejectProof
}
func (d *memoryDriver) Cleanup(ctx context.Context, r ProvisionRecord) error {
	if d.failCleanup || ctx.Err() != nil {
		return ErrDenied
	}
	delete(d.resources, r.ID)
	d.cleaned++
	return nil
}
func (d *memoryDriver) ReplaceRules(ctx context.Context, r []NetworkRule) error {
	if d.failPolicy || ctx.Err() != nil {
		return ErrDenied
	}
	d.rules = append([]NetworkRule(nil), r...)
	return nil
}
func provisionFixture(t *testing.T) (*fixture, Grant, *ProvisionBackend, *memoryDriver) {
	f := newFixture(t)
	_, approved := f.granted(t)
	d := &memoryDriver{resources: map[string]ProvisionRecord{}}
	b := &ProvisionBackend{Store: f.s, Driver: d}
	if e := b.Reconcile(context.Background()); e != nil {
		t.Fatal(e)
	}
	return f, *approved.Grant, b, d
}
func intent(f *fixture, g Grant, k ed25519.PrivateKey, node string) ProvisionEnvelope {
	n, _ := randomHex(16)
	return SignProvision(k, ProvisionBody{g.Claims.ID, "nodekey:" + strings.Repeat(node, 64), n, f.now.Unix()})
}
func enroll(t *testing.T, f *fixture, g Grant, b *ProvisionBackend, k ed25519.PrivateKey, node string) string {
	t.Helper()
	id, c, e := b.Enroll(context.Background(), g, intent(f, g, k, node))
	if e != nil || id == "" || c.Secret == "" {
		t.Fatal("enroll failed")
	}
	return id
}
func bindPair(t *testing.T, f *fixture, g Grant, b *ProvisionBackend) []string {
	ids := []string{enroll(t, f, g, b, f.owner, "a"), enroll(t, f, g, b, f.client, "b")}
	for _, id := range ids {
		if e := b.Bind(context.Background(), id); e != nil {
			t.Fatal(e)
		}
	}
	if e := b.Reconcile(context.Background()); e != nil {
		t.Fatal(e)
	}
	return ids
}
func TestProvisionMinimalRuleAndSecretNeverPersisted(t *testing.T) {
	f, g, b, d := provisionFixture(t)
	id := enroll(t, f, g, b, f.owner, "a")
	if e := b.Bind(context.Background(), id); e != nil {
		t.Fatal(e)
	}
	if e := b.Reconcile(context.Background()); e != nil || len(d.rules) != 0 {
		t.Fatal("one endpoint granted policy")
	}
	id = enroll(t, f, g, b, f.client, "b")
	if e := b.Bind(context.Background(), id); e != nil {
		t.Fatal(e)
	}
	if e := b.Reconcile(context.Background()); e != nil || len(d.rules) != 1 || d.rules[0] != (NetworkRule{"100.120.0.2", "100.120.0.1:47476"}) {
		t.Fatal("non-minimal rule")
	}
	raw, e := os.ReadFile(filepath.Join(f.root, "state.json"))
	if e != nil || strings.Contains(string(raw), "test-secret-") {
		t.Fatal("secret persisted")
	}
}
func TestProvisionRestartAndKeyExpiryDoNotRevokeBoundGrant(t *testing.T) {
	f, g, b, d := provisionFixture(t)
	bindPair(t, f, g, b)
	f.restart(t)
	b = &ProvisionBackend{Store: f.s, Driver: d}
	if _, _, e := b.Enroll(context.Background(), g, intent(f, g, f.owner, "c")); e != ErrDenied {
		t.Fatal("served before startup reconcile")
	}
	f.now = f.now.Add(121 * time.Second)
	if e := b.Reconcile(context.Background()); e != nil || len(d.rules) != 1 || d.cleaned != 0 {
		t.Fatal("registration key expiry ended approved bound grant")
	}
	f.now = f.now.Add(180 * time.Second)
	if e := b.Reconcile(context.Background()); e != nil || len(d.rules) != 0 || len(d.resources) != 0 {
		t.Fatal("grant expiry not reclaimed")
	}
}
func TestProvisionReplayDuplicateAndWrongSubject(t *testing.T) {
	f, g, b, d := provisionFixture(t)
	e := intent(f, g, f.owner, "a")
	if _, _, err := b.Enroll(context.Background(), g, e); err != nil {
		t.Fatal(err)
	}
	for _, v := range []ProvisionEnvelope{e, intent(f, g, f.owner, "b"), intent(f, g, f.other, "c")} {
		if _, _, err := b.Enroll(context.Background(), g, v); err != ErrDenied {
			t.Fatal("replay/duplicate/third accepted")
		}
	}
	f.restart(t)
	b = &ProvisionBackend{Store: f.s, Driver: d}
	if err := b.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, _, err := b.Enroll(context.Background(), g, e); err != ErrDenied || d.minted != 1 {
		t.Fatal("restart replayed credential")
	}
}
func TestProvisionUnknownMintOutcomeRecoveredByTicket(t *testing.T) {
	f, g, b, d := provisionFixture(t)
	d.failMint = true
	id, c, e := b.Enroll(context.Background(), g, intent(f, g, f.owner, "a"))
	if e == nil || c.Secret != "" || len(d.resources) != 1 || id == "" {
		t.Fatal("unknown outcome not quarantined")
	}
	f.restart(t)
	b = &ProvisionBackend{Store: f.s, Driver: d}
	d.failMint = false
	if e = b.Reconcile(context.Background()); e != nil || len(d.resources) != 0 || d.minted != 1 {
		t.Fatal("recovery reminted instead of cleanup")
	}
	if f.s.ProvisionRecords()[0].Status != "cleaned" {
		t.Fatal("cleanup not durable")
	}
}
func TestProvisionUnfinishedIntentNeverReminted(t *testing.T) {
	f, g, b, d := provisionFixture(t)
	r, e := f.s.PlanProvision(g, intent(f, g, f.owner, "a"))
	if e != nil {
		t.Fatal(e)
	}
	// Crash between external creation and completion commit.
	d.resources[r.ID] = r
	f.restart(t)
	b = &ProvisionBackend{Store: f.s, Driver: d}
	if e = b.Reconcile(context.Background()); e != nil || len(d.resources) != 0 || d.minted != 0 {
		t.Fatal("orphan recovery")
	}
}
func TestProvisionObservationAndPossessionBothRequired(t *testing.T) {
	for _, badProof := range []bool{false, true} {
		f, g, b, d := provisionFixture(t)
		id := enroll(t, f, g, b, f.owner, "a")
		d.badObservation = !badProof
		d.rejectProof = badProof
		if e := b.Bind(context.Background(), id); e != ErrDenied {
			t.Fatal("unverified observation bound")
		}
		rules, e := f.s.ProvisionRules()
		if e != nil || len(rules) != 0 {
			t.Fatal("unverified observation granted rule")
		}
	}
}
func TestProvisionRevokeWithdrawsPolicyBeforeCleanupAndRetries(t *testing.T) {
	f, g, b, d := provisionFixture(t)
	bindPair(t, f, g, b)
	f.exec(t, f.owner, Body{Action: "revoke", GrantID: g.Claims.ID})
	d.failCleanup = true
	if e := b.Reconcile(context.Background()); e == nil || len(d.rules) != 0 {
		t.Fatal("policy not withdrawn before failed cleanup")
	}
	f.restart(t)
	b = &ProvisionBackend{Store: f.s, Driver: d}
	d.failCleanup = false
	if e := b.Reconcile(context.Background()); e != nil || len(d.resources) != 0 {
		t.Fatal("cleanup retry failed")
	}
}
func TestProvisionPolicyFailureBlocksCredentials(t *testing.T) {
	f, g, b, d := provisionFixture(t)
	d.failPolicy = true
	if e := b.Reconcile(context.Background()); e == nil {
		t.Fatal("policy error ignored")
	}
	if _, _, e := b.Enroll(context.Background(), g, intent(f, g, f.owner, "a")); e != ErrDenied || d.minted != 0 {
		t.Fatal("issued while unreconciled")
	}
	d.failPolicy = false
	if e := b.Reconcile(context.Background()); e != nil {
		t.Fatal(e)
	}
	enroll(t, f, g, b, f.owner, "a")
}
func TestProvisionConcurrentEnrollmentOnlyOneCredential(t *testing.T) {
	f, g, b, d := provisionFixture(t)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		e := intent(f, g, f.owner, "a")
		wg.Add(1)
		go func() { defer wg.Done(); b.Enroll(context.Background(), g, e) }()
	}
	wg.Wait()
	if d.minted != 1 {
		t.Fatal("duplicate concurrent issuance")
	}
}
func TestProvisionIntentTamperStaleAndBounds(t *testing.T) {
	f, g, b, d := provisionFixture(t)
	for _, kind := range []string{"signature", "stale", "node", "grant"} {
		e := intent(f, g, f.owner, "a")
		switch kind {
		case "signature":
			e.Body.NodeKey = "nodekey:" + strings.Repeat("b", 64)
		case "stale":
			e.Body.IssuedAt -= 60
			e = SignProvision(f.owner, e.Body)
		case "node":
			e.Body.NodeKey = strings.Repeat("a", 100000)
		case "grant":
			e.Body.GrantID = "bad"
		}
		if _, _, err := b.Enroll(context.Background(), g, e); err != ErrDenied {
			t.Fatal("invalid intent accepted")
		}
	}
	if d.minted != 0 {
		t.Fatal("invalid intent touched provider")
	}
}

func TestProvisionClockRollbackWithdrawsProjection(t *testing.T) {
	f, g, b, d := provisionFixture(t)
	bindPair(t, f, g, b)
	f.now = f.now.Add(-time.Minute)
	if e := b.Reconcile(context.Background()); e != ErrDenied || len(d.rules) != 0 {
		t.Fatal("clock rollback kept allow policy")
	}
	if _, _, e := b.Enroll(context.Background(), g, intent(f, g, f.owner, "c")); e != ErrDenied {
		t.Fatal("clock rollback served credentials")
	}
}

func TestProvisionCorruptLedgerRefusesRestart(t *testing.T) {
	f, g, _, _ := provisionFixture(t)
	if _, e := f.s.PlanProvision(g, intent(f, g, f.owner, "a")); e != nil {
		t.Fatal(e)
	}
	if e := f.s.Close(); e != nil {
		t.Fatal(e)
	}
	p := filepath.Join(f.root, "state.json")
	raw, e := os.ReadFile(p)
	if e != nil {
		t.Fatal(e)
	}
	raw = []byte(strings.Replace(string(raw), `"Status":"planned"`, `"Status":"unknown"`, 1))
	if e = os.WriteFile(p, raw, 0600); e != nil {
		t.Fatal(e)
	}
	if _, e = Open(f.root, f.s.clock); e != ErrStorage {
		t.Fatal("corrupt ledger accepted")
	}
}

func TestProvisionSharedStoreFailureFencesAllWrappers(t *testing.T) {
	f, g, b, d := provisionFixture(t)
	other := &ProvisionBackend{Store: f.s, Driver: d}
	d.failPolicy = true
	if e := other.Reconcile(context.Background()); e == nil {
		t.Fatal("policy failure ignored")
	}
	if _, _, e := b.Enroll(context.Background(), g, intent(f, g, f.owner, "a")); e != ErrDenied || d.minted != 0 {
		t.Fatal("other wrapper bypassed policy fence")
	}
	d.failPolicy = false
	if e := other.Reconcile(context.Background()); e != nil {
		t.Fatal(e)
	}
	enroll(t, f, g, b, f.owner, "a")
}
