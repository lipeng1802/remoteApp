// Private fixed-payload enrollment helper. Stdout is a parent pipe, not logs.
package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
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
	"remoteapp.local/selfhost-poc/privatepipe"
	"remoteapp.local/selfhost-poc/streamgate"
	"remoteapp.local/selfhost-poc/trustprofile"
	"tailscale.com/tsnet"
)

type config struct {
	Op, StateDir, AuthKey, NodeKey, AppKey, Verifier, Peer, Token, GrantID string
	VaultExecutable, VaultScope, ExpectedPublic                            string
	Endpoint, Certificate, Issuer                                          string
	ServiceProfile, ReleaseRoot, Service                                   string
	MinimumEpoch                                                           uint64
	profile                                                                *trustprofile.Profile
}

func resolveServiceProfile(c config) (config, error) {
	if c.ServiceProfile == "" {
		if c.ReleaseRoot != "" || c.Service != "" || c.MinimumEpoch != 0 {
			return c, invite.ErrDenied
		}
		return c, nil
	}
	if c.VaultExecutable == "" || c.AppKey != "" || c.Endpoint != "" || c.Certificate != "" || c.Issuer != "" {
		return c, invite.ErrDenied
	}
	root, e := hex.DecodeString(c.ReleaseRoot)
	if e != nil || len(root) != 32 || hex.EncodeToString(root) != c.ReleaseRoot {
		return c, invite.ErrDenied
	}
	p, e := trustprofile.Decode([]byte(c.ServiceProfile))
	if e != nil {
		return c, invite.ErrDenied
	}
	cert, e := trustprofile.Verify(p, root, c.Service, c.MinimumEpoch, time.Now())
	if e != nil {
		return c, invite.ErrDenied
	}
	c.Endpoint = p.Claims.Endpoint
	c.Certificate = base64.StdEncoding.EncodeToString(cert)
	c.Issuer = p.Claims.Issuer
	c.profile = &p
	return c, nil
}

func openNetwork(c config, signer invite.DeviceSigner) (*invite.NetworkClient, error) {
	if c.Endpoint == "" {
		if c.Certificate != "" || c.Issuer != "" {
			return nil, invite.ErrDenied
		}
		return nil, nil
	}
	issuer, e := hex.DecodeString(c.Issuer)
	grantID, ge := hex.DecodeString(c.GrantID)
	if e != nil || len(issuer) != 32 || hex.EncodeToString(issuer) != c.Issuer || ge != nil || len(grantID) != 16 || hex.EncodeToString(grantID) != c.GrantID {
		return nil, invite.ErrDenied
	}
	cert, e := base64.StdEncoding.DecodeString(c.Certificate)
	if e != nil || base64.StdEncoding.EncodeToString(cert) != c.Certificate {
		return nil, invite.ErrDenied
	}
	if c.profile != nil {
		root, _ := hex.DecodeString(c.ReleaseRoot)
		return invite.NewProfileNetworkClient(*c.profile, root, c.Service, c.MinimumEpoch, signer)
	}
	return invite.NewNetworkClientWithSigner(c.Endpoint, cert, signer)
}

func openSigner(c config) (invite.DeviceSigner, func(), error) {
	if c.VaultExecutable != "" {
		// Enrollment only reopens an explicitly pinned identity created earlier.
		if c.AppKey != "" || c.ExpectedPublic == "" {
			return nil, nil, invite.ErrDenied
		}
		s, err := privatepipe.NewSigner(c.VaultExecutable, c.VaultScope, c.ExpectedPublic, false)
		if err != nil {
			return nil, nil, err
		}
		return s, s.Close, nil
	}
	if c.VaultScope != "" || c.ExpectedPublic != "" {
		return nil, nil, invite.ErrDenied
	}
	key, err := hex.DecodeString(c.AppKey)
	if err != nil {
		return nil, nil, invite.ErrDenied
	}
	s, err := invite.NewMemorySigner(key)
	return s, func() {}, err
}

const payload = "remoteapp-enrollment-poc-v1"

var (
	errPrepareSigner = errors.New("prepare signer unavailable")
	errPrepareState  = errors.New("prepare state unavailable")
	errPrepareIntent = errors.New("prepare intent signing unavailable")
)

