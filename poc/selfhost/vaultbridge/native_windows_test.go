package vaultbridge

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/privatepipe"
)

func TestInteractiveVaultBridge(t *testing.T) {
	folder := os.Getenv("REMOTEAPP_INTERACTIVE_TEST_FOLDER")
	if folder == "" {
		t.Skip("interactive Windows bundle required")
	}
	broker := exec.Command(filepath.Join(folder, "interactive-vault.exe"))
	in, e := broker.StdinPipe()
	if e != nil {
		t.Fatal(e)
	}
	out, e := broker.StdoutPipe()
	if e != nil {
		t.Fatal(e)
	}
	if broker.Start() != nil {
		t.Fatal("broker launch")
	}
	exited := make(chan error, 1)
	go func() { exited <- broker.Wait() }()
	stopped := false
	defer func() {
		if !stopped {
			json.NewEncoder(in).Encode(map[string]string{"Op": "stop"})
			in.Close()
			select {
			case <-exited:
			case <-time.After(10 * time.Second):
				broker.Process.Kill()
				<-exited
			}
		}
	}()
	json.NewEncoder(in).Encode(map[string]string{"VaultExecutable": filepath.Join(folder, "identity-vault.exe")})
	var ready struct {
		Status, Pipe, Scope string
		Public              string `json:"public_key"`
	}
	if privatepipe.NewReader(out).Read(&ready) != nil || ready.Status != "ready" {
		t.Fatal("interactive OS identity unavailable")
	}
	t.Setenv("REMOTEAPP_INTERACTIVE_VAULT_PIPE", ready.Pipe)
	relay := filepath.Join(folder, "vault-relay.exe")
	for i := 0; i < 2; i++ {
		s, e := privatepipe.NewSigner(relay, ready.Scope, ready.Public, false)
		if e != nil {
			t.Fatal("pinned relay reopen", e)
		}
		if hex.EncodeToString(s.PublicKey()) != ready.Public {
			t.Fatal("changed identity")
		}
		for _, domain := range []string{"remoteapp-invite-request-v1", "remoteapp-network-api-v1\nPOST /v1/network", "remoteapp-provision-intent-v1", "remoteapp-node-possession-v1", "remoteapp-session-product-tls-target-v1"} {
			if _, e = s.Sign([]byte(domain + "\n{}")); e != nil {
				t.Fatal("native domain signing")
			}
		}
		s.Close()
	}
	t.Log("CHECK pinned OS identity reopen and network/provision/node/TLS domains PASS")
	for _, cfg := range []struct {
		scope, public string
		create        bool
	}{
		{ready.Scope, "", true}, {strings.Repeat("0", 32), ready.Public, false}, {ready.Scope, strings.Repeat("0", 64), false},
	} {
		if s, e := privatepipe.NewSigner(relay, cfg.scope, cfg.public, cfg.create); e == nil {
			s.Close()
			t.Fatal("unpinned access accepted")
		}
	}
	s, e := privatepipe.NewSigner(relay, ready.Scope, ready.Public, false)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Sign([]byte("arbitrary")); e == nil {
		t.Fatal("arbitrary signing accepted")
	}
	s.Close()
	// Malicious request must not poison the owner's native process.
	live, e := privatepipe.NewSigner(relay, ready.Scope, ready.Public, false)
	if e != nil {
		t.Fatal("denial poisoned owner")
	}
	if _, e = live.Sign([]byte("remoteapp-node-possession-v1\n{}")); e != nil {
		t.Fatal(e)
	}
	t.Log("CHECK create/wrong scope/wrong public/arbitrary message denied; owner survives PASS")
	json.NewEncoder(in).Encode(map[string]string{"Op": "stop"})
	in.Close()
	var closed struct{ Status string }
	if privatepipe.NewReader(out).Read(&closed) != nil || closed.Status != "closed" {
		t.Fatal("broker cleanup")
	}
	select {
	case e := <-exited:
		if e != nil {
			t.Fatal("nonzero broker exit")
		}
		stopped = true
	case <-time.After(10 * time.Second):
		t.Fatal("broker join")
	}
	if _, e = live.Sign([]byte("remoteapp-node-possession-v1\n{}")); e == nil {
		t.Fatal("post-close signing")
	}
	live.Close()
	if reopened, e := privatepipe.NewSigner(filepath.Join(folder, "identity-vault.exe"), ready.Scope, ready.Public, false); e == nil {
		reopened.Close()
		t.Fatal("test credential not cleaned")
	}
	t.Log("CHECK stop closes live relay, joins workers and deletes only this test identity PASS")
}

// Run this probe over SSH only while start-interactive-vault.ps1 is held open
// in the desktop session. It deliberately cannot create a replacement identity.
func TestPinnedInteractiveVaultFromBootstrap(t *testing.T) {
	folder := os.Getenv("REMOTEAPP_INTERACTIVE_TEST_FOLDER")
	if folder == "" {
		t.Skip("interactive bootstrap folder required")
	}
	f, e := os.Open(filepath.Join(folder, "interactive-bootstrap.json"))
	if e != nil {
		t.Fatal("desktop bootstrap missing")
	}
	defer f.Close()
	var ready struct {
		Status, Pipe, Scope string
		Public              string `json:"public_key"`
		Relay               string `json:"relay_executable"`
	}
	if privatepipe.NewReader(f).Read(&ready) != nil || ready.Status != "ready" || ready.Relay != filepath.Join(folder, "vault-relay.exe") {
		t.Fatal("invalid trusted local bootstrap")
	}
	t.Setenv("REMOTEAPP_INTERACTIVE_VAULT_PIPE", ready.Pipe)
	for i := 0; i < 2; i++ {
		s, e := privatepipe.NewSigner(ready.Relay, ready.Scope, ready.Public, false)
		if e != nil {
			t.Fatal("pinned interactive identity unavailable; no fallback", e)
		}
		for _, domain := range []string{"remoteapp-network-api-v1\nPOST /v1/network", "remoteapp-provision-intent-v1", "remoteapp-node-possession-v1", "remoteapp-session-product-tls-target-v1"} {
			if _, e = s.Sign([]byte(domain + "\n{}")); e != nil {
				s.Close()
				t.Fatal("native signature failed")
			}
		}
		s.Close()
	}
	t.Log("CHECK same-user caller reopens desktop OS identity and verifies HTTPS/enrollment/node/TLS signatures PASS")
}
