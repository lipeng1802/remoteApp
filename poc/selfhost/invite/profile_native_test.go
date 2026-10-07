package invite

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"testing"

	"remoteapp.local/selfhost-poc/privatepipe"
)

func TestNativeOSSignerReleaseProfileHTTPS(t *testing.T) {
	exe := os.Getenv("REMOTEAPP_VAULT_TEST_EXECUTABLE")
	if exe == "" {
		t.Skip("explicit native vault executable required")
	}
	var raw [16]byte
	if _, e := rand.Read(raw[:]); e != nil {
		t.Fatal(e)
	}
	signer, e := privatepipe.NewSigner(exe, hex.EncodeToString(raw[:]), "", true)
	if e != nil {
		t.Fatal("native signer unavailable", e)
	}
	defer signer.Close()
	defer func() {
		if signer.DeleteCreatedIdentity() != nil {
			t.Error("test identity cleanup failed")
		}
	}()
	server, p, root, count := profileServer(t, signer)
	defer server.Close()
	client, e := NewProfileNetworkClient(p, root, p.Claims.Service, 2, signer)
	if e != nil {
		t.Fatal(e)
	}
	defer client.Close()
	if _, e = client.DeviceRequest(context.Background(), Body{Action: "register"}); e != nil {
		t.Fatal("OS signed HTTPS under release profile", e)
	}
	if count.Load() != 1 {
		t.Fatal("native request not verified")
	}
	t.Log("CHECK real OS signer + release profile + independent TLS root + HTTPS signature PASS; test credential cleanup required")
}
