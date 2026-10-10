package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"
)

// Diagnostic fixture reproduces the existing inherited-stdin shutdown
// ordering without a journal, signer, network, grant or product process.
func TestInheritedInputCloseFixture(t *testing.T) {
	if os.Getenv("REMOTEAPP_STDIN_CLOSE_DIAGNOSTIC") != "1" {
		return
	}
	done, entered := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		close(entered)
		io.Copy(io.Discard, os.Stdin)
	}()
	<-entered
	time.Sleep(50 * time.Millisecond)
	fmt.Println(`{"status":"session_state","joined":true,"fault":true}`)
	fmt.Println(`{"status":"fault_retired","joined":true,"fault":true}`)
	// Identical close-and-join ordering to installed-session.run's defer.
	os.Stdin.Close()
	<-done
	fmt.Println(`{"status":"failed"}`)
	os.Exit(1)
}

func TestInheritedInputCloseDiagnostic(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("Unix inherited descriptor diagnostic")
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe, "-test.run=^TestInheritedInputCloseFixture$")
	cmd.Env = append(os.Environ(), "REMOTEAPP_STDIN_CLOSE_DIAGNOSTIC=1")
	in, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	// Parent EOF is recovery, never a force kill or fabricated process join.
	t.Cleanup(func() { in.Close(); cmd.Wait() })
	lines := make(chan string, 3)
	go func() {
		s := bufio.NewScanner(out)
		for s.Scan() {
			lines <- s.Text()
		}
		close(lines)
	}()
	read := func() string {
		select {
		case line := <-lines:
			return line
		case <-time.After(3 * time.Second):
			t.Fatal("diagnostic response deadline")
			return ""
		}
	}
	if read() != `{"status":"session_state","joined":true,"fault":true}` || read() != `{"status":"fault_retired","joined":true,"fault":true}` {
		t.Fatal("diagnostic completion frames missing")
	}
	select {
	case <-lines:
		t.Fatal("expected inherited blocking descriptor to retain the reader until parent EOF")
	case <-time.After(300 * time.Millisecond):
		t.Log("DIAGNOSTIC terminal frame blocked while parent input remains open")
	}
	in.Close()
	if read() != `{"status":"failed"}` {
		t.Fatal("terminal frame absent after parent EOF")
	}
	if err = cmd.Wait(); err == nil || cmd.ProcessState.ExitCode() != 1 {
		t.Fatal("actual diagnostic exit must be 1")
	}
	t.Log("DIAGNOSTIC parent EOF releases inherited stdin close and actual child exit")
}

func TestUncompiledEntryFailsClosed(t *testing.T) {
	if embeddedRelease != "" || embeddedSignerDigest != "" {
		t.Fatal("test must use ordinary empty build")
	}
	outputs := 0
	if e := run(func(any) { outputs++ }); e == nil || outputs != 0 {
		t.Fatal("uncompiled client admitted")
	}
}
