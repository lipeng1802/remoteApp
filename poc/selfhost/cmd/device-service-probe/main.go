// Read-only signed-profile HTTPS probe via an explicit loopback endpoint.
// It sends only malformed requests: no registration or durable device writes.
package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"remoteapp.local/selfhost-poc/trustprofile"
	"time"
)

func run() error {
	if len(os.Args) != 5 && len(os.Args) != 6 {
		return fmt.Errorf("arguments")
	}
	denied := 403
	if len(os.Args) == 6 {
		if os.Args[5] != "authority-down" {
			return fmt.Errorf("arguments")
		}
		denied = 503
	}
	root, e := hex.DecodeString(os.Args[2])
	if e != nil || hex.EncodeToString(root) != os.Args[2] {
		return fmt.Errorf("root")
	}
	i, e := os.Stat(os.Args[1])
	if e != nil || i.Size() > 8192 {
		return fmt.Errorf("profile")
	}
	b, e := os.ReadFile(os.Args[1])
	if e != nil {
		return e
	}
	p, e := trustprofile.Decode(b)
	if e != nil {
		return e
	}
	der, e := trustprofile.Verify(p, root, os.Args[3], 1, time.Now())
	if e != nil {
		return e
	}
	ca, e := x509.ParseCertificate(der)
	if e != nil {
		return e
	}
	pool := x509.NewCertPool()
	pool.AddCert(ca)
	u, _ := url.Parse(p.Claims.Endpoint)
	host, _, e := net.SplitHostPort(os.Args[4])
	if e != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() {
		return fmt.Errorf("forward")
	}
	tr := &http.Transport{Proxy: nil, TLSClientConfig: &tls.Config{RootCAs: pool, ServerName: u.Hostname(), MinVersion: tls.VersionTLS13}, DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		return (&net.Dialer{Timeout: 3 * time.Second}).DialContext(ctx, "tcp", os.Args[4])
	}}
	defer tr.CloseIdleConnections()
	client := &http.Client{Transport: tr, Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	for _, v := range []struct {
		path, body string
		status     int
	}{{"/v1/request", "{}", denied}, {"/v1/request", `{"body":{"action":"register","Action":"invite"}}`, 400}, {"/v1/network", "{}", 404}, {"/health", "{}", 404}} {
		r, e := client.Post(p.Claims.Endpoint+v.path, "application/json", bytes.NewBufferString(v.body))
		if e != nil {
			return e
		}
		_, e = io.Copy(io.Discard, io.LimitReader(r.Body, 4097))
		r.Body.Close()
		if e != nil || r.StatusCode != v.status {
			return fmt.Errorf("boundary")
		}
	}
	fmt.Printf("PASS release-profile HTTPS via explicit loopback; device status=%d; duplicate/network/health rejected; no device writes\n", denied)
	return nil
}

func main() {
	if run() != nil {
		fmt.Println("FAIL device_service_probe")
		os.Exit(1)
	}
}
