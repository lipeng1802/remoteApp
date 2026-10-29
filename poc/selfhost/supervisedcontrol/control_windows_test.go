package supervisedcontrol

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net"
	"os"
	"os/exec"
	"remoteapp.local/selfhost-poc/desktopcontrol"
	"strings"
	"testing"
)

func TestWindowsNativeBusinessPipe(t *testing.T) {
	var nonce [16]byte
	if _, e := rand.Read(nonce[:]); e != nil {
		t.Fatal(e)
	}
	name := `\\.\pipe\remoteapp-session-` + hex.EncodeToString(nonce[:])
	ln, e := desktopcontrol.Listen(name)
	if e != nil {
		t.Fatal(e)
	}
	// The second listener must not attach to an existing desktop instance.
	if other, e := desktopcontrol.Listen(name); e == nil {
		other.Close()
		ln.Close()
		t.Fatal("pipe collision accepted")
	}
	exercise(t, ln, func() (net.Conn, error) { return desktopcontrol.Dial(context.Background(), name) })
}

func TestWindowsCompiledRelay(t *testing.T) {
	relay := os.Getenv("REMOTEAPP_CONTROL_TEST_RELAY")
	if relay == "" {
		t.Skip("compiled TEST relay required")
	}
	var nonce [16]byte
	if _, e := rand.Read(nonce[:]); e != nil {
		t.Fatal(e)
	}
	name := `\\.\pipe\remoteapp-session-` + hex.EncodeToString(nonce[:])
	ln, e := desktopcontrol.Listen(name)
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	b := &fixture{}
	go func() { defer close(done); Serve(ctx, ln, b) }()
	defer func() { cancel(); ln.Close(); <-done }()
	cmd := exec.Command(relay, name)
	cmd.Stdin = strings.NewReader(`{"Op":"product_count"}` + "\n")
	raw, e := cmd.Output()
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(string(raw), `"product_count"`) || b.calls.Load() != 1 {
		t.Fatal("relay did not reach count backend")
	}
	cmd = exec.Command(relay, name)
	cmd.Stdin = strings.NewReader(`{"Op":"stop"}` + "\n")
	if cmd.Run() == nil || b.calls.Load() != 1 {
		t.Fatal("relay accepted lifecycle command")
	}
}
