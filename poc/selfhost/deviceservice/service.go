// Package deviceservice hosts the bounded device invitation API. Network
// forwarding is explicit loopback-only opt-in through an independent broker.
package deviceservice

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"remoteapp.local/selfhost-poc/broker"
	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/trustprofile"
)

var ErrService = errors.New("device_service_unavailable")

type Config struct {
	Directory, ReleaseRoot, Service, Listen string
	MinimumEpoch                            uint64
	Broker                                  *broker.Client
	// Operator opt-in for isolated loopback integration, not public deployment.
	PrivateNetwork bool
}

func read(path string, limit int64, private bool) ([]byte, error) {
	i, e := os.Lstat(path)
	if e != nil || !i.Mode().IsRegular() || i.Size() > limit || (private && i.Mode().Perm()&0077 != 0) {
		return nil, ErrService
	}
	f, e := os.Open(path)
	if e != nil {
		return nil, ErrService
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, limit+1))
	if e != nil || int64(len(b)) > limit {
		return nil, ErrService
	}
	return b, nil
}

func load(c Config) (trustprofile.Profile, tls.Certificate, error) {
	fail := func() (trustprofile.Profile, tls.Certificate, error) {
		return trustprofile.Profile{}, tls.Certificate{}, ErrService
	}
	if !filepath.IsAbs(c.Directory) || filepath.Clean(c.Directory) != c.Directory {
		return fail()
	}
	real, e := filepath.EvalSymlinks(c.Directory)
	if e != nil || real != c.Directory {
		return fail()
	}
	subs := []string{"server-private", "client-public"}
	if c.Broker == nil {
		subs = append(subs, "server-private/authority")
	}
	for _, sub := range subs {
		i, e := os.Lstat(filepath.Join(c.Directory, sub))
		if e != nil || !i.IsDir() || i.Mode()&os.ModeSymlink != 0 || i.Mode().Perm()&0077 != 0 {
			return fail()
		}
	}
	root, e := hex.DecodeString(c.ReleaseRoot)
	if e != nil || len(root) != 32 || hex.EncodeToString(root) != c.ReleaseRoot {
		return fail()
	}
	raw, e := read(filepath.Join(c.Directory, "client-public/service-profile.json"), 8192, false)
	if e != nil {
		return fail()
	}
	p, e := trustprofile.Decode(raw)
	if e != nil {
		return fail()
	}
	caDER, e := trustprofile.Verify(p, root, c.Service, c.MinimumEpoch, time.Now())
	if e != nil {
		return fail()
	}
	u, _ := url.Parse(p.Claims.Endpoint)
	host, port, e := net.SplitHostPort(c.Listen)
	if e != nil || port != u.Port() || net.ParseIP(host) == nil {
		return fail()
	}
	if c.PrivateNetwork && (c.Broker == nil || !net.ParseIP(host).IsLoopback()) {
		return fail()
	}
	cert, e := read(filepath.Join(c.Directory, "server-private/api-cert.pem"), 16384, false)
	if e != nil {
		return fail()
	}
	key, e := read(filepath.Join(c.Directory, "server-private/api-key.pem"), 8192, true)
	if e != nil {
		return fail()
	}
	pair, e := tls.X509KeyPair(cert, key)
	if e != nil {
		return fail()
	}
	leaf, e := x509.ParseCertificate(pair.Certificate[0])
	if e != nil {
		return fail()
	}
	ca, e := x509.ParseCertificate(caDER)
	if e != nil {
		return fail()
	}
	roots := x509.NewCertPool()
	roots.AddCert(ca)
	if _, e = leaf.Verify(x509.VerifyOptions{DNSName: u.Hostname(), Roots: roots}); e != nil || leaf.NotAfter.Before(time.Unix(p.Claims.ExpiresAt, 0)) {
		return fail()
	}
	if c.Broker == nil {
		if _, e = read(filepath.Join(c.Directory, "server-private/authority/state.json"), 1<<20, true); e != nil {
			return fail()
		}
	}
	return p, pair, nil
}

