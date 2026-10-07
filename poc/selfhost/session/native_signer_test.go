package session

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/privatepipe"
	"testing"
)

// Explicit native-only test; no memory signer substitution when the vault fails.
func TestNativeVaultSessionAndProductTunnel(t *testing.T) {
	exe := os.Getenv("REMOTEAPP_VAULT_TEST_EXECUTABLE")
	if exe == "" {
		t.Skip("native executable required")
	}
	f := setup(t)
	create := func() *privatepipe.Signer {
		var scope [16]byte
		if _, err := rand.Read(scope[:]); err != nil {
			t.Fatal(err)
		}
		s, err := privatepipe.NewSigner(exe, hex.EncodeToString(scope[:]), "", true)
		if err != nil {
			t.Fatal("native creation", err)
		}
		t.Cleanup(func() {
			if err := s.DeleteCreatedIdentity(); err != nil {
				t.Error("native cleanup", err)
			}
			s.Close()
		})
		return s
	}
	target, controller := create(), create()
	request := func(s invite.DeviceSigner, b invite.Body) invite.Result {
		// The native provider signs precisely the same canonical invitation envelope.
		var n [16]byte
		rand.Read(n[:])
		b.Nonce = hex.EncodeToString(n[:])
		b.IssuedAt = f.now.Load()
		// Obtain canonical bytes without placing an OS private key in this process.
		raw, err := invite.SignRequestWithSigner(s, b)
		if err != nil {
			t.Fatal(err)
		}
		result, err := f.s.Execute(raw)
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	registered := request(target, invite.Body{Action: "register"})
	request(controller, invite.Body{Action: "register"})
	invitation := request(target, invite.Body{Action: "invite"})
	claimed := request(controller, invite.Body{Action: "claim", TargetCode: registered.DeviceCode, Code: invitation.Code})
	approved := request(target, invite.Body{Action: "approve", RequestID: claimed.RequestID, Allow: true})
	f.server.Key = nil
	f.server.Signer = target
	f.server.Grant = *approved.Grant
	f.server.Binding.Target = target.PublicKey()
	f.server.Binding.Controller = controller.PublicKey()
	f.probe = f.server
	f.probe.Signer = controller
	r := f.run(f.check)
	tick(t, r)
	r.cancel()
	ended(t, r)
	testTunnel(t, f)
}
