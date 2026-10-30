package productmaintenance

import (
	"encoding/json"
	"io"
	"os"
	"remoteapp.local/selfhost-poc/privatepipe"
	"strings"
	"testing"
	"time"
)

// Dedicated memory fixture only; never compiled into the actual product.
func TestWindowsFaultHolderFixture(t *testing.T) {
	args := os.Args
	if len(args) < 3 || !strings.HasPrefix(args[len(args)-2], "r81-holder-") {
		return
	}
	mode, log := strings.TrimPrefix(args[len(args)-2], "r81-holder-"), args[len(args)-1]
	emit := func(v any) {
		if json.NewEncoder(os.Stdout).Encode(v) != nil {
			os.Exit(8)
		}
	}
	emit(map[string]string{"status": "ready"})
	reader := privatepipe.NewReader(os.Stdin)
	authorized := mode != "rejected"
	for {
		var r struct{ Op string }
		if reader.Read(&r) != nil {
			os.Exit(8)
		}
		f, e := os.OpenFile(log, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if e != nil {
			os.Exit(8)
		}
		f.WriteString(r.Op + "\n")
		f.Close()
		if r.Op == "fixture_authorize" {
			authorized = true
			emit(map[string]string{"status": "fixture_authorized"})
			continue
		}
		if r.Op != "stop_fault" {
			os.Exit(9)
		}
		if !authorized {
			emit(map[string]string{"status": "fault_stop_rejected"})
			continue
		}
		if mode == "late-ready" {
			time.Sleep(250 * time.Millisecond)
		}
		ready := map[string]any{"status": "fault_stop_ready", "joined": true, "fault": true}
		if mode == "integer" {
			ready["joined"] = 1
		}
		emit(ready)
		rest, e := io.ReadAll(os.Stdin)
		if e != nil || len(rest) != 0 {
			os.Exit(9)
		}
		if mode == "late-final" {
			time.Sleep(250 * time.Millisecond)
		}
		final := map[string]any{"status": "fault_closed", "joined": true, "fault": true}
		if mode == "extra" {
			final["extra"] = true
		}
		if mode == "failed" {
			final = map[string]any{"status": "failed"}
		}
		emit(final)
		if mode == "duplicate" {
			emit(final)
		}
		if mode == "zero" {
			os.Exit(0)
		}
		os.Exit(1)
	}
}
