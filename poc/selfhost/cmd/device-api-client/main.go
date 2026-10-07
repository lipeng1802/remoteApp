// Private device client fixture: no keys in argv or logs. HTTP requests are
// signed here, not by a privileged parent. Bootstrap trust is still private.
package main

import (
	"bufio"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"remoteapp.local/selfhost-poc/invite"
	"time"
)

func main() {
	if run() != nil {
		json.NewEncoder(os.Stdout).Encode(map[string]any{"status": "failed"})
		os.Exit(1)
	}
}
func run() error {
	reader := bufio.NewReaderSize(os.Stdin, 8194)
	read := func(v any) error {
		line, e := reader.ReadSlice('\n')
		if e != nil {
			return e
		}
		return json.Unmarshal(line, v)
	}
	var cfg struct {
		Endpoint, Certificate, AppKey string
		Grant                         invite.Grant
	}
	if read(&cfg) != nil {
		return invite.ErrDenied
	}
	key, e := hex.DecodeString(cfg.AppKey)
	if e != nil {
		return invite.ErrDenied
	}
	cert, e := base64.StdEncoding.DecodeString(cfg.Certificate)
	if e != nil {
		return invite.ErrDenied
	}
	client, e := invite.NewNetworkClient(cfg.Endpoint, cert, ed25519.PrivateKey(key))
	if e != nil {
		return e
	}
	defer client.Close()
	out := json.NewEncoder(os.Stdout)
	out.Encode(map[string]any{"status": "ready"})
	for {
		var request struct {
			Op, Ticket, Challenge string
			Intent                invite.ProvisionEnvelope
			Device                invite.Body
		}
		if e := read(&request); e == io.EOF {
			return nil
		} else if e != nil {
			return e
		}
		if request.Op == "stop" {
			out.Encode(map[string]any{"status": "closed"})
			return nil
		}
		if request.Op == "device" {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			result, e := client.DeviceRequest(ctx, request.Device)
			cancel()
			value := map[string]any{"status": "device", "passed": e == nil}
			if e == nil {
				value["device"] = result
			}
			if out.Encode(value) != nil {
				return io.ErrClosedPipe
			}
			continue
		}
		action := request.Op
		if action == "network_state" {
			action = "state"
		}
		if action == "clock_proof" {
			action = "clock"
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		reply, e := client.Request(ctx, invite.NetworkRequestBody{Action: action, Grant: cfg.Grant, Ticket: request.Ticket, Challenge: request.Challenge, Intent: request.Intent})
		cancel()
		value := map[string]any{"status": request.Op, "passed": e == nil}
		if e == nil {
			if reply.Ticket != "" {
				value["ticket"] = reply.Ticket
				value["credential"] = reply.Credential
			}
			if reply.State != nil {
				value["network_state"] = reply.State
			}
			if reply.Proof != nil {
				value["proof"] = reply.Proof
			}
		}
		if out.Encode(value) != nil {
			return io.ErrClosedPipe
		}
	}
}
