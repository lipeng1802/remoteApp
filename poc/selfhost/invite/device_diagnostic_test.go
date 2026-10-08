package invite

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type failedDiagnosticSigner struct{ DeviceSigner }

func (failedDiagnosticSigner) Sign([]byte) ([]byte, error) {
	return nil, errors.New("private material must not propagate")
}

func TestDeviceDiagnosticsRemainDeniedAndSanitized(t *testing.T) {
	_, key, _ := ed25519.GenerateKey(rand.Reader)
	signer, _ := NewMemorySigner(key)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "private response must not propagate", http.StatusForbidden)
	}))
	client, err := NewNetworkClientWithSigner(server.URL, server.Certificate().Raw, signer)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	_, err = client.DeviceRequest(context.Background(), Body{Action: "register"})
	if err != ErrDeviceResponse || !errors.Is(err, ErrDenied) {
		t.Fatal("response category lost denial semantics")
	}
	client.signer = failedDiagnosticSigner{signer}
	_, err = client.DeviceRequest(context.Background(), Body{Action: "register"})
	if err != ErrDeviceSigner || !errors.Is(err, ErrDenied) {
		t.Fatal("signer category lost denial semantics")
	}
	client.signer = signer
	server.Close()
	_, err = client.DeviceRequest(context.Background(), Body{Action: "register"})
	if err != ErrDeviceTransport || !errors.Is(err, ErrDenied) {
		t.Fatal("transport category lost denial semantics")
	}
}
