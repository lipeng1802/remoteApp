// A privatepipe-compatible executable for same-user SSH helpers. Only the
// local pipe name is inherited; neither private key nor credential is exported.
package main

import (
	"context"
	"encoding/json"
	"os"
	"time"

	"remoteapp.local/selfhost-poc/privatepipe"
	"remoteapp.local/selfhost-poc/vaultbridge"
)

func main() {
	if run() != nil {
		json.NewEncoder(os.Stdout).Encode(privatepipe.VaultReply{Status: "failed"})
		os.Exit(1)
	}
}
func run() error {
	if len(os.Args) != 1 {
		return privatepipe.ErrProtocol
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conn, err := vaultbridge.Dial(ctx, os.Getenv("REMOTEAPP_INTERACTIVE_VAULT_PIPE"))
	if err != nil {
		return err
	}
	defer conn.Close()
	input := privatepipe.NewReader(os.Stdin).Read
	output := json.NewEncoder(os.Stdout)
	read := privatepipe.NewReader(conn).Read
	write := json.NewEncoder(conn)
	var cfg struct {
		Scope    string
		Create   bool
		Expected string
	}
	if input(&cfg) != nil || write.Encode(cfg) != nil {
		return privatepipe.ErrProtocol
	}
	var ready privatepipe.VaultReply
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	if read(&ready) != nil || ready.Status != "ready" {
		return privatepipe.ErrProtocol
	}
	if output.Encode(ready) != nil {
		return privatepipe.ErrProtocol
	}
	for {
		var req struct{ Op, Message string }
		if input(&req) != nil || write.Encode(req) != nil {
			return privatepipe.ErrProtocol
		}
		var reply privatepipe.VaultReply
		conn.SetReadDeadline(time.Now().Add(3 * time.Second))
		if read(&reply) != nil || output.Encode(reply) != nil {
			return privatepipe.ErrProtocol
		}
		if reply.Status == "closed" {
			return nil
		}
		if reply.Status != "signed" {
			return privatepipe.ErrProtocol
		}
	}
}
