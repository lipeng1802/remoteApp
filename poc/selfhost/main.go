// A fixed-payload transport experiment. This is not a product remote-control agent.
package main

import (
	"bufio"
	"context"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/netip"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"tailscale.com/envknob"
	"tailscale.com/ipn/ipnstate"
	"tailscale.com/tailcfg"
	"tailscale.com/tsnet"
)

const port = "47476"
const payload = "remoteapp-selfhost-poc-v1"

type config struct {
	Role           string `json:"role"`
	ControlURL     string `json:"control_url"`
	StateDir       string `json:"state_dir"`
	Hostname       string `json:"hostname"`
	AuthKey        string `json:"auth_key"`
	SessionToken   string `json:"session_token"`
	Peer           string `json:"peer"`
	ForceRelay     bool   `json:"force_relay"`
	CheckReject    bool   `json:"check_reject"`
	DiscoverDirect bool   `json:"discover_direct"`
	Stream         bool   `json:"stream"`
}

func parseConfig(r io.Reader) (config, []byte, error) {
	var c config
	raw, readErr := io.ReadAll(io.LimitReader(r, 8193))
	if readErr != nil || len(raw) > 8192 {
		return c, nil, errors.New("invalid_config")
	}
	d := json.NewDecoder(strings.NewReader(string(raw)))
	d.DisallowUnknownFields()
	if err := d.Decode(&c); err != nil {
		return c, nil, errors.New("invalid_config")
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return c, nil, errors.New("invalid_config")
	}
	u, err := url.Parse(c.ControlURL)
	if err != nil || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" {
		return c, nil, errors.New("invalid_control_url")
	}
	// Explicit endpoints only. Public testing retains normal certificate validation.
	local := c.ControlURL == "http://127.0.0.1:18443"
	public := c.ControlURL == "https://mk.fengmap.com:8443"
	if !local && !public {
		return c, nil, errors.New("nonlocal_control_rejected")
	}
	if c.Role != "serve" && c.Role != "probe" {
		return c, nil, errors.New("invalid_role")
	}
	if c.ForceRelay && c.DiscoverDirect || c.Stream && c.CheckReject {
		return c, nil, errors.New("invalid_test_mode")
	}
	if c.Hostname != "poc-server" && c.Hostname != "poc-client" && c.Hostname != "poc-denied" {
		return c, nil, errors.New("invalid_hostname")
	}
	// All private state stays under one explicit, separate PoC directory.
	root, err := filepath.Abs("../../artifacts/connection-poc/nodes")
	if err != nil || c.StateDir == "" {
		return c, nil, errors.New("invalid_state_dir")
	}
	c.StateDir, err = filepath.Abs(c.StateDir)
	rel, relErr := filepath.Rel(root, c.StateDir)
	if err != nil || relErr != nil || rel == "." || strings.Contains(rel, string(filepath.Separator)) || strings.HasPrefix(rel, ".") {
		return c, nil, errors.New("invalid_state_dir")
	}
	token, err := hex.DecodeString(c.SessionToken)
	if err != nil || len(token) != 32 {
		return c, nil, errors.New("invalid_session_token")
	}
	if len(c.AuthKey) == 0 || len(c.AuthKey) > 256 {
		return c, nil, errors.New("registration_key_required")
	}
	if c.Role == "probe" && c.Peer != "100.120.0.1" {
		return c, nil, errors.New("unapproved_peer")
	}
	return c, token, nil
}

func exchange(conn net.Conn, token []byte, server bool) error {
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	if server {
		got := make([]byte, 32)
		if _, err := io.ReadFull(conn, got); err != nil {
			return errors.New("request_failed")
		}
		if subtle.ConstantTimeCompare(got, token) != 1 {
			return errors.New("unauthorized")
		}
		if _, err := io.WriteString(conn, payload); err != nil {
			return errors.New("response_failed")
		}
		return nil
	}
	if n, err := conn.Write(token); err != nil || n != len(token) {
		return errors.New("request_failed")
	}
	got := make([]byte, len(payload))
	if _, err := io.ReadFull(conn, got); err != nil || string(got) != payload {
		return errors.New("response_failed")
	}
	return nil
}

