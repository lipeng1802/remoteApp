package main

import (
	"bufio"
	"bytes"
	"context"
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
	Fault              bool
	Compiled           bool // isolated compiled-pair coordination, never a consent shortcut
}

func (p consentPins) timeout() time.Duration {
	if p.Compiled {
		return 15 * time.Minute
	}
	return 5 * time.Minute
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

// Node removal is not the final durable retirement confirmation. Leave the
// live supervisor time to finish its journal commit before cancelling it.
func awaitClientCleanup(ctx context.Context, read func() ([]byte, error)) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		b, e := read()
		if e != nil {
			return denied
		}
		if clientJournalsCleaned(b) {
			return nil
		}
		select {
		case <-ctx.Done():
			return denied
		case <-ticker.C:
		}
	}
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
	return consentSessionOps(input, output, pins, issuer, ca, listen, timeout, nil, verifierKey...)
}

type faultActions struct{ freeze, recover func() error }

func consentSessionOps(input io.Reader, output io.Writer, pins consentPins, issuer string, ca []byte, listen string, timeout time.Duration, actions *faultActions, verifierKey ...string) error {
	if pins.Compiled && (!pins.Nodes || (!pins.Fault && actions != nil)) {
		return denied
	}
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
	if pins.Fault {
		if !pins.Nodes || actions == nil || actions.freeze == nil || actions.recover == nil {
			return denied
		}
		ready["fault_phase"] = true
	}
	if pins.Compiled {
		ready["compiled_phase"] = true
	}
	if json.NewEncoder(output).Encode(ready) != nil {
		return denied
	}
	lines := make(chan []byte, 1)
	finished := make(chan struct{})
	defer close(finished)
	go func() {
		s := bufio.NewScanner(input)
		s.Buffer(make([]byte, 128), 128)
		for s.Scan() {
			select {
			case lines <- append([]byte(nil), s.Bytes()...):
			case <-finished:
				return
			}
		}
		close(lines)
	}()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	stage := 0
	for {
		select {
		case line, ok := <-lines:
			if !ok {
				return denied
			}
			if pins.Fault {
				if fixedOperatorCommand(line, "freeze") && stage == 0 {
					if actions.freeze() != nil {
						return denied
					}
					stage = 1
					if json.NewEncoder(output).Encode(map[string]any{"status": "frozen", "old_policy_retained": true, "https_503": true}) != nil {
						return denied
					}
					continue
				}
				if fixedOperatorCommand(line, "recover") && stage == 1 {
					if actions.recover() != nil {
						return denied
					}
					stage = 2
					if json.NewEncoder(output).Encode(map[string]any{"status": "recovered", "explicit_restart": true, "same_verifier": true}) != nil {
						return denied
					}
					continue
				}
				if stage != 2 {
					return denied
				}
			}
			// Exact fixed command excludes aliases, duplicate fields and any grant
			// or private-key input. A closed pipe is failure, not completed consent.
			if !fixedOperatorCommand(line, "stop") {
				return denied
			}
			return nil
		case <-timer.C:
			return denied
		}
	}
}

func fixedOperatorCommand(raw []byte, op string) bool {
	return bytes.Equal(raw, []byte(`{"Op": "`+op+`"}`)) || bytes.Equal(raw, []byte(`{"Op":"`+op+`"}`))
}
