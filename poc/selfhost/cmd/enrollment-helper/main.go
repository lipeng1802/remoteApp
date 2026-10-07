// Private fixed-payload enrollment helper. Stdout is a parent pipe, not logs.
package main

import (
	"bufio"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/nodeidentity"
	"remoteapp.local/selfhost-poc/nodeproof"
	"remoteapp.local/selfhost-poc/streamgate"
	"tailscale.com/tsnet"
)

type config struct{ Op, StateDir, AuthKey, NodeKey, AppKey, Verifier, Peer, Token, GrantID string }

const payload = "remoteapp-enrollment-poc-v1"

func output(v any) { json.NewEncoder(os.Stdout).Encode(v) }
func main() {
	if run() != nil {
		output(map[string]string{"status": "failed"})
		os.Exit(1)
	}
}
func run() error {
	r := bufio.NewReaderSize(os.Stdin, 8194)
	decode := func(v any) error {
		line, e := r.ReadSlice('\n')
		if e != nil {
			return e
		}
		d := json.NewDecoder(strings.NewReader(string(line)))
		d.DisallowUnknownFields()
		if d.Decode(v) != nil {
			return io.ErrUnexpectedEOF
		}
		var extra any
		if d.Decode(&extra) != io.EOF {
			return io.ErrUnexpectedEOF
		}
		return nil
	}
	var c config
	if decode(&c) != nil {
		return io.ErrUnexpectedEOF
	}
	root, e := filepath.Abs("../../artifacts/connection-poc/nodes")
	if e != nil {
		return e
	}
	dir, e := filepath.Abs(c.StateDir)
	rel, re := filepath.Rel(root, dir)
	if e != nil || re != nil || rel == "." || strings.HasPrefix(rel, ".") || strings.Contains(rel, string(filepath.Separator)) {
		return io.ErrUnexpectedEOF
	}
	pub, e := nodeidentity.Prepare(dir, "https://mk.fengmap.com:8443")
	if e != nil {
		return e
	}
	if c.Op == "prepare" {
		app, e := hex.DecodeString(c.AppKey)
		if e != nil || len(app) != 64 || !ed25519.PrivateKey(app).Equal(ed25519.NewKeyFromSeed(app[:32])) || len(c.GrantID) != 32 {
			return io.ErrUnexpectedEOF
		}
		nonce := make([]byte, 16)
		if _, e = rand.Read(nonce); e != nil {
			return e
		}
		intent := invite.SignProvision(app, invite.ProvisionBody{GrantID: c.GrantID, NodeKey: pub, Nonce: hex.EncodeToString(nonce), IssuedAt: time.Now().Unix()})
		output(map[string]any{"status": "prepared", "node_key": pub, "intent": intent})
		return nil
	}
	if c.Op != "join" || c.NodeKey != pub || c.AuthKey == "" || len(c.AuthKey) > 256 {
		return io.ErrUnexpectedEOF
	}
	app, e := hex.DecodeString(c.AppKey)
	if e != nil || len(app) != 64 || !ed25519.PrivateKey(app).Equal(ed25519.NewKeyFromSeed(app[:32])) {
		return io.ErrUnexpectedEOF
	}
	tokenBytes, e := hex.DecodeString(c.Token)
	if e != nil || len(tokenBytes) != 32 || len(c.Verifier) != 72 {
		return io.ErrUnexpectedEOF
	}
	nodeidentity.ConfigureOnline()
	s := &tsnet.Server{Dir: dir, Hostname: "poc-enrollment", AuthKey: c.AuthKey, ControlURL: "https://mk.fengmap.com:8443", Logf: func(string, ...any) {}, UserLogf: func(string, ...any) {}}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	var workers sync.WaitGroup
	defer cancel()
	defer func() {
		cancel()
		done := make(chan error, 1)
		go func() { err := s.Close(); workers.Wait(); done <- err }()
		select {
		case closeErr := <-done:
			if closeErr != nil {
				output(map[string]string{"status": "cleanup_failed"})
				os.Exit(1)
			}
			output(map[string]string{"status": "closed"})
		case <-time.After(8 * time.Second):
			output(map[string]string{"status": "cleanup_timeout"})
			os.Exit(1)
		}
	}()
	st, e := s.Up(ctx)
	if e != nil {
		return e
	}
	if st.Self == nil || st.Self.PublicKey.String() != pub {
		return io.ErrUnexpectedEOF
	}
	output(map[string]any{"status": "registered", "node_key": pub, "addresses": st.TailscaleIPs})
	var challenges, signed atomic.Int64
	workers.Add(1)
	go func() {
		defer workers.Done()
		if nodeproof.ServeWithReport(ctx, s, app, c.Verifier, func(stage string) {
			if stage == "challenge" {
				challenges.Add(1)
			}
			if stage == "signed" {
				signed.Add(1)
			}
		}) != nil {
			cancel()
		}
	}()
	ln, e := s.Listen("tcp", ":47476")
	if e != nil {
		return e
	}
	defer ln.Close()
	go func() { <-ctx.Done(); ln.Close() }()
	var count atomic.Int64
	var gateMu sync.Mutex
	var gate *streamgate.Gate
	var streamTicks, streamClosed, streamOpened atomic.Int64
	var probing atomic.Bool
	workers.Add(1)
	go func() {
		defer workers.Done()
		for {
			conn, e := ln.Accept()
			if e != nil {
				return
			}
			gateMu.Lock()
			g := gate
			gateMu.Unlock()
			if g != nil {
				streamOpened.Add(1)
				g.Serve(ctx, s, conn)
				streamClosed.Add(1)
				continue
			}
			conn.SetDeadline(time.Now().Add(2 * time.Second))
			b := make([]byte, len(c.Token))
			if _, e = io.ReadFull(conn, b); e == nil && string(b) == c.Token {
				gateMu.Lock()
				// Activation is irreversible for this process. A token exchange
				// accepted just before activation must not bypass the new gate.
				if gate == nil {
					if _, e = io.WriteString(conn, payload); e == nil {
						count.Add(1)
					}
				}
				gateMu.Unlock()
			}
			conn.Close()
		}
	}()
	output(map[string]string{"status": "ready"})
	clockNow := time.Now
	clockNonce := ""
	clockIssuer := ""
	var clockStart time.Time
	for {
		var request struct {
			Op, Peer, Role, Issuer, Target, Controller string
			State                                      invite.NetworkState
			Proof                                      invite.ClockProof
		}
		readDone := make(chan error, 1)
		go func() { readDone <- decode(&request) }()
		select {
		case <-ctx.Done():
			return nil
		case e := <-readDone:
			if e == io.EOF {
				return nil
			}
			if e != nil {
				return e
			}
		}
		if request.Op == "stop" {
			return nil
		}
		if request.Op == "count" {
			output(map[string]any{"status": "count", "value": count.Load(), "challenges": challenges.Load(), "signed": signed.Load(), "stream_ticks": streamTicks.Load(), "stream_closed": streamClosed.Load(), "stream_opened": streamOpened.Load(), "probing": probing.Load()})
			continue
		}
		if request.Op == "authorize" {
			issuer, e1 := hex.DecodeString(request.Issuer)
			target, e2 := hex.DecodeString(request.Target)
			controller, e3 := hex.DecodeString(request.Controller)
			gateMu.Lock()
			ok := false
			if gate == nil && (clockIssuer == "" || clockIssuer == request.Issuer) && e1 == nil && e2 == nil && e3 == nil && (request.Role == "target" || request.Role == "controller") {
				g, e := streamgate.New(issuer, target, controller, app, request.State, request.Role == "target", clockNow)
				if e == nil {
					gate = g
					ok = true
				}
			}
			gateMu.Unlock()
			output(map[string]any{"status": "authorize", "passed": ok})
			continue
		}
		if request.Op == "clock_challenge" {
			gateMu.Lock()
			enabled := gate != nil
			gateMu.Unlock()
			if enabled || clockNonce != "" {
				return io.ErrUnexpectedEOF
			}
			n := make([]byte, 32)
			if _, e := rand.Read(n); e != nil {
				return e
			}
			clockNonce = hex.EncodeToString(n)
			clockStart = time.Now()
			output(map[string]any{"status": "clock_challenge", "challenge": clockNonce})
			continue
		}
		if request.Op == "clock" {
			issuer, e := hex.DecodeString(request.Issuer)
			gateMu.Lock()
			enabled := gate != nil
			gateMu.Unlock()
			ok := false
			if !enabled && clockNonce != "" && e == nil {
				now, e := invite.AuthorityClock(issuer, request.Proof, c.GrantID, clockNonce, clockStart)
				if e == nil {
					clockNow = now
					clockIssuer = request.Issuer
					ok = true
				}
			}
			clockNonce = ""
			output(map[string]any{"status": "clock", "passed": ok})
			continue
		}
		if request.Op == "state" || request.Op == "stream" {
			gateMu.Lock()
			g := gate
			gateMu.Unlock()
			ok := false
			fresh := false
			if g != nil && request.Op == "state" {
				fresh = g.Fresh(request.State)
				ok = g.Update(request.State)
				if !ok {
					g.Deny()
				}
			}
			if g != nil && request.Op == "stream" && probing.CompareAndSwap(false, true) {
				ok = true
				workers.Add(1)
				go func() {
					defer workers.Done()
					g.Probe(ctx, s, func() { streamTicks.Add(1) })
					streamClosed.Add(1)
					probing.Store(false)
				}()
			}
			output(map[string]any{"status": request.Op, "passed": ok, "fresh": fresh})
			continue
		}
		if request.Op != "probe" {
			return io.ErrUnexpectedEOF
		}
		ip, e := netip.ParseAddr(request.Peer)
		if e != nil || !netip.MustParsePrefix("100.120.0.0/24").Contains(ip) {
			return io.ErrUnexpectedEOF
		}
		probeCtx, stop := context.WithTimeout(ctx, 3*time.Second)
		reason := "exchange_denied"
		success := func() bool {
			lc, e := s.LocalClient()
			if e != nil {
				return false
			}
			conn, e := nodeproof.DialOnlyOverlay(probeCtx, lc, ip.String(), 47476)
			if e != nil {
				reason = "dial_denied"
				if e == nodeproof.ErrNoOverlayRoute {
					reason = "route_denied"
				}
				return false
			}
			defer conn.Close()
			conn.SetDeadline(time.Now().Add(2 * time.Second))
			if _, e = io.WriteString(conn, c.Token); e != nil {
				return false
			}
			b := make([]byte, len(payload))
			_, e = io.ReadFull(conn, b)
			return e == nil && string(b) == payload
		}()
		stop()
		output(map[string]any{"status": "probe", "passed": success, "reason": reason})
	}
}