// Keep a real authenticated stream alive so cancellation is tested against an
// already-open connection, not only against a listener or a subsequent dial.
func streamExchange(ctx context.Context, conn net.Conn, token []byte, server bool, notify func(string)) error {
	defer conn.Close()
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			conn.Close()
		case <-done:
		}
	}()
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	if server {
		got := make([]byte, len(token))
		if _, err := io.ReadFull(conn, got); err != nil {
			return errors.New("request_failed")
		}
		if subtle.ConstantTimeCompare(got, token) != 1 {
			return errors.New("unauthorized")
		}
		if _, err := io.WriteString(conn, payload); err != nil {
			return errors.New("response_failed")
		}
	} else {
		if _, err := conn.Write(token); err != nil {
			return errors.New("request_failed")
		}
		got := make([]byte, len(payload))
		if _, err := io.ReadFull(conn, got); err != nil || string(got) != payload {
			return errors.New("response_failed")
		}
	}
	notify("stream_open")
	for {
		conn.SetDeadline(time.Now().Add(5 * time.Second))
		if server {
			var b [1]byte
			if _, err := io.ReadFull(conn, b[:]); err != nil {
				notify("stream_closed")
				return nil
			}
			if b[0] != 1 {
				return errors.New("request_failed")
			}
			if _, err := io.WriteString(conn, payload); err != nil {
				notify("stream_closed")
				return nil
			}
		} else {
			select {
			case <-ctx.Done():
				notify("stream_closed")
				return nil
			case <-time.After(300 * time.Millisecond):
			}
			if _, err := conn.Write([]byte{1}); err != nil {
				notify("stream_closed")
				return nil
			}
			got := make([]byte, len(payload))
			if _, err := io.ReadFull(conn, got); err != nil {
				notify("stream_closed")
				return nil
			}
			if string(got) != payload {
				return errors.New("response_failed")
			}
		}
		notify("stream_tick")
	}
}

func emit(status string, ips []netip.Addr) {
	json.NewEncoder(os.Stdout).Encode(struct {
		Status    string       `json:"status"`
		Addresses []netip.Addr `json:"addresses,omitempty"`
	}{status, ips})
}

func closeWithin(closeNode func() error, timeout time.Duration) error {
	done := make(chan error, 1)
	go func() { done <- closeNode() }()
	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		return errors.New("cleanup_timeout")
	}
}

func configureNetwork(c config) {
	// Never inherit a system tailnet identity or upload logs to the hosted service.
	for _, key := range []string{"TS_AUTHKEY", "TS_AUTH_KEY", "TS_CONTROL_URL", "TS_CLIENT_SECRET", "TSNET_FORCE_LOGIN"} {
		os.Unsetenv(key)
	}
	envknob.SetNoLogsNoSupport()
	// This online-only PoC requires a current control-plane map after restart.
	// Disable disk-cache restoration and its TSMP disco-advertisement path;
	// cached startup with these pinned versions reproduced a stalled reconnect.
	envknob.Setenv("TS_USE_CACHED_NETMAP", "false")
	envknob.Setenv("TS_DEBUG_ALWAYS_USE_DERP", "false")
	envknob.Setenv("TS_DEBUG_NEVER_DIRECT_UDP", "false")
	if c.ForceRelay {
		// Keep real UDP sockets, but suppress direct discovery. ALWAYS_USE_DERP
		// replaces sockets on rebind without closing the previous dummy socket,
		// leaving WireGuard receive workers blocked during Close (v1.102.5).
		envknob.Setenv("TS_DEBUG_NEVER_DIRECT_UDP", "true")
	}
}

func hasPeer(state *ipnstate.Status, peer string) bool {
	if state == nil {
		return false
	}
	for _, p := range state.Peer {
		if !p.InNetworkMap {
			continue
		}
		for _, ip := range p.TailscaleIPs {
			if ip.String() == peer {
				return true
			}
		}
	}
	return false
}

func run(c config, token []byte) error {
	parent, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return runContext(parent, c, token)
}

