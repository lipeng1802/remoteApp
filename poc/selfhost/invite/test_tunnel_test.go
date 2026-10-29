package invite

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"os"
	"remoteapp.local/selfhost-poc/trustprofile"
	"testing"
)

func TestTestTunnelRetainsTLSAndRejectsOtherDestinations(t *testing.T) {
	_, key, _ := ed25519.GenerateKey(rand.Reader)
	signer, _ := NewMemorySigner(key)
	server, p, _, _ := profileServer(t, signer)
	defer server.Close()
	root, release, _ := ed25519.GenerateKey(rand.Reader)
	p.Claims.Endpoint = "https://mk.fengmap.com:8444"
	p, _ = trustprofile.Sign(release, p.Claims)
	c, e := NewTestTunnelNetworkClient(p, root, p.Claims.Service, 1, signer)
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	tr := c.client.Transport.(*http.Transport)
	if tr.Proxy != nil || tr.TLSClientConfig.InsecureSkipVerify || tr.TLSClientConfig.RootCAs == nil {
		t.Fatal("TLS/proxy boundary changed")
	}
	if conn, e := tr.DialContext(t.Context(), "tcp", "attacker.example:8444"); e == nil {
		conn.Close()
		t.Fatal("foreign destination routed")
	}
	if _, e := NewTestTunnelNetworkClient(p, ed25519.PublicKey(make([]byte, 32)), p.Claims.Service, 1, signer); e == nil {
		t.Fatal("unsigned trust accepted")
	}
	p.Claims.Endpoint = server.URL
	p, _ = trustprofile.Sign(release, p.Claims)
	if _, e := NewTestTunnelNetworkClient(p, root, p.Claims.Service, 1, signer); e == nil {
		t.Fatal("runtime alternate endpoint accepted")
	}
}

func TestLivePrereleaseTunnelTLS(t *testing.T) {
	file := os.Getenv("REMOTEAPP_TEST_TUNNEL_PROFILE")
	if file == "" {
		t.Skip("explicit SSH tunnel and public profile required")
	}
	raw, e := os.ReadFile(file)
	if e != nil {
		t.Fatal(e)
	}
	p, e := trustprofile.Decode(raw)
	if e != nil {
		t.Fatal(e)
	}
	root, _ := hex.DecodeString("5b74a1a3b2b6c477b81016d71c38275d9fdd17d01e6c33ec7847dcd892a5198f")
	_, key, _ := ed25519.GenerateKey(rand.Reader)
	s, _ := NewMemorySigner(key)
	c, e := NewTestTunnelNetworkClient(p, root, "remoteapp-prerelease", 1, s)
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	// GET is rejected before device/replay processing; no registration or grant.
	response, e := c.client.Get(c.endpoint + "/v1/network")
	if e != nil {
		t.Fatal("verified TLS tunnel", e)
	}
	response.Body.Close()
	if response.StatusCode != 405 {
		t.Fatal("unexpected fixed negative response", response.StatusCode)
	}
	t.Log("PASS signed endpoint + fixed SSH TCP route + CA/hostname verified HTTPS 405; NOT device authorization or payload")
}
