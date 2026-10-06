package main

import (
	"bufio"
	"encoding/json"
	"net"
	"net/netip"
	"strings"
	"testing"
	"time"

	"tailscale.com/envknob"
	"tailscale.com/ipn/ipnstate"
	"tailscale.com/types/key"
)

func TestPrivateControlPipe(t *testing.T) {
	raw, _ := json.Marshal(validConfig())
	for _, command := range []string{"stop\n", ""} {
		r := bufio.NewReaderSize(strings.NewReader(string(raw)+"\n"+command), 8194)
		if _, _, err := controlBootstrap(r); err != nil {
			t.Fatal(err)
		}
		if err := controlStop(r); err != nil {
			t.Fatal(err)
		}
	}
	for _, command := range []string{"logout\n", "stop", strings.Repeat("s", 9000) + "\n"} {
		if controlStop(bufio.NewReaderSize(strings.NewReader(command), 8194)) == nil {
			t.Fatal("invalid command accepted")
		}
	}
	for _, bootstrap := range []string{string(raw), string(raw) + "{}\n", strings.Repeat(" ", 9000) + "\n"} {
		if _, _, err := controlBootstrap(bufio.NewReaderSize(strings.NewReader(bootstrap), 8194)); err == nil {
			t.Fatal("invalid bootstrap accepted")
		}
	}
}

func TestNetworkLifecyclePolicy(t *testing.T) {
	for _, force := range []bool{true, false} {
		t.Setenv("TS_DEBUG_ALWAYS_USE_DERP", "true")
		t.Setenv("TS_USE_CACHED_NETMAP", "true")
		configureNetwork(config{ForceRelay: force})
		if envknob.Bool("TS_DEBUG_ALWAYS_USE_DERP") || envknob.BoolDefaultTrue("TS_USE_CACHED_NETMAP") {
			t.Fatal("unsafe lifecycle mode retained")
		}
		if envknob.Bool("TS_DEBUG_NEVER_DIRECT_UDP") != force {
			t.Fatal("relay test mode not explicit")
		}
	}
}

func TestUnknownPeerFailsClosed(t *testing.T) {
	peer := &ipnstate.PeerStatus{TailscaleIPs: []netip.Addr{netip.MustParseAddr("100.120.0.1")}}
	state := &ipnstate.Status{Peer: map[key.NodePublic]*ipnstate.PeerStatus{key.NewNode().Public(): peer}}
	if hasPeer(nil, "100.120.0.1") || hasPeer(state, "100.120.0.1") {
		t.Fatal("peer outside current map accepted")
	}
	peer.InNetworkMap = true
	if !hasPeer(state, "100.120.0.1") || hasPeer(state, "100.120.0.3") {
		t.Fatal("peer membership check failed")
	}
}

func validConfig() config {
	return config{Role: "probe", ControlURL: "http://127.0.0.1:18443", StateDir: "../../artifacts/connection-poc/nodes/test", Hostname: "poc-client", AuthKey: "test-only", SessionToken: strings.Repeat("ab", 32), Peer: "100.120.0.1"}
}

func TestCloseWithin(t *testing.T) {
	if err := closeWithin(func() error { return nil }, time.Second); err != nil {
		t.Fatal(err)
	}
	blocked := make(chan struct{})
	released := make(chan struct{})
	err := closeWithin(func() error { <-blocked; close(released); return nil }, 20*time.Millisecond)
	close(blocked)
	<-released
	if err == nil || err.Error() != "cleanup_timeout" {
		t.Fatal("blocking library close not bounded")
	}
}

func TestConfigBoundaries(t *testing.T) {
	cases := []struct {
		name   string
		change func(*config)
	}{
		{"hosted", func(c *config) { c.ControlURL = "https://login.tailscale.com" }},
		{"public-http", func(c *config) { c.ControlURL = "http://mk.fengmap.com:8443" }},
		{"public-wrong-port", func(c *config) { c.ControlURL = "https://mk.fengmap.com" }},
		{"public-path", func(c *config) { c.ControlURL = "https://mk.fengmap.com:8443/test" }},
		{"public-query", func(c *config) { c.ControlURL = "https://mk.fengmap.com:8443?key=secret" }},
		{"public-empty-query", func(c *config) { c.ControlURL = "https://mk.fengmap.com:8443?" }},
		{"public-empty-fragment", func(c *config) { c.ControlURL = "https://mk.fengmap.com:8443#" }},
		{"public-suffix", func(c *config) { c.ControlURL = "https://mk.fengmap.com.evil:8443" }},
		{"missing-control", func(c *config) { c.ControlURL = "" }},
		{"url-secret", func(c *config) { c.ControlURL = "http://secret@127.0.0.1:18443" }},
		{"wrong-peer", func(c *config) { c.Peer = "100.73.4.118" }},
		{"shared-state", func(c *config) { c.StateDir = "../../artifacts/connection-poc/nodes" }},
		{"outside-state", func(c *config) { c.StateDir = "/tmp" }},
		{"no-token", func(c *config) { c.SessionToken = "" }},
		{"no-key", func(c *config) { c.AuthKey = "" }},
	}
	good := validConfig()
	public := validConfig()
	public.ControlURL = "https://mk.fengmap.com:8443"
	publicRaw, _ := json.Marshal(public)
	if _, _, err := parseConfig(strings.NewReader(string(publicRaw))); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(good)
	if _, _, err := parseConfig(strings.NewReader(string(raw))); err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := validConfig()
			tc.change(&c)
			b, _ := json.Marshal(c)
			if _, _, err := parseConfig(strings.NewReader(string(b))); err == nil {
				t.Fatal("accepted")
			}
		})
	}
	for _, suffix := range []string{"{}", "garbage"} {
		if _, _, err := parseConfig(strings.NewReader(string(raw) + suffix)); err == nil {
			t.Fatal("accepted trailing input")
		}
	}
	if _, _, err := parseConfig(strings.NewReader(string(raw) + strings.Repeat(" ", 8192))); err == nil {
		t.Fatal("accepted oversized bootstrap")
	}
	if _, _, err := parseConfig(strings.NewReader(`{"auth_key":"SECRET","unexpected":true}`)); err == nil || strings.Contains(err.Error(), "SECRET") {
		t.Fatal("unsafe error")
	}
}

func TestExchange(t *testing.T) {
	for _, matches := range []bool{true, false} {
		a, b := net.Pipe()
		token := make([]byte, 32)
		clientToken := make([]byte, 32)
		if !matches {
			clientToken[0] = 1
		}
		result := make(chan error, 1)
		go func() { result <- exchange(a, token, true) }()
		clientErr := exchange(b, clientToken, false)
		serverErr := <-result
		if matches && (clientErr != nil || serverErr != nil) {
			t.Fatal(clientErr, serverErr)
		}
		if !matches && (clientErr == nil || serverErr == nil) {
			t.Fatal("wrong token accepted")
		}
	}
}
