package streamgate

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"time"

	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/session"
)

// ManagedGate owns HTTPS polling and irreversible gate shutdown. The caller
// must Stop before closing its NetworkClient or signer. No parent state relay.
type ManagedGate struct {
	gate   *Gate
	watch  *invite.StateWatch
	cancel context.CancelFunc
	done   chan struct{}
}

// Calibrate uses a helper-generated nonce and monotonic request start. Neither
// an untrusted parent timestamp nor receipt-time TTL extends authority leases.
func Calibrate(ctx context.Context, c *invite.NetworkClient, issuer ed25519.PublicKey, grant invite.Grant) (func() time.Time, error) {
	if c == nil || len(issuer) != 32 {
		return nil, session.ErrDenied
	}
	var nonce [32]byte
	if _, e := rand.Read(nonce[:]); e != nil {
		return nil, session.ErrDenied
	}
	challenge := hex.EncodeToString(nonce[:])
	start := time.Now()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	reply, e := c.Request(ctx, invite.NetworkRequestBody{Action: "grant_clock", Grant: grant, Challenge: challenge})
	if e != nil || reply.Proof == nil {
		return nil, session.ErrDenied
	}
	clock, e := invite.AuthorityClock(issuer, *reply.Proof, grant.Claims.ID, challenge, start)
	if e != nil || !invite.VerifyGrant(issuer, grant, grant.Claims.Target, grant.Claims.Controller, clock()) {
		return nil, session.ErrDenied
	}
	return clock, nil
}

func StartManaged(ctx context.Context, client *invite.NetworkClient, issuer, target, controller ed25519.PublicKey, signer invite.DeviceSigner, grant invite.Grant, binding invite.NetworkBinding, server bool, clock func() time.Time) (*ManagedGate, error) {
	if client == nil {
		return nil, session.ErrDenied
	}
	ctx, cancel := context.WithCancel(ctx)
	watch, e := client.StartStateWatch(ctx, issuer, grant, binding, clock)
	if e != nil {
		cancel()
		return nil, session.ErrDenied
	}
	fail := func() (*ManagedGate, error) { cancel(); watch.Stop(); return nil, session.ErrDenied }
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	poll := time.NewTicker(25 * time.Millisecond)
	defer poll.Stop()
	var gate *Gate
	for gate == nil {
		if state, err := watch.Snapshot(); err == nil {
			gate, e = NewWithSigner(issuer, target, controller, signer, state, server, clock)
			if e != nil {
				return fail()
			}
			break
		}
		select {
		case <-ctx.Done():
			return fail()
		case <-watch.Done():
			return fail()
		case <-timer.C:
			return fail()
		case <-poll.C:
		}
	}
	m := &ManagedGate{gate: gate, watch: watch, cancel: cancel, done: make(chan struct{})}
	go func() {
		defer close(m.done)
		defer watch.Stop()
		defer gate.Deny() // latch off BEFORE waiting for the HTTPS worker to join
		tick := time.NewTicker(25 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-watch.Done():
				return
			case <-tick.C:
				state, err := watch.Snapshot()
				if err != nil || !gate.Update(state) || !state.Claims.Active {
					return
				}
			}
		}
	}()
	return m, nil
}
func (m *ManagedGate) Gate() *Gate           { return m.gate }
func (m *ManagedGate) Done() <-chan struct{} { return m.done }
func (m *ManagedGate) Stop()                 { m.gate.Deny(); m.cancel(); <-m.done }
