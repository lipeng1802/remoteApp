package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestConsentPublicPins(t *testing.T) {
	a, b := strings.Repeat("a", 64), strings.Repeat("b", 64)
	for _, c := range [][2]string{{a, a}, {strings.ToUpper(a), b}, {a[:63], b}, {"private", b}} {
		if _, e := parseConsentPins(c[0], c[1]); e == nil {
			t.Fatal("invalid pins accepted")
		}
	}
	if _, e := parseConsentPins(a, b); e != nil {
		t.Fatal("valid pins refused")
	}
}

func TestConsentProtocolNeverApprovesOrAcceptsPrivateMaterial(t *testing.T) {
	pins := consentPins{Target: strings.Repeat("a", 64), Controller: strings.Repeat("b", 64)}
	for _, c := range []struct {
		line string
		ok   bool
	}{
		{`{"Op":"stop"}`, true}, {`{"Op": "stop"}`, true}, {`{"Op":"approve"}`, false},
		{`{"Op":"stop","Grant":{}}`, false}, {`{"Op":"stop","Op":"approve"}`, false},
		{`{"op":"stop"}`, false}, {strings.Repeat("x", 129), false}, {"", false},
	} {
		var out bytes.Buffer
		e := consentSession(strings.NewReader(c.line+"\n"), &out, pins, strings.Repeat("c", 64), []byte("public-ca"), "127.0.0.1:12345", time.Second)
		if (e == nil) != c.ok {
			t.Fatal("operator protocol mismatch")
		}
		var ready map[string]any
		if json.Unmarshal(out.Bytes(), &ready) != nil || len(ready) != 7 || ready["split_process"] != true || ready["target_key"] != nil || ready["grant"] != nil {
			t.Fatal("bootstrap exposes unexpected material")
		}
	}
}

func TestNodePhaseRequiresPublicVerifierMarker(t *testing.T) {
	pins := consentPins{Target: strings.Repeat("a", 64), Controller: strings.Repeat("b", 64), Nodes: true}
	var out bytes.Buffer
	if consentSession(strings.NewReader(`{"Op":"stop"}`), &out, pins, "issuer", []byte("ca"), "127.0.0.1:12345", time.Second) == nil {
		t.Fatal("missing verifier accepted")
	}
	if consentSession(strings.NewReader("{\"Op\":\"stop\"}\n"), &out, pins, "issuer", []byte("ca"), "127.0.0.1:12345", time.Second, "nodekey:"+strings.Repeat("c", 64)) != nil {
		t.Fatal("public verifier refused")
	}
	var ready map[string]any
	if json.Unmarshal(out.Bytes(), &ready) != nil || len(ready) != 9 || ready["node_phase"] != true || ready["verifier"] == nil {
		t.Fatal("node bootstrap mismatch")
	}
}

func TestClientJournalCleanupRequiresConfirmedStatuses(t *testing.T) {
	for _, c := range []struct {
		raw string
		ok  bool
	}{
		{`{"Version":1}`, true}, {`{"Version":1,"Provisions":{"a":{"Status":"cleaned"}}}`, true},
		{`{"Version":1,"Provisions":{"a":{"Status":"bound"}}}`, false},
		{`{"Version":1,"Provisions":{"a":{"Status":"cleanup_pending"}}}`, false},
		{`{"Version":1,"Provisions":{"a":{}}}`, false}, {`{"Version":2}`, false}, {`bad`, false},
	} {
		if clientJournalsCleaned([]byte(c.raw)) != c.ok {
			t.Fatal("journal cleanup assertion mismatch")
		}
	}
}
