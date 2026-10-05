// Loopback-only DERP fixture with an ephemeral, explicitly pinned TLS certificate.
package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"net"
	"net/http"
	"os"
	"time"

	"tailscale.com/derp/derpserver"
	"tailscale.com/types/key"
)

func run() error {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return err
	}
	template := x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: "remoteapp-local-derp"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	cert, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return err
	}
	ln, err := net.Listen("tcp", "127.0.0.1:18444")
	if err != nil {
		return err
	}
	defer ln.Close()
	tlsListener := tls.NewListener(ln, &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{{Certificate: [][]byte{cert}, PrivateKey: priv}}})
	quiet := func(string, ...any) {}
	relay := derpserver.New(key.NewNode(), quiet)
	defer relay.Close()
	mux := http.NewServeMux()
	mux.Handle("/derp", derpserver.Handler(relay))
	hash := sha256.Sum256(cert)
	json.NewEncoder(os.Stdout).Encode(map[string]string{"status": "listening", "cert_name": "sha256-raw:" + hex.EncodeToString(hash[:])})
	server := http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	return server.Serve(tlsListener)
}

func main() {
	if run() != nil {
		json.NewEncoder(os.Stdout).Encode(map[string]string{"status": "derp_failed"})
		os.Exit(1)
	}
}
