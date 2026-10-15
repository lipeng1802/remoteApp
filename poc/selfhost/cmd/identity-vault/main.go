// Windows current-user Credential Manager signer. No private-key export.
package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"remoteapp.local/selfhost-poc/devicevault"
	"remoteapp.local/selfhost-poc/privatepipe"
	"strings"
)

func main() {
	if err := run(); err != nil {
		reason := "vault_unavailable"
		if err == devicevault.ErrLogonUnavailable {
			reason = "interactive_logon_required"
		}
		json.NewEncoder(os.Stdout).Encode(map[string]string{"status": "failed", "reason": reason})
		os.Exit(1)
	}
}
func canonical(s string, n int) bool {
	b, e := hex.DecodeString(s)
	return e == nil && len(b) == n && hex.EncodeToString(b) == s
}
func run() error {
	if len(os.Args) != 1 {
		return privatepipe.ErrProtocol
	}
	r := privatepipe.NewReader(os.Stdin)
	out := json.NewEncoder(os.Stdout)
	var cfg struct {
		Scope       string
		Create      bool
		Expected    string
		TestCleanup bool
	}
	if r.Read(&cfg) != nil || !canonical(cfg.Scope, 16) || (cfg.Create && (cfg.Expected != "" || cfg.TestCleanup)) || (!cfg.Create && !canonical(cfg.Expected, 32)) {
		return privatepipe.ErrProtocol
	}
	seed, e := devicevault.Load(cfg.Scope, cfg.Create)
	if e != nil {
		return e
	}
	defer func() {
		for i := range seed {
			seed[i] = 0
		}
	}()
	key := ed25519.NewKeyFromSeed(seed)
	defer func() {
		for i := range key {
			key[i] = 0
		}
	}()
	pub := hex.EncodeToString(key.Public().(ed25519.PublicKey))
	if cfg.Expected != "" && cfg.Expected != pub {
		return privatepipe.ErrProtocol
	}
	out.Encode(map[string]string{"status": "ready", "public_key": pub})
	for {
		var c struct {
			Op      string
			Message string
		}
		e := r.Read(&c)
		if e == io.EOF {
			return nil
		}
		if e != nil {
			return e
		}
		if c.Op == "stop" && c.Message == "" {
			return out.Encode(map[string]string{"status": "closed"})
		}
		if c.Op == "delete" && (cfg.Create || cfg.TestCleanup) && c.Message == "" {
			if devicevault.Delete(cfg.Scope) != nil {
				return privatepipe.ErrProtocol
			}
			return out.Encode(map[string]string{"status": "deleted"})
		}
		raw, e := base64.StdEncoding.DecodeString(c.Message)
		if c.Op != "sign" || e != nil || len(raw) > 4096 || !allowedDomain(string(raw)) {
			return privatepipe.ErrProtocol
		}
		if out.Encode(map[string]string{"status": "signed", "signature": hex.EncodeToString(ed25519.Sign(key, raw))}) != nil {
			return io.ErrClosedPipe
		}
	}
}

func allowedDomain(raw string) bool {
	for _, domain := range []string{"remoteapp-invite-request-v1\n", "remoteapp-network-api-v1\nPOST /v1/network\n", "remoteapp-provision-intent-v1\n", "remoteapp-node-possession-v1\n", "remoteapp-session-target-v1\n", "remoteapp-session-controller-v1\n", "remoteapp-session-product-tls-target-v1\n", "remoteapp-session-product-tls-controller-v1\n"} {
		if strings.HasPrefix(raw, domain) {
			return true
		}
	}
	return false
}
