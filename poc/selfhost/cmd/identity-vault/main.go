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
		Scope    string
		Create   bool
		Expected string
	}
	if r.Read(&cfg) != nil || !canonical(cfg.Scope, 16) || (cfg.Create && cfg.Expected != "") || (!cfg.Create && !canonical(cfg.Expected, 32)) {
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
		if c.Op == "delete" && cfg.Create && c.Message == "" {
			if devicevault.Delete(cfg.Scope) != nil {
				return privatepipe.ErrProtocol
			}
			return out.Encode(map[string]string{"status": "deleted"})
		}
		raw, e := base64.StdEncoding.DecodeString(c.Message)
		if c.Op != "sign" || e != nil || len(raw) > 4096 || (!strings.HasPrefix(string(raw), "remoteapp-invite-request-v1\n") && !strings.HasPrefix(string(raw), "remoteapp-network-api-v1\nPOST /v1/network\n")) {
			return privatepipe.ErrProtocol
		}
		if out.Encode(map[string]string{"status": "signed", "signature": hex.EncodeToString(ed25519.Sign(key, raw))}) != nil {
			return io.ErrClosedPipe
		}
	}
}
