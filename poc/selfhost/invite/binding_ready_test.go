package invite

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"
)

func TestBindingReadinessWaitsForActualPeerWithoutChangingOrdinaryState(t *testing.T) {
	f, g, b, _ := provisionFixture(t)
	id := enroll(t, f, g, b, f.owner, "a")
	if b.Bind(context.Background(), id) != nil {
		t.Fatal("first bind")
	}
	if _, err := b.NetworkState(g); err != ErrDenied {
		t.Fatal("ordinary state must still reject a partial binding")
	}
	server := httptest.NewTLSServer(NetworkHandler(b))
	defer server.Close()
	client, err := NewNetworkClient(server.URL, server.Certificate().Raw, f.owner)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	client.now = func() time.Time { return f.now }
	type result struct {
		reply NetworkReply
		err   error
	}
	done := make(chan result, 1)
	go func() {
		r, e := client.Request(context.Background(), NetworkRequestBody{Action: "await_binding", Grant: g})
		done <- result{r, e}
	}()
	select {
	case <-done:
		t.Fatal("partial binding returned before peer")
	case <-time.After(150 * time.Millisecond):
	}
	id = enroll(t, f, g, b, f.client, "b")
	if b.Bind(context.Background(), id) != nil || b.Reconcile(context.Background()) != nil {
		t.Fatal("peer bind or policy blocked by readiness wait")
	}
	select {
	case r := <-done:
		if r.err != nil || r.reply.State == nil || !VerifyNetworkState(f.s.PublicKey(), *r.reply.State, f.now) || !validBinding(r.reply.State.Claims.Binding) {
			t.Fatal("full signed binding unavailable", r.err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("readiness did not finish")
	}
}

func TestBindingReadinessCancellationRevocationAndAuthorizationRemainClosed(t *testing.T) {
	for _, mode := range []string{"cancel", "revoke", "foreign", "extra", "replay", "health", "rollback", "expiry"} {
		t.Run(mode, func(t *testing.T) {
			f, g, b, _ := provisionFixture(t)
			request := apiEnvelope(f, f.owner, g, "await_binding")
			ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
			defer cancel()
			switch mode {
			case "revoke":
				f.exec(t, f.owner, Body{Action: "revoke", GrantID: g.Claims.ID})
			case "foreign":
				request = apiEnvelope(f, f.other, g, "await_binding")
			case "extra":
				request.Body.Challenge = "foreign"
				request = SignNetworkRequest(f.owner, request.Body)
			case "replay":
				if _, err := b.ExecuteNetworkRequest(ctx, request); err == nil {
					t.Fatal("first wait unexpectedly ready")
				}
				ctx = context.Background()
			case "health":
				b.mu.Lock()
				b.ready = false
				b.mu.Unlock()
			case "rollback":
				f.now = f.now.Add(-time.Second)
			case "expiry":
				f.now = f.now.Add(301 * time.Second)
			}
			start := time.Now()
			if r, err := b.ExecuteNetworkRequest(ctx, request); err == nil || r != (NetworkReply{}) {
				t.Fatal("failed admission returned readiness")
			}
			if time.Since(start) > time.Second {
				t.Fatal("failure incorrectly waited as pending")
			}
		})
	}
}

func TestBindingReadinessWaitDoesNotOutliveOwnerRevocation(t *testing.T) {
	f, g, b, _ := provisionFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := b.AwaitNetworkState(ctx, g); done <- err }()
	select {
	case <-done:
		t.Fatal("missing pair did not wait")
	case <-time.After(120 * time.Millisecond):
	}
	f.exec(t, f.owner, Body{Action: "revoke", GrantID: g.Claims.ID})
	select {
	case err := <-done:
		if err != ErrDenied {
			t.Fatal("revocation returned a readiness state")
		}
	case <-time.After(time.Second):
		t.Fatal("revocation did not interrupt readiness")
	}
}
