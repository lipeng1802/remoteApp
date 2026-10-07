package invite

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type NetworkClient struct {
	endpoint string
	signer   DeviceSigner
	client   *http.Client
	now      func() time.Time
}
type DeviceSigner interface {
	PublicKey() ed25519.PublicKey
	Sign([]byte) ([]byte, error)
}
type memorySigner struct{ key ed25519.PrivateKey }

func (s memorySigner) PublicKey() ed25519.PublicKey    { return s.key.Public().(ed25519.PublicKey) }
func (s memorySigner) Sign(raw []byte) ([]byte, error) { return ed25519.Sign(s.key, raw), nil }

// The bootstrap root is provided by a separately trusted channel, not fetched
// from this endpoint. Private loopback endpoints only in this rollout stage.
func NewNetworkClient(endpoint string, rootDER []byte, key ed25519.PrivateKey) (*NetworkClient, error) {
	if len(key) != 64 || !bytes.Equal(key, ed25519.NewKeyFromSeed(key[:32])) {
		return nil, ErrDenied
	}
	return NewNetworkClientWithSigner(endpoint, rootDER, memorySigner{append(ed25519.PrivateKey(nil), key...)})
}
func NewNetworkClientWithSigner(endpoint string, rootDER []byte, signer DeviceSigner) (*NetworkClient, error) {
	u, e := url.Parse(endpoint)
	if e != nil || u.Scheme != "https" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.Hostname() != "127.0.0.1" || u.Port() == "" || signer == nil || len(signer.PublicKey()) != 32 {
		return nil, ErrDenied
	}
	port, err := strconv.ParseUint(u.Port(), 10, 16)
	if err != nil || port == 0 {
		return nil, ErrDenied
	}
	cert, e := x509.ParseCertificate(rootDER)
	if e != nil {
		return nil, ErrDenied
	}
	roots := x509.NewCertPool()
	roots.AddCert(cert)
	transport := &http.Transport{Proxy: nil, TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots}, DialContext: (&net.Dialer{Timeout: 3 * time.Second}).DialContext, TLSHandshakeTimeout: 3 * time.Second, MaxIdleConns: 2, MaxConnsPerHost: 2, ResponseHeaderTimeout: 13 * time.Second}
	client := &http.Client{Transport: transport, Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return ErrDenied }}
	return &NetworkClient{endpoint, signer, client, time.Now}, nil
}
func (c *NetworkClient) signed(raw []byte) (string, string, error) {
	pub := c.signer.PublicKey()
	sig, e := c.signer.Sign(raw)
	if e != nil || len(pub) != 32 || len(sig) != 64 || !ed25519.Verify(pub, raw, sig) {
		return "", "", ErrDenied
	}
	return hex.EncodeToString(pub), hex.EncodeToString(sig), nil
}
func (c *NetworkClient) Close() { c.client.CloseIdleConnections() }

// Device registration/invitation requests retain their existing signature and
// persistent nonce/rate protocol, now transported through the same strict TLS.
func (c *NetworkClient) DeviceRequest(ctx context.Context, b Body) (Result, error) {
	var nonce [16]byte
	if _, e := rand.Read(nonce[:]); e != nil {
		return Result{}, ErrDenied
	}
	b.Nonce = hex.EncodeToString(nonce[:])
	b.IssuedAt = c.now().Unix()
	pub, sig, e := c.signed(requestBytes(b))
	if e != nil {
		return Result{}, e
	}
	raw, _ := json.Marshal(Envelope{pub, b, sig})
	if len(raw) > 4096 {
		return Result{}, ErrDenied
	}
	req, e := http.NewRequestWithContext(ctx, "POST", c.endpoint+"/v1/request", bytes.NewReader(raw))
	if e != nil {
		return Result{}, ErrDenied
	}
	req.Header.Set("Content-Type", "application/json")
	res, e := c.client.Do(req)
	if e != nil {
		return Result{}, ErrDenied
	}
	defer res.Body.Close()
	if res.StatusCode != 200 || res.Header.Get("Content-Type") != "application/json" {
		return Result{}, ErrDenied
	}
	data, e := io.ReadAll(io.LimitReader(res.Body, 8193))
	if e != nil || len(data) > 8192 {
		return Result{}, ErrDenied
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var result Result
	var extra any
	if dec.Decode(&result) != nil || dec.Decode(&extra) != io.EOF {
		return Result{}, ErrDenied
	}
	if b.Action == "register" && result.DeviceID != deviceID(c.signer.PublicKey()) {
		return Result{}, ErrDenied
	}
	return result, nil
}
func (c *NetworkClient) Request(ctx context.Context, b NetworkRequestBody) (NetworkReply, error) {
	var nonce [16]byte
	if _, e := rand.Read(nonce[:]); e != nil {
		return NetworkReply{}, ErrDenied
	}
	b.Nonce = hex.EncodeToString(nonce[:])
	b.IssuedAt = c.now().Unix()
	pub, sig, e := c.signed(networkRequestBytes(b))
	if e != nil {
		return NetworkReply{}, e
	}
	raw, _ := json.Marshal(NetworkRequest{pub, b, sig})
	if len(raw) > 8192 {
		return NetworkReply{}, ErrDenied
	}
	req, e := http.NewRequestWithContext(ctx, "POST", c.endpoint+"/v1/network", bytes.NewReader(raw))
	if e != nil {
		return NetworkReply{}, ErrDenied
	}
	req.Header.Set("Content-Type", "application/json")
	res, e := c.client.Do(req)
	if e != nil {
		return NetworkReply{}, ErrDenied
	}
	defer res.Body.Close()
	if res.StatusCode != 200 || res.Header.Get("Content-Type") != "application/json" {
		return NetworkReply{}, ErrDenied
	}
	data, e := io.ReadAll(io.LimitReader(res.Body, 8193))
	if e != nil || len(data) > 8192 {
		return NetworkReply{}, ErrDenied
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var result NetworkReply
	var extra any
	if dec.Decode(&result) != nil || dec.Decode(&extra) != io.EOF {
		return NetworkReply{}, ErrDenied
	}
	switch b.Action {
	case "publish_tls":
		if result != (NetworkReply{}) {
			return NetworkReply{}, ErrDenied
		}
	case "tls_identity":
		if result.TLS == nil || result.TLS.Claims.Grant != b.Grant || result.TLS.Claims.Challenge != b.Challenge || result.State != nil || result.Proof != nil || result.Credential != nil || result.Ticket != "" {
			return NetworkReply{}, ErrDenied
		}
	case "enroll":
		if result.TLS != nil || len(result.Ticket) != 32 || result.Credential == nil || len(result.Credential.Secret) == 0 || len(result.Credential.Secret) > 256 || result.State != nil || result.Proof != nil {
			return NetworkReply{}, ErrDenied
		}
	case "bind":
		if result != (NetworkReply{}) {
			return NetworkReply{}, ErrDenied
		}
	case "state":
		if result.TLS != nil || result.State == nil || result.State.Claims.Grant != b.Grant || result.Ticket != "" || result.Credential != nil || result.Proof != nil {
			return NetworkReply{}, ErrDenied
		}
	case "clock", "grant_clock":
		if result.TLS != nil || result.Proof == nil || result.Proof.Claims.GrantID != b.Grant.Claims.ID || result.Proof.Claims.Nonce != b.Challenge || result.Ticket != "" || result.Credential != nil || result.State != nil {
			return NetworkReply{}, ErrDenied
		}
	default:
		return NetworkReply{}, ErrDenied
	}
	return result, nil
}
