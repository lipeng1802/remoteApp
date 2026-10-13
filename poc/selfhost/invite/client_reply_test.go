package invite

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPClientRejectsDuplicateAndCaseDuplicateReplies(t *testing.T) {
	for _, c := range []struct {
		raw              string
		network, allowed bool
	}{
		{`{"status":"idle"}`, false, true},
		{`{"status":"idle","status":"approved"}`, false, false},
		{`{"status":"idle","STATUS":"approved"}`, false, false},
		{`{"grant":{"claims":{"id":"a","ID":"b"}}}`, false, false},
		{`{"network_state":{}}`, true, true}, // Parsed, not cryptographically authorized.
		{`{"network_state":{},"NETWORK_STATE":{}}`, true, false},
		{`{"network_state":{"claims":{"active":false,"ACTIVE":true}}}`, true, false},
		{`{"network_state":{},"unknown":"secret"}`, true, false},
		{`{"network_state":{}} {}`, true, false},
	} {
		t.Run(c.raw, func(t *testing.T) {
			s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(c.raw))
			}))
			defer s.Close()
			_, key, _ := ed25519.GenerateKey(rand.Reader)
			signer, _ := NewMemorySigner(key)
			client, err := NewNetworkClientWithSigner(s.URL, s.Certificate().Raw, signer)
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			if c.network {
				_, err = client.Request(context.Background(), NetworkRequestBody{Action: "state"})
			} else {
				_, err = client.DeviceRequest(context.Background(), Body{Action: "status"})
			}
			if (err == nil) != c.allowed {
				t.Fatal("ambiguous reply interpretation")
			}
		})
	}
}
