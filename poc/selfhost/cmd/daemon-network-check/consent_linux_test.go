package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

func TestCompiledPhaseExplicitMarkerAndBoundedWindow(t *testing.T) {
	p := consentPins{Target: strings.Repeat("a", 64), Controller: strings.Repeat("b", 64), Nodes: true, Compiled: true}
	if p.timeout() != 15*time.Minute || (consentPins{}).timeout() != 5*time.Minute {
		t.Fatal("coordination window changed")
	}
	var out bytes.Buffer
	if e := consentSession(strings.NewReader("{\"Op\":\"stop\"}\n"), &out, p, "issuer", []byte("ca"), "127.0.0.1:12345", time.Second, "nodekey:"+strings.Repeat("c", 64)); e != nil {
		t.Fatal(e)
	}
	var ready map[string]any
	if json.Unmarshal(out.Bytes(), &ready) != nil || len(ready) != 10 || ready["compiled_phase"] != true || ready["fault_phase"] != nil {
		t.Fatal("compiled public marker")
	}
	for _, bad := range []consentPins{{Compiled: true}, {Nodes: true, Compiled: true, Fault: true}} {
		out.Reset()
		if consentSession(strings.NewReader("{\"Op\":\"stop\"}\n"), &out, bad, "issuer", []byte("ca"), "127.0.0.1:12345", time.Second, "nodekey:"+strings.Repeat("c", 64)) == nil || out.Len() != 0 {
			t.Fatal("mixed phase accepted")
		}
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

func TestCleanupBarrierWaitsForCommitAndRejectsTimeoutOrReadError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	calls := 0
	if awaitClientCleanup(ctx, func() ([]byte, error) {
		calls++
		if calls == 1 {
			return []byte(`{"Version":1,"Provisions":{"a":{"Status":"cleanup_pending"}}}`), nil
		}
		return []byte(`{"Version":1,"Provisions":{"a":{"Status":"cleaned"}}}`), nil
	}) != nil || calls != 2 {
		t.Fatal("did not wait for durable cleanup")
	}
	closed, c := context.WithCancel(context.Background())
	c()
	if awaitClientCleanup(closed, func() ([]byte, error) { return []byte(`{"Version":1,"Provisions":{"a":{"Status":"bound"}}}`), nil }) == nil {
		t.Fatal("unconfirmed cleanup accepted")
	}
	if awaitClientCleanup(ctx, func() ([]byte, error) { return nil, errors.New("read failed") }) == nil {
		t.Fatal("read failure accepted")
	}
}
