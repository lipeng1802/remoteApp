// Package desktopcontrol is a SAME-USER LOCAL coordination pipe, not a public
// peer API. Authentication is the Windows protected DACL; public identity
// fields pin the intended desktop instance, not a reusable network secret.
package desktopcontrol

import (
	"context"
	"encoding/json"
	"net"
	"sync"
	"time"

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
			}
		default:
			e = privatepipe.ErrProtocol
		}
		cancel()
		if e != nil {
			out(map[string]string{"status": "failed"})
			return
		}
	}
}
