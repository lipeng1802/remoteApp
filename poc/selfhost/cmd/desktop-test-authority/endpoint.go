//go:build darwin || linux

package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"remoteapp.local/selfhost-poc/broker"
	"remoteapp.local/selfhost-poc/productprofile"
	"remoteapp.local/selfhost-poc/trustprofile"
)

const readyFile = "authority-ready.json"

type readyReceipt struct {
	Port        uint16               `json:"port"`
	ProfileRoot string               `json:"profile_root"`
	Profile     trustprofile.Profile `json:"profile"`
}

func testTLS(now time.Time, issuer string) (tls.Certificate, readyReceipt, error) {
	var pair tls.Certificate
	var receipt readyReceipt
	endpoint, err := url.Parse(productprofile.DesktopEndpoint)
	if err != nil {
		return pair, receipt, errStage
	}
	caPublic, caKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return pair, receipt, errStage
	}
	defer clear(caKey)
	ca := &x509.Certificate{
		Subject:      pkix.Name{CommonName: "RemoteApp Desktop TEST Authority CA"},
		SerialNumber: big.NewInt(1), NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour),
		BasicConstraintsValid: true, IsCA: true, KeyUsage: x509.KeyUsageCertSign,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, ca, ca, caPublic, caKey)
	if err != nil {
		return pair, receipt, errStage
	}
	leafPublic, leafKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return pair, receipt, errStage
	}
	leaf := &x509.Certificate{
		SerialNumber: big.NewInt(2), NotBefore: ca.NotBefore, NotAfter: ca.NotAfter,
		DNSNames: []string{endpoint.Hostname()}, KeyUsage: x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leaf, ca, leafPublic, caKey)
	if err != nil {
		return pair, receipt, errStage
	}
	root, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return pair, receipt, errStage
	}
	defer clear(key)
	profile, err := trustprofile.Sign(key, trustprofile.Claims{
		Version: 1, Epoch: 1, Service: productprofile.DesktopService, Endpoint: productprofile.DesktopEndpoint,
		Certificate: base64.StdEncoding.EncodeToString(caDER), Issuer: issuer,
		IssuedAt: now.Unix(), ExpiresAt: now.Add(45 * time.Minute).Unix(),
	})
	if err != nil {
		return pair, receipt, errStage
	}
	if _, err = trustprofile.Verify(profile, root, productprofile.DesktopService, 1, now); err != nil {
		return pair, receipt, errStage
	}
	return tls.Certificate{Certificate: [][]byte{leafDER, caDER}, PrivateKey: leafKey},
		readyReceipt{ProfileRoot: hex.EncodeToString(root), Profile: profile}, nil
}

// serveEndpoint joins all HTTP handlers before stopping the private broker.
// The caller joins policy and verifier owners only after this call returns.
func serveEndpoint(ctx context.Context, root, issuer string, executor broker.Executor, networkDone <-chan struct{}) (result error) {
	pair, receipt, err := testTLS(time.Now(), issuer)
	if err != nil {
		return err
	}
	defer clear(pair.PrivateKey.(ed25519.PrivateKey))
	socket := filepath.Join(root, "broker.sock")
	uid := uint32(os.Geteuid())
	brokerLife, stopBroker := context.WithCancel(context.Background())
	brokerReady, brokerDone := make(chan struct{}), make(chan struct{})
	var brokerErr error
	go func() {
		defer close(brokerDone)
		brokerErr = broker.Serve(brokerLife, broker.Config{Socket: socket, PeerUID: uid, GroupID: uint32(os.Getegid()), Executor: executor},
			func() { close(brokerReady) })
	}()
	defer func() {
		stopBroker()
		<-brokerDone
		if brokerErr != nil {
			result = errStage
		}
	}()
	select {
	case <-ctx.Done():
		return errStage
	case <-networkDone:
		return errStage
	case <-brokerDone:
		return errStage
	case <-brokerReady:
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return errStage
	}
	defer listener.Close()
	receipt.Port = uint16(listener.Addr().(*net.TCPAddr).Port)
	life, stop := context.WithDeadline(ctx, time.Unix(receipt.Profile.Claims.ExpiresAt, 0))
	defer stop()
	server := &http.Server{
		Handler:           broker.SignedNetworkHandler(broker.Client{Socket: socket, ServerUID: uid}, issuer),
		ReadHeaderTimeout: time.Second,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      20 * time.Second,
		ErrorLog:          log.New(io.Discard, "", 0),
		BaseContext:       func(net.Listener) context.Context { return life },
	}
	httpDone := make(chan struct{})
	var httpErr error
	go func() {
		defer close(httpDone)
		httpErr = server.Serve(tls.NewListener(listener, &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{pair}}))
	}()
	defer func() {
		stop()
		err := server.Shutdown(context.Background())
		<-httpDone
		if err != nil || httpErr != http.ErrServerClosed {
			result = errStage
		}
	}()
	raw, err := json.Marshal(receipt)
	if err != nil || life.Err() != nil || writeOnce(root, readyFile, raw) != nil {
		return errStage
	}
	select {
	case <-life.Done():
		return nil
	case <-networkDone:
		return errStage
	case <-brokerDone:
		return errStage
	case <-httpDone:
		return errStage
	}
}