type cappedListener struct {
	net.Listener
	slots chan struct{}
}
type cappedConn struct {
	net.Conn
	once    sync.Once
	release func()
}

func (c *cappedConn) Close() error { e := c.Conn.Close(); c.once.Do(c.release); return e }
func (l *cappedListener) Accept() (net.Conn, error) {
	for {
		c, e := l.Listener.Accept()
		if e != nil {
			return nil, e
		}
		select {
		case l.slots <- struct{}{}:
			return &cappedConn{Conn: c, release: func() { <-l.slots }}, nil
		default:
			c.Close()
		}
	}
}

// Run reopens existing state only. Public release root/service/floor must be
// supplied by trusted operator configuration, not read from the API itself.
// ready carries only the listening address, never keys, codes or grants.
func Run(ctx context.Context, c Config, ready func(net.Addr)) (result error) {
	p, pair, e := load(c)
	if e != nil || ctx.Err() != nil {
		return ErrService
	}
	var device http.Handler
	if c.Broker != nil {
		check, cancel := context.WithTimeout(ctx, 4*time.Second)
		identity, e := c.Broker.Call(check, broker.Request{Operation: "identity"})
		cancel()
		if e != nil || identity.Status != 200 || identity.Issuer != p.Claims.Issuer {
			return ErrService
		}
		device = broker.DeviceHandler(*c.Broker, p.Claims.Issuer)
		if c.PrivateNetwork {
			device = broker.PrivateNetworkHandler(*c.Broker, p.Claims.Issuer)
		}
	} else {
		s, e := invite.Open(filepath.Join(c.Directory, "server-private/authority"), time.Now)
		if e != nil {
			return ErrService
		}
		defer func() {
			if s.Close() != nil {
				result = ErrService
			}
		}()
		if hex.EncodeToString(s.PublicKey()) != p.Claims.Issuer {
			return ErrService
		}
		device = invite.NetworkHandler(&invite.ProvisionBackend{Store: s})
	}
	l, e := net.Listen("tcp", c.Listen)
	if e != nil {
		return ErrService
	}
	defer l.Close()
	// Default routes device requests only. The private opt-in forwards network
	// requests; all actual authorization remains inside the privileged broker.
	h := invite.NewAdmission().Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		now := time.Now().Unix()
		if r.Context().Err() != nil || now < p.Claims.IssuedAt || now >= p.Claims.ExpiresAt {
			w.Header().Set("Cache-Control", "no-store")
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		if r.URL.Path != "/v1/request" && !(c.PrivateNetwork && r.URL.Path == "/v1/network") {
			w.Header().Set("Cache-Control", "no-store")
			w.WriteHeader(http.StatusNotFound)
			return
		}
		device.ServeHTTP(w, r)
	}))
	server := &http.Server{Handler: h, ReadHeaderTimeout: 2 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second, IdleTimeout: 10 * time.Second, MaxHeaderBytes: 8192, ErrorLog: log.New(io.Discard, "", 0), TLSConfig: &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{pair}, NextProtos: []string{"http/1.1"}}}
	if c.PrivateNetwork {
		server.WriteTimeout = 20 * time.Second
	}
	server.SetKeepAlivesEnabled(false)
	lifecycle, cancel := context.WithDeadline(ctx, time.Now().Add(time.Until(time.Unix(p.Claims.ExpiresAt, 0))))
	defer cancel()
	server.BaseContext = func(net.Listener) context.Context { return lifecycle }
	done := make(chan error, 1)
	go func() {
		done <- server.Serve(tls.NewListener(&cappedListener{Listener: l, slots: make(chan struct{}, 32)}, server.TLSConfig))
	}()
	if ready != nil {
		ready(l.Addr())
	}
	select {
	case e = <-done:
		if e == http.ErrServerClosed {
			return nil
		}
		return ErrService
	case <-lifecycle.Done():
		shutdown, stop := context.WithTimeout(context.Background(), 6*time.Second)
		defer stop()
		if server.Shutdown(shutdown) != nil {
			server.Close()
			<-done
			return ErrService
		}
		if e = <-done; e != http.ErrServerClosed {
			return ErrService
		}
		return nil
	}
}
