// Package devicevalidation is TEST-only native admission acceptance. It never
// imports production authority state or supplies runtime trust to the client.
package devicevalidation

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/trustinstall"
	"remoteapp.local/selfhost-poc/trustprofile"
)

var ErrValidation = errors.New("installed_device_native_validation_failed")

type TLSFixture struct{ Certificate, Key string }

// NewMaterial generates throwaway TEST signing roots only in memory. Only a
// loopback fixture leaf key is embedded in the validator, not a CA/release key.
func NewMaterial(journal []byte, now time.Time) (trustinstall.Specification, TLSFixture, error) {
	return newMaterial(journal, now, false)
}

// NewNetworkMaterial reuses ONLY the throwaway TEST leaf as the fixture issuer.
// This is not production key-role separation. No release/CA key is exported.
func NewNetworkMaterial(journal []byte, now time.Time) (trustinstall.Specification, TLSFixture, error) {
	return newMaterial(journal, now, true)
}
func newMaterial(journal []byte, now time.Time, network bool) (trustinstall.Specification, TLSFixture, error) {
	if len(journal) == 0 {
		return trustinstall.Specification{}, TLSFixture{}, ErrValidation
	}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return trustinstall.Specification{}, TLSFixture{}, err
	}
	endpoint := "https://" + l.Addr().String()
	l.Close()
	root, rootKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return trustinstall.Specification{}, TLSFixture{}, err
	}
	defer func() {
		for i := range rootKey {
			rootKey[i] = 0
		}
	}()
	caPub, caKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return trustinstall.Specification{}, TLSFixture{}, err
	}
	defer func() {
		for i := range caKey {
			caKey[i] = 0
		}
	}()
	leafPub, leafKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return trustinstall.Specification{}, TLSFixture{}, err
	}
	defer func() {
		for i := range leafKey {
			leafKey[i] = 0
		}
	}()
	issuer, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return trustinstall.Specification{}, TLSFixture{}, err
	}
	if network {
		issuer = leafPub
	}
	ca := &x509.Certificate{SerialNumber: big.NewInt(1), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(24 * time.Hour)}
	caDER, err := x509.CreateCertificate(rand.Reader, ca, ca, caPub, caKey)
	if err != nil {
		return trustinstall.Specification{}, TLSFixture{}, err
	}
	leaf := &x509.Certificate{SerialNumber: big.NewInt(2), IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, KeyUsage: x509.KeyUsageDigitalSignature, NotBefore: ca.NotBefore, NotAfter: ca.NotAfter}
	leafDER, err := x509.CreateCertificate(rand.Reader, leaf, ca, leafPub, caKey)
	if err != nil {
		return trustinstall.Specification{}, TLSFixture{}, err
	}
	pk, err := x509.MarshalPKCS8PrivateKey(leafKey)
	if err != nil {
		return trustinstall.Specification{}, TLSFixture{}, err
	}
	profile, err := trustprofile.Sign(rootKey, trustprofile.Claims{Version: 1, Service: "installed-device-native-test", Epoch: 1, Endpoint: endpoint, Certificate: base64.StdEncoding.EncodeToString(caDER), Issuer: hex.EncodeToString(issuer), IssuedAt: now.Add(-time.Minute).Unix(), ExpiresAt: now.Add(23 * time.Hour).Unix()})
	if err != nil {
		return trustinstall.Specification{}, TLSFixture{}, err
	}
	var raw [16]byte
	if _, err = rand.Read(raw[:]); err != nil {
		return trustinstall.Specification{}, TLSFixture{}, err
	}
	digest := sha256.Sum256(journal)
	spec := trustinstall.Specification{Version: 1, Channel: "device-validation-" + hex.EncodeToString(raw[:]), Root: hex.EncodeToString(root), Service: profile.Claims.Service, MinimumEpoch: 1, HelperDigest: hex.EncodeToString(digest[:]), Bundled: profile, Test: true}
	fixture := TLSFixture{Certificate: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER})), Key: string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pk}))}
	return spec, fixture, nil
}

type verifier struct {
	mu                 sync.Mutex
	public             ed25519.PublicKey
	nonces             map[string]bool
	registers, invites int
	last               string
}

func (v *verifier) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	v.mu.Lock()
	v.last = "surface"
	v.mu.Unlock()
	w.Header().Set("Cache-Control", "no-store")
	if r.TLS == nil || r.Method != "POST" || r.URL.Path != "/v1/request" || r.URL.RawQuery != "" || r.Header.Get("Content-Type") != "application/json" {
		w.WriteHeader(400)
		return
	}
	env, err := invite.DecodeDeviceRequest(http.MaxBytesReader(w, r.Body, 4096))
	if err != nil {
		w.WriteHeader(400)
		return
	}
	pub, pe := hex.DecodeString(env.PublicKey)
	sig, se := hex.DecodeString(env.Signature)
	nonce, ne := hex.DecodeString(env.Body.Nonce)
	body, _ := json.Marshal(env.Body)
	v.mu.Lock()
	defer v.mu.Unlock()
	v.last = "signature"
	if pe != nil || se != nil || ne != nil || len(pub) != 32 || len(sig) != 64 || len(nonce) != 16 || env.PublicKey != hex.EncodeToString(v.public) || env.Signature != hex.EncodeToString(sig) || env.Body.Nonce != hex.EncodeToString(nonce) || v.nonces[env.Body.Nonce] || env.Body.IssuedAt < time.Now().Unix()-30 || env.Body.IssuedAt > time.Now().Unix()+30 || !ed25519.Verify(v.public, append([]byte("remoteapp-invite-request-v1\n"), body...), sig) {
		w.WriteHeader(403)
		return
	}
	v.nonces[env.Body.Nonce] = true
	v.last = "verified"
	id := sha256.Sum256(v.public)
	result := invite.Result{}
	switch env.Body.Action {
	case "register":
		v.last = "registered"
		v.registers++
		result.DeviceID = hex.EncodeToString(id[:])
		result.DeviceCode = "native-fixture-code"
	case "invite":
		v.last = "invited"
		v.invites++
		result.Code = "native-fixture-invite"
	default:
		w.WriteHeader(403)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func fixturePair(release trustinstall.Release, fixture TLSFixture) (tls.Certificate, error) {
	spec := release.Specification()
	if !spec.Test || !strings.HasPrefix(spec.Channel, "device-validation-") {
		return tls.Certificate{}, ErrValidation
	}
	pair, err := tls.X509KeyPair([]byte(fixture.Certificate), []byte(fixture.Key))
	if err != nil {
		return tls.Certificate{}, ErrValidation
	}
	leaf, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		return tls.Certificate{}, ErrValidation
	}
	anchor := release.Anchor()
	der, err := trustprofile.Verify(spec.Bundled, anchor.Root[:], anchor.Service, anchor.MinimumEpoch, time.Now())
	if err != nil {
		return tls.Certificate{}, ErrValidation
	}
	ca, err := x509.ParseCertificate(der)
	if err != nil {
		return tls.Certificate{}, ErrValidation
	}
	roots := x509.NewCertPool()
	roots.AddCert(ca)
	if _, err = leaf.Verify(x509.VerifyOptions{DNSName: "127.0.0.1", Roots: roots}); err != nil {
		return tls.Certificate{}, ErrValidation
	}
	return pair, nil
}
