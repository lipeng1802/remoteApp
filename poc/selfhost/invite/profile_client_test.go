package invite

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/trustprofile"
)

func profileServer(t *testing.T, signer DeviceSigner) (*httptest.Server, trustprofile.Profile, ed25519.PublicKey, *atomic.Int32) {
	t.Helper()
	root, release, _ := ed25519.GenerateKey(rand.Reader)
	issuer, _, _ := ed25519.GenerateKey(rand.Reader)
	ca, key, _ := ed25519.GenerateKey(rand.Reader)
	now := time.Now().Truncate(time.Second)
	cert := &x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: now.Add(-time.Hour), NotAfter: now.Add(24 * time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, DNSNames: []string{"localhost"}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}}
	der, e := x509.CreateCertificate(rand.Reader, cert, cert, ca, key)
	if e != nil {
		t.Fatal(e)
	}
	count := &atomic.Int32{}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var env Envelope
		if r.Method != "POST" || r.URL.Path != "/v1/request" || json.NewDecoder(r.Body).Decode(&env) != nil {
			w.WriteHeader(403)
			return
		}
		sig, _ := hex.DecodeString(env.Signature)
		if env.PublicKey != hex.EncodeToString(signer.PublicKey()) || !ed25519.Verify(signer.PublicKey(), requestBytes(env.Body), sig) {
			w.WriteHeader(403)
			return
		}
		count.Add(1)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Result{DeviceID: deviceID(signer.PublicKey()), DeviceCode: "123456789012"})
	}))
	server.TLS = &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}}, MinVersion: tls.VersionTLS12}
	server.StartTLS()
	claims := trustprofile.Claims{Version: 1, Service: "remoteapp-test", Epoch: 2, Endpoint: strings.Replace(server.URL, "127.0.0.1", "localhost", 1), Certificate: base64.StdEncoding.EncodeToString(der), Issuer: hex.EncodeToString(issuer), IssuedAt: now.Add(-time.Minute).Unix(), ExpiresAt: now.Add(time.Hour).Unix()}
	profile, _ := trustprofile.Sign(release, claims)
	return server, profile, root, count
}

func TestProfileHTTPSClientReleaseTrustAndExpiry(t *testing.T) {
	_, key, _ := ed25519.GenerateKey(rand.Reader)
	signer, _ := NewMemorySigner(key)
	server, p, root, count := profileServer(t, signer)
	defer server.Close()
	der, _ := base64.StdEncoding.DecodeString(p.Claims.Certificate)
	if c, e := NewNetworkClientWithSigner(p.Claims.Endpoint, der, signer); e == nil {
		c.Close()
		t.Fatal("legacy constructor widened")
	}
	c, e := NewProfileNetworkClient(p, root, p.Claims.Service, 2, signer)
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	if _, e = c.DeviceRequest(context.Background(), Body{Action: "register"}); e != nil {
		t.Fatal("release profile HTTPS request", e)
	}
	if count.Load() != 1 {
		t.Fatal("request not delivered")
	}
	bound, stop := c.BoundLifetime(context.Background())
	deadline, finite := bound.Deadline()
	stop()
	if !finite || deadline.Unix() != p.Claims.ExpiresAt {
		t.Fatal("profile not bound to lifetime")
	}
	changed := p
	changed.Claims.Endpoint = server.URL
	if c, e := NewProfileNetworkClient(changed, root, p.Claims.Service, 2, signer); e == nil {
		c.Close()
		t.Fatal("tampered destination")
	}
	wrong, _, _ := ed25519.GenerateKey(rand.Reader)
	if c, e := NewProfileNetworkClient(p, wrong, p.Claims.Service, 2, signer); e == nil {
		c.Close()
		t.Fatal("wrong release root")
	}
	c.now = func() time.Time { return time.Unix(p.Claims.ExpiresAt, 0) }
	if _, e = c.DeviceRequest(context.Background(), Body{Action: "register"}); e == nil {
		t.Fatal("expired profile used")
	}
	if count.Load() != 1 {
		t.Fatal("expired client reached server")
	}
	// An independent elapsed monotonic deadline is enforced even if a supplied
	// signing clock is still before the wall-clock profile expiry.
	c.now = time.Now
	c.profileDeadline = time.Now().Add(-time.Second)
	if _, e = c.DeviceRequest(context.Background(), Body{Action: "register"}); e == nil {
		t.Fatal("elapsed independent lifetime")
	}
	if count.Load() != 1 {
		t.Fatal("late request delivered")
	}
}
