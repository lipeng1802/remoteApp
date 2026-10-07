// Run only in the user's interactive desktop. Ephemeral PoC identity; never
// opens the installed application's credential namespace.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"

	"remoteapp.local/selfhost-poc/privatepipe"
	"remoteapp.local/selfhost-poc/vaultbridge"
)

func main() {
	if run() != nil {
		json.NewEncoder(os.Stdout).Encode(map[string]string{"status": "failed"})
		os.Exit(1)
	}
}
func run() error {
	if len(os.Args) != 1 {
		return privatepipe.ErrProtocol
	}
	read := privatepipe.NewReader(os.Stdin).Read
	var cfg struct{ VaultExecutable string }
	if read(&cfg) != nil {
		return privatepipe.ErrProtocol
	}
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return err
	}
	scope := hex.EncodeToString(raw[:])
	signer, err := privatepipe.NewSigner(cfg.VaultExecutable, scope, "", true)
	if err != nil {
		return err
	}
	defer signer.Close()
	name := `\\.\pipe\remoteapp-vault-` + scope
	ln, err := vaultbridge.Listen(name)
	if err != nil {
		signer.DeleteCreatedIdentity()
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { vaultbridge.Serve(ctx, ln, signer, scope, hex.EncodeToString(signer.PublicKey())); close(done) }()
	defer func() { cancel(); ln.Close(); <-done }()
	out := json.NewEncoder(os.Stdout)
	if out.Encode(map[string]string{"status": "ready", "pipe": name, "scope": scope, "public_key": hex.EncodeToString(signer.PublicKey())}) != nil {
		cancel()
		ln.Close()
		<-done
		signer.DeleteCreatedIdentity()
		return privatepipe.ErrProtocol
	}
	var stop struct{ Op string }
	e := read(&stop)
	cancel()
	ln.Close()
	<-done
	// Always clean this run's random credential, even on control-pipe EOF.
	if signer.DeleteCreatedIdentity() != nil {
		return privatepipe.ErrProtocol
	}
	if e != nil || stop.Op != "stop" {
		return privatepipe.ErrProtocol
	}
	return out.Encode(map[string]string{"status": "closed"})
}
