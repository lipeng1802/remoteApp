// Package prerelease creates isolated, explicitly new pre-release material.
// It does not install trust, publish an API, or upload release private keys.
package prerelease

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/trustprofile"
)

var ErrBootstrap = errors.New("prerelease_bootstrap_failed_state_preserved")

type Anchor struct {
	ReleaseRoot  string `json:"release_root"`
	Service      string `json:"service"`
	MinimumEpoch uint64 `json:"minimum_epoch"`
}

func write(root, name string, data []byte) error {
	f, e := os.OpenFile(filepath.Join(root, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return e
	}
	_, e = f.Write(data)
	if e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e != nil {
		return e
	}
	return ce
}

func privatePEM(key any) ([]byte, error) {
	b, e := x509.MarshalPKCS8PrivateKey(key)
	if e != nil {
		return nil, e
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: b}), nil
}

func serial() (*big.Int, error) { return rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128)) }

// Create refuses any existing target and symlinked parent. On failure it keeps
// the partial directory for explicit inspection; it never silently rekeys.
// The release directory contains UNENCRYPTED owner-only private keys: it must
// remain local and be backed up/secured independently before production use.
func Create(destination, endpoint, service string, now time.Time) error {
	if !filepath.IsAbs(destination) || filepath.Clean(destination) != destination || service == "" || len(service) > 64 || strings.ContainsAny(service, "\r\n\x00") {
		return ErrBootstrap
	}
	parent := filepath.Dir(destination)
	real, e := filepath.EvalSymlinks(parent)
	if e != nil || real != parent {
		return ErrBootstrap
	}
	u, e := url.Parse(endpoint)
	if e != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.Opaque != "" || u.String() != endpoint || strings.ContainsAny(endpoint, "\r\n\x00") {
		return ErrBootstrap
	}
	// A pre-release service must explicitly name a port, not silently occupy 443.
	port, portErr := strconv.Atoi(u.Port())
	if portErr != nil || port < 1 || port > 65535 || port == 443 {
		return ErrBootstrap
	}
	if e = os.Mkdir(destination, 0700); e != nil {
		return ErrBootstrap
	}
	for _, name := range []string{"release-local-only", "server-private", "client-public"} {
		if os.Mkdir(filepath.Join(destination, name), 0700) != nil {
			return ErrBootstrap
		}
	}
	fail := func() error { return ErrBootstrap }
	pub, release, e := ed25519.GenerateKey(rand.Reader)
	if e != nil {
		return fail()
	}
	releasePEM, e := privatePEM(release)
	if e != nil {
		return fail()
	}
	if write(filepath.Join(destination, "release-local-only"), "release-root.pem", releasePEM) != nil {
		return fail()
	}
	caKey, e := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if e != nil {
		return fail()
	}
	n, e := serial()
	if e != nil {
		return fail()
	}
	ca := &x509.Certificate{SerialNumber: n, Subject: pkix.Name{CommonName: service + " pre-release API CA"}, NotBefore: now.Add(-5 * time.Minute), NotAfter: now.Add(90 * 24 * time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign}
	caDER, e := x509.CreateCertificate(rand.Reader, ca, ca, &caKey.PublicKey, caKey)
	if e != nil {
		return fail()
	}
	caPEM, e := privatePEM(caKey)
	if e != nil {
		return fail()
	}
	if write(filepath.Join(destination, "release-local-only"), "api-ca-key.pem", caPEM) != nil {
		return fail()
	}
	serverKey, e := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if e != nil {
		return fail()
	}
	n, e = serial()
	if e != nil {
		return fail()
	}
	leaf := &x509.Certificate{SerialNumber: n, Subject: pkix.Name{CommonName: u.Hostname()}, NotBefore: ca.NotBefore, NotAfter: now.Add(30 * 24 * time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	if ip := net.ParseIP(u.Hostname()); ip != nil {
		leaf.IPAddresses = []net.IP{ip}
	} else {
		leaf.DNSNames = []string{u.Hostname()}
	}
	leafDER, e := x509.CreateCertificate(rand.Reader, leaf, ca, &serverKey.PublicKey, caKey)
	if e != nil {
		return fail()
	}
	keyPEM, e := privatePEM(serverKey)
	if e != nil {
		return fail()
	}
	online := filepath.Join(destination, "server-private")
	if write(online, "api-key.pem", keyPEM) != nil || write(online, "api-cert.pem", pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER})) != nil {
		return fail()
	}
	store, e := invite.Open(filepath.Join(online, "authority"), func() time.Time { return now })
	if e != nil {
		return fail()
	}
	issuer := store.PublicKey()
	if store.Close() != nil {
		return fail()
	}
	claims := trustprofile.Claims{Version: 1, Service: service, Epoch: 1, Endpoint: endpoint, Certificate: base64.StdEncoding.EncodeToString(caDER), Issuer: hex.EncodeToString(issuer), IssuedAt: now.Unix(), ExpiresAt: leaf.NotAfter.Unix()}
	profile, e := trustprofile.Sign(release, claims)
	if e != nil {
		return fail()
	}
	if _, e = trustprofile.Verify(profile, pub, service, 1, now); e != nil {
		return fail()
	}
	profileJSON, e := json.Marshal(profile)
	if e != nil {
		return fail()
	}
	anchorJSON, e := json.Marshal(Anchor{hex.EncodeToString(pub), service, 1})
	if e != nil {
		return fail()
	}
	public := filepath.Join(destination, "client-public")
	if write(public, "service-profile.json", profileJSON) != nil || write(public, "installation-anchor.json", anchorJSON) != nil {
		return fail()
	}
	// READY is written last. It is not a cryptographic installation attestation.
	if write(destination, "READY", []byte("pre-release material only; not deployed or installed\n")) != nil {
		return fail()
	}
	return nil
}
