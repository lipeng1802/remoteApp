package productmaintenance

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"remoteapp.local/selfhost-poc/privatepipe"
	"remoteapp.local/selfhost-poc/processowner"
	"strings"
	"testing"
	"time"
)

func TestArmedRemovalProcessFixture(t *testing.T) {
	if len(os.Args) < 2 || !strings.HasPrefix(os.Args[len(os.Args)-1], "armed-fixture-") {
		return
	}
	mode := strings.TrimPrefix(os.Args[len(os.Args)-1], "armed-fixture-")
	reader := privatepipe.NewReader(os.Stdin)
	out := json.NewEncoder(os.Stdout)
	var r struct{ Op string }
	if reader.Read(&r) != nil || r.Op != "arm" {
		os.Exit(7)
	}
	out.Encode(map[string]string{"status": "removal_armed"})
	if reader.Read(&r) != nil {
		os.Exit(8)
	}
	if r.Op == "cancel" {
		out.Encode(map[string]string{"status": "removal_cancelled"})
	} else if r.Op == "remove" {
		if mode == "slow" {
			time.Sleep(150 * time.Millisecond)
		}
		out.Encode(map[string]any{"status": "files_removed", "epoch": 1, "transaction": strings.Repeat("a", 32), "preserved": true})
		if mode == "extra" {
			out.Encode(map[string]string{"status": "closed"})
		}
	} else {
		os.Exit(9)
	}
	io.Copy(io.Discard, os.Stdin)
	if mode == "nonzero" {
		os.Exit(1)
	}
	os.Exit(0)
}
func TestArmedRemovalSameChildOneShotLateJoinAndCancel(t *testing.T) {
	for _, mode := range []string{"success", "slow", "extra", "nonzero", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			exe, _ := os.Executable()
			exe, _ = filepath.EvalSymlinks(exe)
			raw, _ := os.ReadFile(exe)
			h := sha256.Sum256(raw)
			a, e := StartArmedRemoval(context.Background(), processowner.Spec{Executable: exe, SHA256: hex.EncodeToString(h[:]), Args: []string{"-test.run=^TestArmedRemovalProcessFixture$", "armed-fixture-" + mode}})
			if e != nil {
				t.Fatal(e)
			}
			if a.PID() <= 0 {
				t.Fatal("no original child")
			}
			if mode != "cancel" {
				if a.Begin() != nil || a.Begin() == nil {
					t.Fatal("duplicate removal admitted")
				}
				if mode == "slow" {
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
					e = a.Close(ctx)
					cancel()
					if e == nil {
						t.Fatal("late receipt claimed early")
					}
				}
			}
			e = a.Close(context.Background())
			bad := mode == "extra" || mode == "nonzero"
			if (e != nil) != bad {
				t.Fatal("join result", e)
			}
			if !bad && mode != "cancel" {
				v, ok := a.Status().(RemovalReceipt)
				if !ok || !v.Joined || !v.ZeroExit || !v.Preserved {
					t.Fatal("missing actual receipt")
				}
			}
			if !bad && a.child.Stop(context.Background()) == nil {
				t.Fatal("one-shot receipt borrowed as ordinary closed")
			}
		})
	}
}
