// Same-user coordination relay. All frames stay on anonymous/local pipes;
// stdout may contain a TEST product HMAC and must never become a public log.
package main

import (
	"context"
	"encoding/json"
	"os"
	"remoteapp.local/selfhost-poc/desktopcontrol"
	"remoteapp.local/selfhost-poc/privatepipe"
	"time"
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
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	c, e := desktopcontrol.Dial(ctx, os.Getenv("REMOTEAPP_DESKTOP_SESSION_PIPE"))
	if e != nil {
		return e
	}
	defer c.Close()
	input := privatepipe.NewReader(os.Stdin)
	read := privatepipe.NewReader(c)
	write := json.NewEncoder(c)
	output := json.NewEncoder(os.Stdout)
	var hello struct{ Scope, Expected string }
	if input.Read(&hello) != nil || write.Encode(hello) != nil {
		return privatepipe.ErrProtocol
	}
	var ready struct {
		Status string `json:"status"`
		Public string `json:"public_key"`
	}
	c.SetReadDeadline(time.Now().Add(3 * time.Second))
	if read.Read(&ready) != nil || ready.Status != "ready" || ready.Public != hello.Expected || output.Encode(ready) != nil {
		return privatepipe.ErrProtocol
	}
	for {
		var req desktopcontrol.Request
		if input.Read(&req) != nil {
			return nil
		}
		if write.Encode(req) != nil {
			return privatepipe.ErrProtocol
		}
		for {
			var reply json.RawMessage
			c.SetReadDeadline(time.Now().Add(50 * time.Second))
			if read.Read(&reply) != nil {
				return privatepipe.ErrProtocol
			}
			var status struct {
				Status string `json:"status"`
			}
			if json.Unmarshal(reply, &status) != nil || output.Encode(reply) != nil {
				return privatepipe.ErrProtocol
			}
			if status.Status == "product_ready" && req.Op == "start" {
				continue
			}
			if !terminalReplyAllowed(req.Op, status.Status, reply) {
				return privatepipe.ErrProtocol
			}
			break
		}
	}
}

// A successful idle replacement is not session readiness. Forward its exact
// ACK without terminating the relay needed for the subsequent state/start.
func terminalReplyAllowed(op, status string, raw json.RawMessage) bool {
	if op == "reopen_fault" {
		var reply map[string]json.RawMessage
		if json.Unmarshal(raw, &reply) != nil || len(reply) != 2 {
			return false
		}
		return status == "owner_reopened" && string(reply["status"]) == `"owner_reopened"` && string(reply["started"]) == "false"
	}
	switch status {
	case "ready", "probe_passed", "session_closed", "upgraded", "product_count", "session_state", "fault_retired":
		return true
	default:
		return false
	}
}
