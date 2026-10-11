package main

import (
	"encoding/json"
	"testing"
)

func TestUpgradeReceiptsAreExactAndOperationBound(t *testing.T) {
	if !terminalReplyAllowed("upgrade", "upgrade_rejected", json.RawMessage(`{"status":"upgrade_rejected"}`)) {
		t.Fatal("candidate rejection rejected")
	}
	if !terminalReplyAllowed("installation_state", "installation_state", json.RawMessage(`{"status":"installation_state","epoch":2}`)) {
		t.Fatal("epoch rejected")
	}
	for _, raw := range []string{`{"status":"installation_state","epoch":0}`, `{"status":"installation_state","epoch":true}`, `{"status":"installation_state","epoch":2,"secret":"not-allowed"}`} {
		if terminalReplyAllowed("installation_state", "installation_state", json.RawMessage(raw)) {
			t.Fatal("invalid epoch receipt")
		}
	}
	if terminalReplyAllowed("start", "upgrade_rejected", json.RawMessage(`{"status":"upgrade_rejected"}`)) || terminalReplyAllowed("count", "installation_state", json.RawMessage(`{"status":"installation_state","epoch":2}`)) {
		t.Fatal("wrong operation")
	}
}

func TestReplacementReplyKeepsRelayAvailable(t *testing.T) {
	ack := json.RawMessage(`{"status":"owner_reopened","started":false}`)
	if !terminalReplyAllowed("reopen_fault", "owner_reopened", ack) {
		t.Fatal("exact idle replacement ACK rejected")
	}
	if !terminalReplyAllowed("session_state", "session_state", json.RawMessage(`{"status":"session_state"}`)) ||
		!terminalReplyAllowed("start", "ready", json.RawMessage(`{"status":"ready"}`)) {
		t.Fatal("subsequent state/start replies rejected")
	}
	for _, op := range []string{"start", "session_state", "probe", "stop_session"} {
		if terminalReplyAllowed(op, "owner_reopened", ack) {
			t.Fatal("replacement ACK accepted for unrelated operation")
		}
	}
}

func TestReplacementReplyRejectsNonIdleOrMalformedACK(t *testing.T) {
	for _, raw := range []string{
		`{"status":"owner_reopened","started":true}`,
		`{"status":"owner_reopened","started":0}`,
		`{"status":"owner_reopened","started":null}`,
		`{"status":"owner_reopened"}`,
		`{"status":"owner_reopened","started":false,"extra":true}`,
		`{"status":"ready","started":false}`,
		`{"status":"failed"}`,
	} {
		var value struct{ Status string }
		json.Unmarshal([]byte(raw), &value)
		if terminalReplyAllowed("reopen_fault", value.Status, json.RawMessage(raw)) {
			t.Fatal("non-exact replacement ACK accepted")
		}
	}
}
