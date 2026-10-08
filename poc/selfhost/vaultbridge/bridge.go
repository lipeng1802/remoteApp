// Package vaultbridge exposes one explicitly created, pinned test identity to
// trusted same-user helpers. It is not a public signing service.
package vaultbridge

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net"
	"strings"
	"sync"
	"time"

	"remoteapp.local/selfhost-poc/privatepipe"
)

type Identity interface {
	Sign([]byte) ([]byte, error)
}

// Serve owns listener and connections, not identity. Caller deletes its random
// test identity only after Serve has joined all workers.
func Serve(ctx context.Context, ln net.Listener, signer Identity, scope, public string) {
	ctx, cancel := context.WithCancel(ctx)
	var workers sync.WaitGroup
	slots := make(chan struct{}, 4)
	done := make(chan struct{})
	go func() {
		defer close(done)
		<-ctx.Done()
		ln.Close()
	}()
	defer func() { cancel(); ln.Close(); workers.Wait(); <-done }()
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		select {
		case slots <- struct{}{}:
		default:
			conn.Close()
			continue
		}
		workers.Add(1)
		go func() {
			defer workers.Done()
			defer func() { <-slots }()
			defer conn.Close()
			finished := make(chan struct{})
			joined := make(chan struct{})
			go func() {
				defer close(joined)
				select {
				case <-ctx.Done():
					conn.Close()
				case <-finished:
				}
			}()
			defer func() { close(finished); <-joined }()
			handle(conn, signer, scope, public)
		}()
	}
}

func handle(conn net.Conn, signer Identity, scope, public string) {
	read := privatepipe.NewReader(conn).Read
	reply := func(r privatepipe.VaultReply) error {
		conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
		return json.NewEncoder(conn).Encode(r)
	}
	var cfg struct {
		Scope    string
		Create   bool
		Expected string
	}
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	if read(&cfg) != nil || cfg.Create || cfg.Scope != scope || cfg.Expected != public {
		reply(privatepipe.VaultReply{Status: "failed"})
		return
	}
	if reply(privatepipe.VaultReply{Status: "ready", PublicKey: public}) != nil {
		return
	}
	for {
		var req struct{ Op, Message string }
		// Device API signer can be idle while the product is built or while a
		// different helper owns refresh. Owner stop still closes it immediately.
		conn.SetReadDeadline(time.Now().Add(10 * time.Minute))
		if read(&req) != nil {
			return
		}
		if req.Op == "stop" && req.Message == "" {
			reply(privatepipe.VaultReply{Status: "closed"})
			return
		}
		raw, err := base64.StdEncoding.DecodeString(req.Message)
		if req.Op != "sign" || err != nil || len(raw) > 4096 || base64.StdEncoding.EncodeToString(raw) != req.Message || !allowed(raw) {
			reply(privatepipe.VaultReply{Status: "failed"})
			return
		}
		// Domain enforcement remains in the native vault, signature verification in
		// privatepipe.Signer. No export/delete/create operations are forwarded.
		signature, err := signer.Sign(raw)
		if err != nil {
			reply(privatepipe.VaultReply{Status: "failed"})
			return
		}
		if reply(privatepipe.VaultReply{Status: "signed", Signature: hex.EncodeToString(signature)}) != nil {
			return
		}
	}
}

func allowed(raw []byte) bool {
	for _, domain := range []string{"remoteapp-invite-request-v1\n", "remoteapp-network-api-v1\nPOST /v1/network\n", "remoteapp-provision-intent-v1\n", "remoteapp-node-possession-v1\n", "remoteapp-session-target-v1\n", "remoteapp-session-controller-v1\n", "remoteapp-session-product-tls-target-v1\n", "remoteapp-session-product-tls-controller-v1\n"} {
		if strings.HasPrefix(string(raw), domain) {
			return true
		}
	}
	return false
}
