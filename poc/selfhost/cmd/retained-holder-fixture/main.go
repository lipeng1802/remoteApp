// Windows-only memory transport fixture. No native credentials, SDK, authority
// or real installation. Never included in product/installer build inputs.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"remoteapp.local/selfhost-poc/clientruntime"
	"remoteapp.local/selfhost-poc/desktopcontrol"
	"remoteapp.local/selfhost-poc/installeddevice"
	"remoteapp.local/selfhost-poc/privatepipe"
	"remoteapp.local/selfhost-poc/supervisedcontrol"
	"runtime"
	"strings"
)

type broker struct{}

func (broker) Forward(context.Context, installeddevice.Request) (json.RawMessage, error) {
	return nil, errors.New("fixture denied")
}
func (broker) Count(context.Context) (clientruntime.ProductCount, error) {
	return clientruntime.ProductCount{}, errors.New("fixture idle")
}
func emit(v any) {
	if json.NewEncoder(os.Stdout).Encode(v) != nil {
		os.Exit(7)
	}
}
func main() {
	if runtime.GOOS != "windows" || len(os.Args) != 1 {
		os.Exit(2)
	}
	exe, e := os.Executable()
	if e != nil {
		os.Exit(2)
	}
	if strings.EqualFold(filepath.Base(exe), "test-setup.exe") {
		emit(map[string]string{"status": "test_installed"})
		return
	}
	if !strings.EqualFold(filepath.Base(exe), "supervisor.exe") {
		os.Exit(2)
	}
	os.Exit(run(filepath.Dir(exe)))
}
func run(dir string) int {
	var nonce [16]byte
	rand.Read(nonce[:])
	name := `\\.\pipe\remoteapp-session-` + hex.EncodeToString(nonce[:])
	listener, e := desktopcontrol.Listen(name)
	if e != nil {
		return 3
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); supervisedcontrol.Serve(ctx, listener, broker{}) }()
	defer func() { cancel(); listener.Close(); <-done }()
	reader := privatepipe.NewReader(os.Stdin)
	for {
		var r struct{ Op string }
		if reader.Read(&r) != nil {
			return 3
		}
		f, e := os.OpenFile(filepath.Join(dir, "fixture-ops.txt"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if e != nil {
			return 3
		}
		f.WriteString(r.Op + "\n")
		f.Close()
		switch r.Op {
		case "open":
			emit(map[string]any{"status": "ready", "control_pipe": name, "vault_scope": strings.Repeat("a", 32), "public_key": strings.Repeat("b", 64)})
		case "stop_fault":
			if _, e := os.Stat(filepath.Join(dir, "fixture-authorize.request")); e != nil {
				emit(map[string]string{"status": "fault_stop_rejected"})
				continue
			}
			emit(map[string]any{"status": "fault_stop_ready", "joined": true, "fault": true})
			rest, e := io.ReadAll(os.Stdin)
			if e != nil || len(rest) != 0 {
				return 3
			}
			cancel()
			listener.Close()
			<-done
			emit(map[string]any{"status": "fault_closed", "joined": true, "fault": true})
			return 1
		case "stop":
			rest, e := io.ReadAll(os.Stdin)
			if e != nil || len(rest) != 0 {
				return 3
			}
			cancel()
			listener.Close()
			<-done
			emit(map[string]string{"status": "closed"})
			return 0
		default:
			return 3
		}
	}
}
