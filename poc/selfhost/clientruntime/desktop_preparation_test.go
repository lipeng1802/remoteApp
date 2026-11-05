package clientruntime

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/processowner"
)

type blockingPreparedAPI struct{ entered, cancelled, release chan struct{} }

func (a *blockingPreparedAPI) Request(ctx context.Context, _ invite.NetworkRequestBody) (invite.NetworkReply, error) {
	close(a.entered)
	<-ctx.Done()
	close(a.cancelled)
	<-a.release
	return invite.NetworkReply{}, ctx.Err()
}

func prepareFixture(t *testing.T, scope invite.DesktopPermission, server bool, flag string, barrier ...string) (*PreparedDesktop, Product, *productAPI) {
	t.Helper()
	a, _ := desktopProductAPI(t, scope)
	role, self := "controller", a.c.Grant.Claims.Controller
	if server {
		role, self = "target", a.c.Grant.Claims.Target
	}
	mode := "prepared-"
	if flag != "" {
		mode += flag + "-"
	}
	mode += string(scope) + "-" + role
	spec := productSpec(t, mode)
	if len(barrier) == 1 {
		spec.Args = append(spec.Args[:1], "prepared-barrier="+barrier[0], spec.Args[1])
	}
	p, e := PrepareDesktop(context.Background(), scope, self, server, spec)
	if p == nil {
		t.Fatal("missing partial owner")
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = p.Stop(ctx)
		select {
		case <-p.child.Done():
		default:
			t.Error("preparation child not actually joined")
		}
	})
	if flag == "readywrong" {
		if e == nil {
			t.Fatal("wrong preparation scope accepted")
		}
	} else if e != nil {
		t.Fatal(e)
	}
	product := Product{Prepared: p, Spec: spec}
	if server {
		product.TargetReady = func(string, string) {}
	} else {
		product.ControllerKey = testHMAC
	}
	return p, product, a
}

func TestPreparedDesktopActivatesSameChildOnlyAfterDecisionAndMatchingApproval(t *testing.T) {
	for _, scope := range []invite.DesktopPermission{invite.DesktopView, invite.DesktopControl} {
		for _, server := range []bool{false, true} {
			p, product, a := prepareFixture(t, scope, server, "")
			issuer := a.key.Public().(ed25519.PublicKey)
			id := p.PID()
			if id == 0 || strings.Contains(fmt.Sprintf("%#v", p), a.c.Grant.Signature) {
				t.Fatal("missing/redaction")
			}
			if child, e := StartBoundDesktopProduct(context.Background(), scope, product, a, issuer, a.c.Grant, a.c.Binding, server, time.Now, 32123, ""); e == nil || child != nil || a.published {
				t.Fatal("activation before consent")
			}
			if p.ArmTargetStop(context.Background()) == nil {
				t.Fatal("unactivated GUI armed")
			}
			request := strings.Repeat("e", 32)
			allowed, e := p.Decide(context.Background(), request, a.c.Grant.Claims.Controller)
			if e != nil || !allowed {
				t.Fatal("explicit decision failed")
			}
			if _, e = p.Decide(context.Background(), request, a.c.Grant.Claims.Controller); e == nil {
				t.Fatal("repeated consent")
			}
			if p.ObserveApproved(issuer, "other", a.c.Grant) == nil {
				t.Fatal("other request adopted")
			}
			wrong := a.c.Grant
			wrong.Claims.Controller = strings.Repeat("f", 64)
			if p.ObserveApproved(issuer, request, wrong) == nil {
				t.Fatal("foreign grant adopted")
			}
			if p.ObserveApproved(issuer, request, a.c.Grant) != nil || p.ObserveApproved(issuer, request, a.c.Grant) == nil {
				t.Fatal("approval not exact/one-shot")
			}
			altered := product
			altered.Spec.Args = append(append([]string(nil), altered.Spec.Args...), "extra")
			if child, e := StartBoundDesktopProduct(context.Background(), scope, altered, a, issuer, a.c.Grant, a.c.Binding, server, time.Now, 32123, ""); e == nil || child != nil {
				t.Fatal("replaced executable specification")
			}
			child, e := StartBoundDesktopProduct(context.Background(), scope, product, a, issuer, a.c.Grant, a.c.Binding, server, time.Now, 32123, "")
			if e != nil || child != p.child || child.PID() != id {
				t.Fatal("new child substituted for prepared process", e)
			}
			if repeated, e := StartBoundDesktopProduct(context.Background(), scope, product, a, issuer, a.c.Grant, a.c.Binding, server, time.Now, 32123, ""); e == nil || repeated != nil {
				t.Fatal("second activation")
			}
			if e = p.ArmTargetStop(context.Background()); (e == nil) != server {
				t.Fatal("stop arm scope or phase changed", e)
			}
			if p.ArmTargetStop(context.Background()) == nil {
				t.Fatal("stop arm replay")
			}
			if p.Stop(context.Background()) != nil {
				t.Fatal("prepared child not normally joined")
			}
		}
	}
}