func runContext(parent context.Context, c config, token []byte) (result error) {
	configureNetwork(c)
	if err := os.MkdirAll(c.StateDir, 0700); err != nil {
		return errors.New("state_failed")
	}
	st, err := os.Lstat(c.StateDir)
	if err != nil || !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
		return errors.New("state_failed")
	}
	if err := os.Chmod(c.StateDir, 0700); err != nil {
		return errors.New("state_failed")
	}
	quiet := func(string, ...any) {}
	s := &tsnet.Server{Dir: c.StateDir, Hostname: c.Hostname, ControlURL: c.ControlURL, AuthKey: c.AuthKey, UserLogf: quiet, Logf: quiet}
	defer func() {
		emit("closing", nil)
		if closeWithin(s.Close, 8*time.Second) != nil {
			emit("cleanup_timeout", nil)
			if result == nil {
				result = errors.New("cleanup_timeout")
			}
			return
		}
		emit("closed", nil)
	}()
	timeout := 180 * time.Second
	if c.Role == "probe" {
		timeout = 25 * time.Second
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	status, err := s.Up(ctx)
	if err != nil {
		return errors.New("registration_failed")
	}
	emit("registered", status.TailscaleIPs)
	if c.Role == "probe" {
		lc, err := s.LocalClient()
		if err != nil {
			return errors.New("status_failed")
		}
		state, err := lc.Status(ctx)
		if err != nil {
			return errors.New("status_failed")
		}
		// tsnet.Dial can fall back to the system network for an unknown IP.
		// Never let a lab probe accidentally use the installed system Tailscale.
		if !hasPeer(state, c.Peer) {
			return errors.New("dial_failed")
		}
		if c.DiscoverDirect {
			pingCtx, stopPing := context.WithTimeout(ctx, 8*time.Second)
			// Failure to punch through is expected on restrictive NATs. The real
			// data path below must still be identified and successfully exchange.
			for pingCtx.Err() == nil {
				result, pingErr := lc.Ping(pingCtx, netip.MustParseAddr(c.Peer), tailcfg.PingDisco)
				if pingErr == nil && result != nil && result.Endpoint != "" {
					break
				}
				select {
				case <-pingCtx.Done():
				case <-time.After(200 * time.Millisecond):
				}
			}
			stopPing()
		}
		emit("dialing", nil)
		conn, err := s.Dial(ctx, "tcp", net.JoinHostPort(c.Peer, port))
		if err != nil {
			return errors.New("dial_failed")
		}
		if c.Stream {
			return streamExchange(ctx, conn, token, false, func(status string) { emit(status, nil) })
		}
		if err := exchange(conn, token, false); err != nil {
			return err
		}
		state, err = lc.Status(ctx)
		if err != nil {
			return errors.New("status_failed")
		}
		path := "unknown"
		for _, p := range state.Peer {
			for _, ip := range p.TailscaleIPs {
				if ip.String() == c.Peer {
					if p.CurAddr != "" {
						path = "direct"
					} else if (c.ControlURL == "http://127.0.0.1:18443" && p.Relay == "lab") ||
						(c.ControlURL == "https://mk.fengmap.com:8443" && p.Relay == "remoteapp-poc") {
						path = "selfhost_relay"
					}
				}
			}
		}
		if c.ForceRelay && path != "selfhost_relay" {
			return errors.New("relay_not_verified")
		}
		emit(path, nil)
		emit("probe_passed", nil)
		if c.CheckReject {
			badToken := append([]byte(nil), token...)
			badToken[0] ^= 1
			badConn, err := s.Dial(ctx, "tcp", net.JoinHostPort(c.Peer, port))
			if err != nil {
				return errors.New("negative_dial_failed")
			}
			if exchange(badConn, badToken, false) == nil {
				return errors.New("wrong_token_accepted")
			}
			emit("wrong_token_rejected", nil)
		}
		return nil
	}
	ln, err := s.Listen("tcp", ":"+port)
	if err != nil {
		return errors.New("listen_failed")
	}
	defer ln.Close()
	go func() { <-ctx.Done(); ln.Close() }()
	emit("listening", nil)
	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return errors.New("accept_failed")
		}
		if c.Stream {
			err = streamExchange(ctx, conn, token, true, func(status string) { emit(status, nil) })
		} else {
			err = exchange(conn, token, true)
		}
		if err == nil {
			emit("exchange_passed", nil)
		} else if err.Error() == "unauthorized" {
			emit("token_rejected", nil)
		}
	}
}

// Windows has no SIGTERM equivalent for a redirected console child. This mode
// keeps a private parent-child pipe open, with bounded JSON bootstrap then a
// single stop/revoke command (or EOF if the parent disappears). No IPC listener.
func controlBootstrap(r *bufio.Reader) (config, []byte, error) {
	line, err := r.ReadSlice('\n')
	if err != nil {
		return config{}, nil, errors.New("invalid_config")
	}
	return parseConfig(strings.NewReader(string(line)))
}

func controlStop(r *bufio.Reader) error {
	_, err := controlCommand(r)
	return err
}

func controlCommand(r *bufio.Reader) (string, error) {
	line, err := r.ReadSlice('\n')
	if err == io.EOF && len(line) == 0 {
		return "stop", nil
	}
	if err != nil || (string(line) != "stop\n" && string(line) != "revoke\n") {
		return "", errors.New("invalid_control")
	}
	return strings.TrimSuffix(string(line), "\n"), nil
}

func controlledRun(input io.Reader) error {
	r := bufio.NewReaderSize(input, 8194)
	c, token, err := controlBootstrap(r)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stopped := make(chan error, 1)
	go func() {
		command, err := controlCommand(r)
		if command == "revoke" {
			emit("authorization_revoked", nil)
		}
		stopped <- err
		cancel()
	}()
	err = runContext(ctx, c, token)
	select {
	case controlErr := <-stopped:
		if controlErr != nil {
			return controlErr
		}
	default:
	}
	return err
}

func main() {
	var err error
	if len(os.Args) == 2 && os.Args[1] == "--control-stdin" {
		err = controlledRun(os.Stdin)
	} else if len(os.Args) == 1 {
		var c config
		var token []byte
		c, token, err = parseConfig(os.Stdin)
		if err == nil {
			err = run(c, token)
		}
	} else {
		err = errors.New("invalid_arguments")
	}
	if err != nil {
		emit(err.Error(), nil)
		os.Exit(1)
	}
}
