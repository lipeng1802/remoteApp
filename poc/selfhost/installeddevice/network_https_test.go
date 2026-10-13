package installeddevice

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
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/clientruntime"
	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/prerelease"
	"remoteapp.local/selfhost-poc/trustinstall"
	"remoteapp.local/selfhost-poc/trustprofile"
)

func TestInstalledRunnerSignedNetworkHTTPSAndTerminalJoin(t *testing.T) {
	for _, inactive := range []bool{false, true} {
		t.Run(map[bool]string{false: "normal", true: "inactive_fault"}[inactive], func(t *testing.T) {
			gate, grant, issuerKey, good := authorizationFixture(t)
			signer := gate.signer.(*testSigner)
			join := make(chan struct{})
			signer.join = join
			pub, key, _ := ed25519.GenerateKey(rand.Reader)
			now := time.Now()
			ca := &x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}}
			der, err := x509.CreateCertificate(rand.Reader, ca, ca, pub, key)
			if err != nil {
				t.Fatal(err)
			}
			server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				e, err := invite.DecodeNetworkRequest(r.Body)
				raw, _ := json.Marshal(e.Body)
				sig, _ := hex.DecodeString(e.Signature)
				if err != nil || r.URL.Path != "/v1/network" || e.PublicKey != hex.EncodeToString(gate.public) || !ed25519.Verify(gate.public, append([]byte("remoteapp-network-api-v1\nPOST /v1/network\n"), raw...), sig) {
					w.WriteHeader(403)
					return
				}
				if e.Body.Grant != grant || e.Body.Nonce == "" || e.Body.IssuedAt == 0 {
					w.WriteHeader(403)
					return
				}
				reply, _ := good(r.Context(), e.Body)
				if inactive && reply.State != nil {
					reply.State.Claims.Active = false
					reply.State.Signature = testSignature(issuerKey, "remoteapp-network-state-v1\n", reply.State.Claims)
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(reply)
			}))
			server.TLS = &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}}, MinVersion: tls.VersionTLS13}
			server.StartTLS()
			defer server.Close()
			root, releaseKey, _ := ed25519.GenerateKey(rand.Reader)
			profile, err := trustprofile.Sign(releaseKey, trustprofile.Claims{Version: 1, Service: "network-client-test", Epoch: 1, Endpoint: server.URL, Certificate: base64.StdEncoding.EncodeToString(der), Issuer: hex.EncodeToString(gate.issuer), IssuedAt: now.Add(-time.Minute).Unix(), ExpiresAt: now.Add(30 * time.Minute).Unix()})
			if err != nil {
				t.Fatal(err)
			}
			anchor, _ := json.Marshal(prerelease.Anchor{ReleaseRoot: hex.EncodeToString(root), Service: profile.Claims.Service, MinimumEpoch: 1})
			profileRaw, _ := json.Marshal(profile)
			spec, err := trustinstall.PublicSpecification(anchor, profileRaw, []byte("test-journal"), "network-fixture", now)
			if err != nil {
				t.Fatal(err)
			}
			compiled, err := trustinstall.EncodeForTrustedBuild(spec, now)
			if err != nil {
				t.Fatal(err)
			}
			release, err := trustinstall.DecodeCompiled(compiled)
			if err != nil {
				t.Fatal(err)
			}
			commands := make(chan Request, 1)
			replies := make(chan any, 4)
			done := make(chan error, 1)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			go func() {
				done <- Runner(release, func() (clientruntime.Signer, error) { return signer, nil }, commands, func(v any) error { replies <- v; return nil })(ctx, profile)
			}()
			read := func() any {
				select {
				case v := <-replies:
					return v
				case <-time.After(5 * time.Second):
					t.Fatal("HTTPS result deadline")
					return nil
				}
			}
			read()
			commands <- Request{Op: "network", Network: invite.NetworkRequestBody{Action: "grant_clock", Grant: grant}}
			if r := read().(authorizationResult); !r.Passed || r.Phase != "grant_clock" {
				t.Fatal("issuer clock not verified")
			}
			commands <- Request{Op: "network", Network: invite.NetworkRequestBody{Action: "state", Grant: grant}}
			if r := read().(authorizationResult); !r.Passed || r.Active == inactive {
				t.Fatal("network state not verified")
			}
			if !inactive {
				commands <- Request{Op: "stop"}
			}
			select {
			case <-done:
				t.Fatal("runner skipped signer join")
			case <-time.After(20 * time.Millisecond):
			}
			close(join)
			select {
			case err := <-done:
				if (err != nil) != inactive {
					t.Fatal("inactive state not terminal")
				}
			case <-time.After(5 * time.Second):
				t.Fatal("signer not joined")
			}
		})
	}
}
