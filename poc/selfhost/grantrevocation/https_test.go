package grantrevocation

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/trustinstall"
	"remoteapp.local/selfhost-poc/trustprofile"
	"sync/atomic"
	"testing"
	"time"
)

func TestRealHTTPSRevocationAfterOriginalClientClosedAndLostReceipt(t *testing.T) {
	directory, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	store, e := invite.Open(filepath.Join(directory, "authority"), time.Now)
	if e != nil {
		t.Fatal(e)
	}
	defer store.Close()
	root, key, _ := ed25519.GenerateKey(rand.Reader)
	_, ownerKey, _ := ed25519.GenerateKey(rand.Reader)
	_, peerKey, _ := ed25519.GenerateKey(rand.Reader)
	owner, _ := invite.NewMemorySigner(ownerKey)
	peer, _ := invite.NewMemorySigner(peerKey)
	ca, caKey, _ := ed25519.GenerateKey(rand.Reader)
	now := time.Now().Truncate(time.Second)
	cert := &x509.Certificate{SerialNumber: big.NewInt(1), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour)}
	der, e := x509.CreateCertificate(rand.Reader, cert, cert, ca, caKey)
	if e != nil {
		t.Fatal(e)
	}
	var mutations atomic.Int32
	handler := invite.Handler(store)
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, e := io.ReadAll(io.LimitReader(r.Body, 4097))
		if e != nil {
			t.Error(e)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(raw))
		var env invite.Envelope
		json.Unmarshal(raw, &env)
		if env.Body.Action == "revoke" && mutations.Add(1) == 1 {
			receipt := httptest.NewRecorder()
			handler.ServeHTTP(receipt, r)
			if receipt.Code != 200 {
				t.Errorf("real mutation rejected %d", receipt.Code)
			}
			conn, _, e := w.(http.Hijacker).Hijack()
			if e != nil {
				t.Error(e)
				return
			}
			conn.Close()
			return
		}
		handler.ServeHTTP(w, r)
	}))
	server.TLS = &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: caKey}}, MinVersion: tls.VersionTLS12}
	server.StartTLS()
	defer server.Close()
	profile, _ := trustprofile.Sign(key, trustprofile.Claims{Version: 1, Service: "revoke-https", Epoch: 1, Endpoint: server.URL, Certificate: base64.StdEncoding.EncodeToString(der), Issuer: hex.EncodeToString(store.PublicKey()), IssuedAt: now.Add(-time.Minute).Unix(), ExpiresAt: now.Add(20 * time.Minute).Unix()})
	target, e := invite.NewProfileNetworkClient(profile, root, profile.Claims.Service, 1, owner)
	if e != nil {
		t.Fatal(e)
	}
	controller, e := invite.NewProfileNetworkClient(profile, root, profile.Claims.Service, 1, peer)
	if e != nil {
		t.Fatal(e)
	}
	defer controller.Close()
	call := func(c *invite.NetworkClient, b invite.Body) invite.Result {
		v, e := c.DeviceRequest(context.Background(), b)
		if e != nil {
			t.Fatal(e)
		}
		return v
	}
	targetID := call(target, invite.Body{Action: "register"})
	call(controller, invite.Body{Action: "register"})
	invitation := call(target, invite.Body{Action: "invite"})
	pending := call(controller, invite.Body{Action: "claim", TargetCode: targetID.DeviceCode, Code: invitation.Code})
	approved := call(target, invite.Body{Action: "approve", RequestID: pending.RequestID, Allow: true})
	if approved.Grant == nil {
		t.Fatal("grant missing")
	}
	channel, e := New(trustinstall.Specification{Test: true, Root: hex.EncodeToString(root), Service: profile.Claims.Service, MinimumEpoch: 1, Bundled: profile}, owner, false)
	if e != nil {
		t.Fatal(e)
	}
	defer channel.Close()
	if e = channel.Observe(pending.RequestID, *approved.Grant); e != nil {
		t.Fatal(e)
	}
	if e = channel.Pin(*approved.Grant); e != nil {
		t.Fatal(e)
	}
	peerChannel, e := New(trustinstall.Specification{Test: true, Root: hex.EncodeToString(root), Service: profile.Claims.Service, MinimumEpoch: 1, Bundled: profile}, peer, false)
	if e != nil {
		t.Fatal(e)
	}
	defer peerChannel.Close()
	if peerChannel.Observe(pending.RequestID, *approved.Grant) != nil || peerChannel.Pin(*approved.Grant) != nil {
		t.Fatal("peer pin")
	}
	if channel.ConfirmRevoked(context.Background()) == nil || peerChannel.ConfirmRevoked(context.Background()) == nil || mutations.Load() != 0 {
		t.Fatal("approved status became fault close authority")
	}
	target.Close() // independent transport must remain usable
	if channel.Revoke(context.Background()) == nil {
		t.Fatal("lost HTTP mutation receipt accepted")
	}
	if e = channel.Revoke(context.Background()); e != nil {
		t.Fatal("real signed status failed to resolve", e)
	}
	if mutations.Load() != 1 {
		t.Fatal("HTTP mutation blindly retried")
	}
	final := call(controller, invite.Body{Action: "status", RequestID: pending.RequestID})
	if final.Status != "revoked" || final.Grant != nil {
		t.Fatal("revocation not authoritative")
	}
	if channel.ConfirmRevoked(context.Background()) != nil || peerChannel.ConfirmRevoked(context.Background()) != nil || mutations.Load() != 1 {
		t.Fatal("both real signed status confirmations must remain read-only")
	}
}
