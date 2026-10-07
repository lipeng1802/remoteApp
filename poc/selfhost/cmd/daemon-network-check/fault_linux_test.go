package main

import (
	"bytes"
	"errors"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestFaultProtocolRequiresExplicitOrderedRecovery(t *testing.T) {
	pins := consentPins{Target: strings.Repeat("a", 64), Controller: strings.Repeat("b", 64), Nodes: true, Fault: true}
	for _, c := range []struct {
		input string
		ok    bool
		calls string
	}{
		{"freeze recover stop", true, "FR"}, {"stop", false, ""}, {"recover stop", false, ""},
		{"freeze stop", false, "F"}, {"freeze freeze recover stop", false, "F"},
		{"freeze recover recover stop", false, "FR"}, {"approve", false, ""},
	} {
		var input, out bytes.Buffer
		for _, op := range strings.Fields(c.input) {
			input.WriteString(`{"Op":"` + op + `"}` + "\n")
		}
		calls := ""
		actions := &faultActions{freeze: func() error { calls += "F"; return nil }, recover: func() error { calls += "R"; return nil }}
		e := consentSessionOps(&input, &out, pins, "issuer", []byte("ca"), "127.0.0.1:12345", time.Second, actions, "nodekey:"+strings.Repeat("c", 64))
		if (e == nil) != c.ok || calls != c.calls {
			t.Fatal("fault protocol order mismatch")
		}
	}
	var out bytes.Buffer
	actions := &faultActions{freeze: func() error { return errors.New("fault failed") }, recover: func() error { return nil }}
	if consentSessionOps(strings.NewReader("{\"Op\":\"freeze\"}\n"), &out, pins, "issuer", []byte("ca"), "127.0.0.1:12345", time.Second, actions, "nodekey:"+strings.Repeat("c", 64)) == nil || strings.Contains(out.String(), `"status":"frozen"`) {
		t.Fatal("failed injection reported successful")
	}
	if fixedOperatorCommand([]byte(`{"Op":"freeze","Grant":{}}`), "freeze") || fixedOperatorCommand([]byte(`{"Op":"freeze","Op":"stop"}`), "freeze") {
		t.Fatal("extra fields accepted")
	}
}

func TestFaultPolicyMustRetainNonzeroExactRules(t *testing.T) {
	raw := []byte(`{"acls":[{"action":"accept","src":["100.120.0.2"],"dst":["100.120.0.1:47476"]}]}`)
	if !samePolicy(raw, raw) {
		t.Fatal("same rule refused")
	}
	for _, other := range []string{`{"acls":[]}`, `{"acls":null}`, `{}`, `{"acls":[{"action":"accept","src":["*"],"dst":["*:*"]}]}`, `{"acls":[],"extra":true}`} {
		if samePolicy(raw, []byte(other)) {
			t.Fatal("altered rules accepted")
		}
	}
}

func TestKernelStoppedStateConfirmedOnlyForOwnedPausedProcess(t *testing.T) {
	p := exec.Command("sleep", "10")
	if p.Start() != nil {
		t.Fatal("start failed")
	}
	defer func() { p.Process.Signal(syscall.SIGCONT); p.Process.Kill(); p.Wait() }()
	if p.Process.Signal(syscall.SIGSTOP) != nil || waitStopped(p.Process.Pid) != nil {
		t.Fatal("kernel pause not confirmed")
	}
	if p.Process.Signal(syscall.SIGCONT) != nil {
		t.Fatal("resume failed")
	}
}
