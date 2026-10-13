// Server-local maintenance probe. Never registers a device, approves a grant
// or receives credentials; fixed negative requests keep the sole Store intact.
package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	"remoteapp.local/selfhost-poc/broker"
	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/prerelease"
	"remoteapp.local/selfhost-poc/trustprofile"
)

// Compile-time operator pin, never sourced solely from frontend-owned files.
// Missing pin refuses even if a mutable profile/anchor pair is self-consistent.
var releaseRoot string

func run(mode string) error {
	if len(releaseRoot) != 64 || os.Geteuid() == 0 || (mode != "active" && mode != "unavailable") {
		return broker.ErrBroker
	}
	base := "/var/lib/remoteapp-prerelease/client-public/"
	b, err := os.ReadFile(base + "installation-anchor.json")
	if err != nil {
		return err
	}
	var a prerelease.Anchor
	if json.Unmarshal(b, &a) != nil || a.ReleaseRoot != releaseRoot || a.Service != "remoteapp-prerelease" || a.MinimumEpoch != 1 {
		return broker.ErrBroker
	}
	b, err = os.ReadFile(base + "service-profile.json")
	if err != nil {
		return err
	}
	p, err := trustprofile.Decode(b)
	if err != nil {
		return err
	}
	root, err := hex.DecodeString(a.ReleaseRoot)
	if err != nil {
		return err
	}
	der, err := trustprofile.Verify(p, root, a.Service, a.MinimumEpoch, time.Now())
	if err != nil {
		return err
	}
	ca, err := x509.ParseCertificate(der)
	if err != nil {
		return err
	}
	pool := x509.NewCertPool()
	pool.AddCert(ca)
	u, err := url.Parse(p.Claims.Endpoint)
	if err != nil || u.Port() != "8444" {
		return broker.ErrBroker
	}
	tr := &http.Transport{Proxy: nil, TLSClientConfig: &tls.Config{RootCAs: pool, ServerName: u.Hostname(), MinVersion: tls.VersionTLS13}, DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "tcp", "127.0.0.1:8444")
	}}
	defer tr.CloseIdleConnections()
	hc := &http.Client{Transport: tr, Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c := broker.Client{Socket: "/run/remoteapp-authority/authority.sock", ServerUID: 0}
	r, err := c.Call(ctx, broker.Request{Operation: "identity"})
	if mode == "active" && (err != nil || r.Status != 200 || r.Issuer != p.Claims.Issuer) {
		return broker.ErrBroker
	}
	if mode == "unavailable" && err == nil {
		return broker.ErrBroker
	}
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	nonce := make([]byte, 16)
	if _, err = rand.Read(nonce); err != nil {
		return err
	}
	network, err := json.Marshal(invite.SignNetworkRequest(key, invite.NetworkRequestBody{Action: "state", Nonce: hex.EncodeToString(nonce), IssuedAt: time.Now().Unix()}))
	if err != nil {
		return err
	}
	status := 403
	if mode == "unavailable" {
		status = 503
	}
	tests := []struct {
		path, method, origin string
		body                 []byte
		status               int
	}{
		{"/v1/network", "POST", "", network, status},
		// Empty device envelope fails signature verification before any durable
		// replay/budget accounting. Do not send signed unknown device requests.
		{"/v1/request", "POST", "", []byte(`{}`), status},
	}
	if mode == "active" {
		tests = append(tests, struct {
			path, method, origin string
			body                 []byte
			status               int
		}{"/v1/network", "POST", "", []byte(`{"body":{"action":"state","Action":"bind"}}`), 400}, struct {
			path, method, origin string
			body                 []byte
			status               int
		}{"/v1/network", "POST", "", bytes.Repeat([]byte("x"), 8193), 400}, struct {
			path, method, origin string
			body                 []byte
			status               int
		}{"/v1/network", "POST", "https://untrusted.invalid", network, 400}, struct {
			path, method, origin string
			body                 []byte
			status               int
		}{"/v1/network?x=1", "POST", "", network, 400}, struct {
			path, method, origin string
			body                 []byte
			status               int
		}{"/v1/network", "GET", "", nil, 405}, struct {
			path, method, origin string
			body                 []byte
			status               int
		}{"/health", "POST", "", nil, 404})
	}
	for _, test := range tests {
		req, err := http.NewRequest(test.method, p.Claims.Endpoint+test.path, bytes.NewReader(test.body))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		if test.origin != "" {
			req.Header.Set("Origin", test.origin)
		}
		response, err := hc.Do(req)
		if err != nil {
			return err
		}
		body, readErr := io.ReadAll(io.LimitReader(response.Body, 8193))
		response.Body.Close()
		if readErr != nil || len(body) > 8192 || response.StatusCode != test.status {
			return broker.ErrBroker
		}
	}
	return nil
}

func main() {
	if len(os.Args) != 2 || run(os.Args[1]) != nil {
		fmt.Println("FAIL deployed_network_check")
		os.Exit(1)
	}
	fmt.Println("PASS deployed_network_" + os.Args[1] + "_kernel_uid_verified_tls_negative_requests")
}