func output(v any) { json.NewEncoder(os.Stdout).Encode(v) }
func main() {
	if err := run(); err != nil {
		stage := "protocol"
		switch err {
		case errPrepareSigner:
			stage = "signer_open"
		case errPrepareState:
			stage = "node_state"
		case errPrepareIntent:
			stage = "intent_sign"
		}
		output(map[string]string{"status": "failed", "failure_stage": stage})
		os.Exit(1)
	}
}
func run() error {
	decode := privatepipe.NewReader(os.Stdin).Read
	var c config
	if decode(&c) != nil {
		return io.ErrUnexpectedEOF
	}
	var profileErr error
	c, profileErr = resolveServiceProfile(c)
	if profileErr != nil {
		return profileErr
	}
	app, closeSigner, e := openSigner(c)
	if e != nil {
		return errPrepareSigner
	}
	defer closeSigner()
	networkClient, e := openNetwork(c, app)
	if e != nil {
		return e
	}
	if networkClient != nil {
		defer networkClient.Close()
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
		return errPrepareState
	}
	if c.Op == "prepare" {
		if len(c.GrantID) != 32 {
			return io.ErrUnexpectedEOF
		}
		nonce := make([]byte, 16)
		if _, e = rand.Read(nonce); e != nil {
			return e
		}
		intent, e := invite.SignProvisionWithSigner(app, invite.ProvisionBody{GrantID: c.GrantID, NodeKey: pub, Nonce: hex.EncodeToString(nonce), IssuedAt: time.Now().Unix()})
		if e != nil {
			return errPrepareIntent
		}
		output(map[string]any{"status": "prepared", "node_key": pub, "intent": intent})
		return nil
	}
	if c.Op != "join" || c.NodeKey != pub || c.AuthKey == "" || len(c.AuthKey) > 256 {
		return io.ErrUnexpectedEOF
	}
	tokenBytes, e := hex.DecodeString(c.Token)
	if len(c.Verifier) != 72 || (networkClient == nil && (e != nil || len(tokenBytes) != 32)) || (networkClient != nil && c.Token != "") {
		return io.ErrUnexpectedEOF
	}
	nodeidentity.ConfigureOnline()
	s := &tsnet.Server{Dir: dir, Hostname: "poc-enrollment", AuthKey: c.AuthKey, ControlURL: "https://mk.fengmap.com:8443", Logf: func(string, ...any) {}, UserLogf: func(string, ...any) {}}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	var workers sync.WaitGroup
	var managed *streamgate.ManagedGate
	defer cancel()
	defer func() {
		cancel()
		if managed != nil {
			managed.Stop()
		}
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
		if nodeproof.ServeWithSigner(ctx, s, app, c.Verifier, func(stage string) {
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
	var tlsBridge bool
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
			bridge := tlsBridge
			gateMu.Unlock()
			if g != nil {
				streamOpened.Add(1)
				if bridge {
					g.ServeTLS(ctx, s, conn)
				} else {
					g.Serve(ctx, s, conn)
				}
				streamClosed.Add(1)
				continue
			}
			if networkClient != nil {
				conn.Close()
				continue
			} // no pre-activation token fallback
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
	var bridgeListener net.Listener
	defer func() {
		if bridgeListener != nil {
			bridgeListener.Close()
		}
	}()
	for {
		var request struct {
			Op, Peer, PeerNode, Role, Issuer, Target, Controller string
			State                                                invite.NetworkState
			Proof                                                invite.ClockProof
			Bridge                                               bool
		}
		readDone := make(chan error, 1)
		go func() { readDone <- decode(&request) }()
		select {
		case <-ctx.Done():
			// This executable owns its inherited stdin pipe. Unblock and join
			// the outstanding decoder rather than abandoning a read goroutine.
			os.Stdin.Close()
			select {
			case <-readDone:
			case <-time.After(2 * time.Second):
				return io.ErrClosedPipe
			}
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
			active, done := false, false
			if managed != nil {
				active, done = managed.Status()
			}
			output(map[string]any{"status": "count", "value": count.Load(), "challenges": challenges.Load(), "signed": signed.Load(), "stream_ticks": streamTicks.Load(), "stream_closed": streamClosed.Load(), "stream_opened": streamOpened.Load(), "probing": probing.Load(), "managed_active": active, "managed_done": done})
			continue
		}
		if request.Op == "route_status" {
			ip, err := netip.ParseAddr(request.Peer)
			if err != nil || !netip.MustParsePrefix("100.120.0.0/24").Contains(ip) {
				return io.ErrUnexpectedEOF
			}
			lc, err := s.LocalClient()
			if err != nil {
				output(nodeproof.RouteStatus{Status: "route_status"})
			} else {
				output(nodeproof.ClientRouteStatus(ctx, lc, request.PeerNode, ip))
			}
			continue
		}
		if request.Op == "watch_stop" {
			ok := managed != nil
			if ok {
				managed.Stop()
			}
			output(map[string]any{"status": "watch_stop", "passed": ok})
			continue
		}
		if request.Op == "authorize" {
			issuer, e1 := hex.DecodeString(request.Issuer)
			target, e2 := hex.DecodeString(request.Target)
			controller, e3 := hex.DecodeString(request.Controller)
			gateMu.Lock()
			ok := false
			if gate == nil && (clockIssuer == "" || clockIssuer == request.Issuer) && e1 == nil && e2 == nil && e3 == nil && (request.Role == "target" || request.Role == "controller") {
				var g *streamgate.Gate
				var e error
				if networkClient != nil {
					if request.Issuer != c.Issuer || request.State.Claims.Grant.Claims.ID != c.GrantID {
						e = invite.ErrDenied
					} else {
						clockNow, e = streamgate.Calibrate(ctx, networkClient, issuer, request.State.Claims.Grant)
						if e == nil {
							managed, e = streamgate.StartManaged(ctx, networkClient, issuer, target, controller, app, request.State.Claims.Grant, request.State.Claims.Binding, request.Role == "target", clockNow)
							if e == nil {
								g = managed.Gate()
							}
						}
					}
				} else {
					g, e = streamgate.NewWithSigner(issuer, target, controller, app, request.State, request.Role == "target", clockNow)
				}
				if e == nil {
					gate = g
					tlsBridge = request.Bridge
					ok = true
				}
			}
			gateMu.Unlock()
			output(map[string]any{"status": "authorize", "passed": ok})
			continue
		}
		if request.Op == "bridge" {
			gateMu.Lock()
			g, enabled := gate, tlsBridge
			gateMu.Unlock()
			if g == nil || !enabled || request.Role != "controller" || bridgeListener != nil {
				return io.ErrUnexpectedEOF
			}
			ln, e := net.Listen("tcp4", "127.0.0.1:0")
			if e != nil {
				return e
			}
			bridgeListener = ln
			workers.Add(1)
			go func() {
				defer workers.Done()
				for {
					conn, e := ln.Accept()
					if e != nil {
						return
					}
					streamOpened.Add(1)
					g.ConnectTLS(ctx, s, conn)
					streamClosed.Add(1)
				}
			}()
			output(map[string]any{"status": "bridge", "port": ln.Addr().(*net.TCPAddr).Port})
			continue
		}
		if request.Op == "clock_challenge" {
			if networkClient != nil {
				return invite.ErrDenied
			}
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
			if networkClient != nil {
				return invite.ErrDenied
			}
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
				fresh = g.Fresh(request.State) // diagnostic verification only in autonomous mode
				if networkClient == nil {
					ok = g.Update(request.State)
					if !ok {
						g.Deny()
					}
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
		if networkClient != nil {
			output(map[string]any{"status": "probe", "passed": false, "reason": "autonomous_mode"})
			continue
		}
		ip, e := netip.ParseAddr(request.Peer)
		if e != nil || !netip.MustParsePrefix("100.120.0.0/24").Contains(ip) {
			return io.ErrUnexpectedEOF
		}
		probeCtx, stop := context.WithTimeout(ctx, 3*time.Second)
		reason := "exchange_denied"
		stage := "local_client_failed"
		success := func() bool {
			lc, e := s.LocalClient()
			if e != nil {
				return false
			}
			conn, e := nodeproof.DialOnlyOverlay(probeCtx, lc, ip.String(), 47476)
			if e != nil {
				reason = "dial_denied"
				stage = "overlay_dial_failed"
				if e == nodeproof.ErrNoOverlayRoute {
					reason = "route_denied"
					stage = "overlay_route_missing"
				}
				return false
			}
			defer conn.Close()
			ok, current := exchangeProbe(conn, c.Token)
			stage = current
			if ok {
				reason = "none"
			}
			return ok
		}()
		stop()
		output(map[string]any{"status": "probe", "passed": success, "reason": reason, "stage": stage})
	}
}
