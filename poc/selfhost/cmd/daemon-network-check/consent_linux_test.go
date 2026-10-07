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
	pins := consentPins{strings.Repeat("a", 64), strings.Repeat("b", 64)}
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
