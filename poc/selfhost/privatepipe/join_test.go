package privatepipe

import (
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestStrictSignerFixture(t *testing.T) {
	args := os.Args
	if len(args) < 5 || args[len(args)-3] != "signer-join-fixture" {
		return
	}
	release, ack := args[len(args)-2], args[len(args)-1]
	var cmd struct{ Op string }
	if NewReader(os.Stdin).Read(&cmd) != nil || cmd.Op != "stop" {
		os.Exit(7)
	}
	json.NewEncoder(os.Stdout).Encode(VaultReply{Status: "closed"})
	if os.WriteFile(ack, []byte("ack"), 0600) != nil {
		os.Exit(7)
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if b, e := os.ReadFile(release); e == nil {
			if string(b) == "fail" {
				os.Exit(7)
			}
			os.Exit(0)
		}
		time.Sleep(10 * time.Millisecond)
	}
	os.Exit(7)
}
func testJoinedSigner(t *testing.T) (*Signer, string, string) {
	t.Helper()
	exe, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	dir := t.TempDir()
	release, ack := filepath.Join(dir, "release"), filepath.Join(dir, "ack")
	c := exec.Command(exe, "-test.run=^TestStrictSignerFixture$", "signer-join-fixture", release, ack)
	in, e := c.StdinPipe()
	if e != nil {
		t.Fatal(e)
	}
	out, e := c.StdoutPipe()
	if e != nil {
		t.Fatal(e)
	}
	c.Stderr = io.Discard
	if e = c.Start(); e != nil {
		t.Fatal(e)
	}
	s := &Signer{cmd: c, in: in, out: out, reader: NewReader(out), wait: make(chan struct{})}
	t.Cleanup(func() { os.WriteFile(release, []byte("ok"), 0600); s.CloseJoined() })
	return s, release, ack
}
func TestSignerCloseJoinedWaitsAfterClosedAck(t *testing.T) {
	s, release, ack := testJoinedSigner(t)
	result := make(chan error, 1)
	go func() { result <- s.CloseJoined() }()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, e := os.Stat(ack); e == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, e := os.Stat(ack); e != nil {
		t.Fatal("no close acknowledgment")
	}
	select {
	case <-result:
		t.Fatal("ack mistaken for process join")
	default:
	}
	if e := os.WriteFile(release, []byte("ok"), 0600); e != nil {
		t.Fatal(e)
	}
	select {
	case e := <-result:
		if e != nil {
			t.Fatal(e)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("strict signer did not join")
	}
	if e := s.CloseJoined(); e != nil {
		t.Fatal("sticky Wait retry failed", e)
	}
}
func TestSignerCloseJoinedRejectsNonzeroExit(t *testing.T) {
	s, release, _ := testJoinedSigner(t)
	if e := os.WriteFile(release, []byte("fail"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := s.CloseJoined(); e == nil {
		t.Fatal("nonzero signer exit accepted")
	}
	if e := s.CloseJoined(); e == nil {
		t.Fatal("failed close forgotten on retry")
	}
}
