package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net"
	"strconv"
	"time"
)

type consentPins struct {
	Target, Controller string
	Nodes              bool
}

func clientJournalsCleaned(raw []byte) bool {
	var snapshot struct {
		Version    int
		Provisions map[string]struct{ Status string }
	}
	if json.Unmarshal(raw, &snapshot) != nil || snapshot.Version != 1 {
		return false
	}
	for _, record := range snapshot.Provisions {
		if record.Status != "cleaned" {
			return false
		}
	}
	return true
}

func parseConsentPins(target, controller string) (consentPins, error) {
	for _, key := range []string{target, controller} {
		b, e := hex.DecodeString(key)
		if e != nil || len(b) != 32 || hex.EncodeToString(b) != key {
			return consentPins{}, denied
		}
	}
	if target == controller {
		return consentPins{}, denied
	}
	return consentPins{Target: target, Controller: controller}, nil
}

// Private operator protocol: public readiness and stop only. No Store access,
// device signature, consent shortcut or network mutation is accepted here.
func consentSession(input io.Reader, output io.Writer, pins consentPins, issuer string, ca []byte, listen string, timeout time.Duration, verifierKey ...string) error {
	host, port, e := net.SplitHostPort(listen)
	p, pe := strconv.Atoi(port)
	if e != nil || pe != nil || host != "127.0.0.1" || p < 1 || p > 65535 {
		return denied
	}
	ready := map[string]any{"status": "ready", "split_process": true, "target": pins.Target, "controller": pins.Controller,
		"issuer": issuer, "api_certificate": base64.StdEncoding.EncodeToString(ca), "api_port": p}
	if pins.Nodes {
		if len(verifierKey) != 1 || len(verifierKey[0]) != 72 {
			return denied
		}
		ready["node_phase"] = true
		ready["verifier"] = verifierKey[0]
	}
	if json.NewEncoder(output).Encode(ready) != nil {
		return denied
	}
	lines := make(chan []byte, 1)
	go func() {
		s := bufio.NewScanner(input)
		s.Buffer(make([]byte, 128), 128)
		if s.Scan() {
			lines <- append([]byte(nil), s.Bytes()...)
		} else {
			lines <- nil
		}
	}()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case line := <-lines:
		// Exact fixed command excludes aliases, duplicate fields and any grant
		// or private-key input. A closed pipe is failure, not completed consent.
		if !bytes.Equal(line, []byte(`{"Op": "stop"}`)) && !bytes.Equal(line, []byte(`{"Op":"stop"}`)) {
			return denied
		}
		return nil
	case <-timer.C:
		return denied
	}
}