func TestPreparedDesktopRefusalAndUnknownDecisionsNeverPublishOrRearm(t *testing.T) {
	for _, flag := range []string{"deny", "wrong", "missing", "readywrong"} {
		p, product, a := prepareFixture(t, invite.DesktopView, true, flag)
		issuer := a.key.Public().(ed25519.PublicKey)
		allowed, e := p.Decide(context.Background(), strings.Repeat("e", 32), a.c.Grant.Claims.Controller)
		if allowed || ((flag == "deny") != (e == nil)) {
			t.Fatal("unsafe decision accepted")
		}
		if p.ObserveApproved(issuer, strings.Repeat("e", 32), a.c.Grant) == nil {
			t.Fatal("refusal/unknown bound grant")
		}
		if child, e := StartBoundDesktopProduct(context.Background(), invite.DesktopView, product, a, issuer, a.c.Grant, a.c.Binding, true, time.Now, 0, ""); e == nil || child != nil || a.published {
			t.Fatal("refusal/unknown opened TLS")
		}
		if p.Stop(context.Background()) != nil {
			t.Fatal("unused child did not normally join")
		}
	}
}

func TestPreparedDesktopStopCancelsDialogAndTimeoutRetainsSameJoin(t *testing.T) {
	barrier := filepath.Join(t.TempDir(), "dialog-entered")
	p, _, a := prepareFixture(t, invite.DesktopControl, true, "slow", barrier)
	decision := make(chan error, 1)
	go func() {
		_, e := p.Decide(context.Background(), strings.Repeat("e", 32), a.c.Grant.Claims.Controller)
		decision <- e
	}()
	end := time.Now().Add(time.Second)
	for {
		if _, e := os.Stat(barrier); e == nil {
			break
		}
		if time.Now().After(end) {
			t.Fatal("decision not entered")
		}
		time.Sleep(time.Millisecond)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	if p.Stop(ctx) == nil {
		t.Fatal("dialog exit latency falsely joined")
	}
	cancel()
	select {
	case <-p.child.Done():
		t.Fatal("partial owner released early")
	default:
	}
	if e := <-decision; e == nil {
		t.Fatal("cancelled dialog became consent")
	}
	if p.Stop(context.Background()) == nil {
		t.Fatal("unknown receipt became normal success")
	}
	select {
	case <-p.child.Done():
	default:
		t.Fatal("same child not eventually joined")
	}
}

func TestPreparedDesktopInvalidPolicyIdentityOrContextStartsNoProcess(t *testing.T) {
	for _, scope := range []invite.DesktopPermission{"", "fixed-payload", "clipboard"} {
		if p, e := PrepareDesktop(context.Background(), scope, strings.Repeat("a", 64), true, processowner.Spec{}); e == nil || p != nil {
			t.Fatal("invalid scope")
		}
	}
	if p, e := PrepareDesktop(context.Background(), invite.DesktopControl, "invalid", true, processowner.Spec{}); e == nil || p != nil {
		t.Fatal("invalid local identity")
	}
}

func TestPreparedDesktopUnknownDescriptorDoesNotRestartOrRetryActivation(t *testing.T) {
	p, product, a := prepareFixture(t, invite.DesktopControl, false, "")
	issuer := a.key.Public().(ed25519.PublicKey)
	request := strings.Repeat("e", 32)
	if allow, e := p.Decide(context.Background(), request, a.c.Grant.Claims.Controller); e != nil || !allow {
		t.Fatal("consent")
	}
	if p.ObserveApproved(issuer, request, a.c.Grant) != nil {
		t.Fatal("approved binding")
	}
	a.mutate = "mixed"
	child, e := StartBoundDesktopProduct(context.Background(), invite.DesktopControl, product, a, issuer, a.c.Grant, a.c.Binding, false, time.Now, 32123, "")
	if e == nil || child != p.child || child.PID() != p.PID() {
		t.Fatal("unknown activation lost original child")
	}
	a.mutate = ""
	if retried, e := StartBoundDesktopProduct(context.Background(), invite.DesktopControl, product, a, issuer, a.c.Grant, a.c.Binding, false, time.Now, 32123, ""); e == nil || retried != nil {
		t.Fatal("activation retried after unknown descriptor")
	}
	if p.Stop(context.Background()) != nil {
		t.Fatal("unused original GUI not joined")
	}
}

func TestPreparedDesktopStopWaitsForActivationPrivateOperation(t *testing.T) {
	p, product, a := prepareFixture(t, invite.DesktopView, false, "")
	issuer := a.key.Public().(ed25519.PublicKey)
	request := strings.Repeat("e", 32)
	if allow, e := p.Decide(context.Background(), request, a.c.Grant.Claims.Controller); e != nil || !allow {
		t.Fatal("consent")
	}
	if p.ObserveApproved(issuer, request, a.c.Grant) != nil {
		t.Fatal("approval")
	}
	api := &blockingPreparedAPI{entered: make(chan struct{}), cancelled: make(chan struct{}), release: make(chan struct{})}
	result := make(chan error, 1)
	go func() {
		child, e := StartBoundDesktopProduct(context.Background(), invite.DesktopView, product, api, issuer, a.c.Grant, a.c.Binding, false, time.Now, 32123, "")
		if child != p.child {
			t.Error("lost child")
		}
		result <- e
	}()
	<-api.entered
	released := false
	defer func() {
		if !released {
			close(api.release)
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	if p.Stop(ctx) == nil {
		t.Fatal("GUI exit alone became full preparation join")
	}
	cancel()
	select {
	case <-api.cancelled:
	case <-time.After(time.Second):
		t.Fatal("activation was not cancelled")
	}
	close(api.release)
	released = true
	if e := <-result; e == nil {
		t.Fatal("cancelled activation became ready")
	}
	if p.Stop(context.Background()) != nil {
		t.Fatal("same GUI and operation not joined")
	}
}

func TestPreparedDesktopConnectIntentIsLocalScopedPrivateAndSingleUse(t *testing.T) {
	for _, flag := range []string{"connect", "connectcancel", "connectwrong"} {
		p, _, a := prepareFixture(t, invite.DesktopControl, false, flag)
		intent, e := p.ConnectIntent(context.Background())
		if flag == "connect" {
			if e != nil || intent == nil || intent.Claim() != (invite.Body{Action: "claim", Permission: invite.DesktopControl, TargetCode: "123456abcdef", Code: "12345678"}) {
				t.Fatal("invalid GUI intent")
			}
			if strings.Contains(fmt.Sprintf("%#v", intent), "12345678") {
				t.Fatal("intent leaked to diagnostics")
			}
			if _, e = json.Marshal(intent); e == nil {
				t.Fatal("intent became a config/JSON record")
			}
			if _, e = p.Decide(context.Background(), strings.Repeat("e", 32), a.c.Grant.Claims.Controller); e != nil {
				t.Fatal("consent after intent")
			}
		} else if flag == "connectcancel" {
			if e != nil || intent != nil {
				t.Fatal("cancelled intent became request")
			}
			if _, e = p.Decide(context.Background(), strings.Repeat("e", 32), a.c.Grant.Claims.Controller); e == nil {
				t.Fatal("cancelled UI rearmed")
			}
		} else if e == nil || intent != nil {
			t.Fatal("peer selected another scope")
		}
		if _, e = p.ConnectIntent(context.Background()); e == nil {
			t.Fatal("intent request repeated")
		}
		if p.Stop(context.Background()) != nil {
			t.Fatal("intent child not normally joined")
		}
	}
	target, _, _ := prepareFixture(t, invite.DesktopView, true, "")
	if _, e := target.ConnectIntent(context.Background()); e == nil {
		t.Fatal("target became connection initiator")
	}
}
