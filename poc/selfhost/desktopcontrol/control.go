// Package desktopcontrol is a SAME-USER LOCAL coordination pipe, not a public
// peer API. Authentication is the Windows protected DACL; public identity
// fields pin the intended desktop instance, not a reusable network secret.
package desktopcontrol

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"sync"
	"time"

	"remoteapp.local/selfhost-poc/clientowner"
	"remoteapp.local/selfhost-poc/clientruntime"
	"remoteapp.local/selfhost-poc/privatepipe"
	"remoteapp.local/selfhost-poc/trustprofile"
)

type Request struct {
	Op      string
	Config  clientruntime.Config
	Profile trustprofile.Profile
}
type Backend interface {
	Probe(context.Context) error
	Start(context.Context, clientruntime.Config, func(any) error) error
	Stop(context.Context) error
	Upgrade(context.Context, trustprofile.Profile) error
	Count(context.Context) (clientruntime.ProductCount, error)
}

var ErrUpgradeCandidate = errors.New("upgrade_candidate_rejected")

type installationObserver interface {
	InstalledEpoch(context.Context) (uint64, error)
}

func Serve(ctx context.Context, ln net.Listener, scope, public string, b Backend) {
	life, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	monitor := make(chan struct{})
	go func() { defer close(monitor); <-life.Done(); ln.Close() }()
	defer func() { cancel(); ln.Close(); wg.Wait(); <-monitor }()
	slots := make(chan struct{}, 4)
	for {
		conn, e := ln.Accept()
		if e != nil {
			return
		}
		select {
		case slots <- struct{}{}:
		default:
			conn.Close()
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-slots }()
			defer conn.Close()
			connCtx, stop := context.WithCancel(life)
			defer stop()
			joined := make(chan struct{})
			go func() { defer close(joined); <-connCtx.Done(); conn.Close() }()
			defer func() { stop(); <-joined }()
			handle(connCtx, conn, scope, public, b)
		}()
	}
}
func handle(ctx context.Context, c net.Conn, scope, public string, b Backend) {
	r := privatepipe.NewReader(c)
	var mu sync.Mutex
	out := func(v any) error {
		mu.Lock()
		defer mu.Unlock()
		c.SetWriteDeadline(time.Now().Add(3 * time.Second))
		return json.NewEncoder(c).Encode(v)
	}
	c.SetReadDeadline(time.Now().Add(3 * time.Second))
	var hello struct{ Scope, Expected string }
	if b == nil || r.Read(&hello) != nil || hello.Scope != scope || hello.Expected != public {
		out(map[string]string{"status": "failed"})
		return
	}
	if out(map[string]string{"status": "ready", "public_key": public}) != nil {
		return
	}
	for {
		c.SetReadDeadline(time.Now().Add(10 * time.Minute))
		var req Request
		if r.Read(&req) != nil {
			return
		}
		limit, cancel := context.WithTimeout(ctx, 40*time.Second)
		var e error
		switch {
		case req.Op == "reopen_fault" && req.Config == (clientruntime.Config{}) && req.Profile == (trustprofile.Profile{}):
			if replacement, ok := b.(interface{ ReopenFault(context.Context) error }); ok {
				e = replacement.ReopenFault(limit)
				if e == nil {
					e = out(map[string]any{"status": "owner_reopened", "started": false})
				}
			} else {
				e = privatepipe.ErrProtocol
			}
		case req.Op == "retire_fault" && req.Config == (clientruntime.Config{}) && req.Profile == (trustprofile.Profile{}):
			if retire, ok := b.(interface{ RetireFault(context.Context) error }); ok {
				e = retire.RetireFault(limit)
				if e == nil {
					e = out(map[string]any{"status": "fault_retired", "joined": true, "fault": true})
				}
			} else {
				e = privatepipe.ErrProtocol
			}
		case req.Op == "session_state" && req.Config == (clientruntime.Config{}) && req.Profile == (trustprofile.Profile{}):
			// Optional capability keeps existing backends fail-closed.
			if observer, ok := b.(interface {
				SessionState() clientowner.SessionState
			}); ok {
				e = out(observer.SessionState())
			} else {
				e = privatepipe.ErrProtocol
			}
		case req.Op == "count" && req.Config == (clientruntime.Config{}) && req.Profile == (trustprofile.Profile{}):
			var count clientruntime.ProductCount
			count, e = b.Count(limit)
			if e == nil {
				e = out(count)
			}
		case req.Op == "probe" && req.Config == (clientruntime.Config{}) && req.Profile == (trustprofile.Profile{}):
			e = b.Probe(limit)
			if e == nil {
				e = out(map[string]string{"status": "probe_passed"})
			}
		case req.Op == "start" && req.Profile == (trustprofile.Profile{}):
			e = b.Start(ctx, req.Config, out) // runner outlives readiness and is tied to desktop owner, not this deadline
		case req.Op == "stop_session" && req.Config == (clientruntime.Config{}) && req.Profile == (trustprofile.Profile{}):
			e = b.Stop(limit)
			if e == nil {
				e = out(map[string]string{"status": "session_closed"})
			}
		case req.Op == "upgrade" && req.Config == (clientruntime.Config{}):
			e = b.Upgrade(limit, req.Profile)
			if e == nil {
				e = out(map[string]any{"status": "upgraded", "epoch": req.Profile.Claims.Epoch})
			} else if errors.Is(e, ErrUpgradeCandidate) {
				e = out(map[string]string{"status": "upgrade_rejected"})
			}
		case req.Op == "installation_state" && req.Config == (clientruntime.Config{}) && req.Profile == (trustprofile.Profile{}):
			observer, ok := b.(installationObserver)
			if !ok {
				e = privatepipe.ErrProtocol
				break
			}
			var epoch uint64
			epoch, e = observer.InstalledEpoch(limit)
			if e == nil {
				e = out(map[string]any{"status": "installation_state", "epoch": epoch})
			}
		default:
			e = privatepipe.ErrProtocol
		}
		cancel()
		if e != nil {
			if errors.Is(e, clientruntime.ErrTLSCertificatePublication) {
				out(map[string]string{"status": "failed", "failure_stage": "product_tls_publish"})
			} else if errors.Is(e, clientruntime.ErrTestCertificateOpen) {
				out(map[string]string{"status": "failed", "failure_stage": "product_certificate_open"})
			} else {
				out(map[string]string{"status": "failed"})
			}
			return
		}
	}
}
