// Dedicated inherited-pipe journal helper. No network/shell or existing keys.
package main

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"os"
	"runtime"

	"remoteapp.local/selfhost-poc/nativejournal"
	"remoteapp.local/selfhost-poc/privatepipe"
)

func main() {
	runtime.LockOSThread() // Windows mutex ownership is OS-thread-affine.
	defer runtime.UnlockOSThread()
	if e := run(); e != nil {
		reason := "native_journal_unavailable"
		if e == nativejournal.ErrLogon {
			reason = "interactive_logon_required"
		}
		json.NewEncoder(os.Stdout).Encode(map[string]string{"status": "failed", "reason": reason})
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) != 1 {
		return privatepipe.ErrProtocol
	}
	r := privatepipe.NewBoundedReader(os.Stdin, 16384)
	out := json.NewEncoder(os.Stdout)
	var cfg struct {
		Scope string
		Mode  string
		Test  bool
	}
	if r.Read(&cfg) != nil {
		return privatepipe.ErrProtocol
	}
	if cfg.Mode == "cleanup" && cfg.Test {
		if e := nativejournal.CleanupTest(cfg.Scope); e != nil {
			return e
		}
		return out.Encode(map[string]string{"status": "deleted"})
	}
	if cfg.Mode != "create" && cfg.Mode != "open" {
		return privatepipe.ErrProtocol
	}
	s, e := nativejournal.Open(cfg.Scope, cfg.Mode == "create", cfg.Test)
	if e != nil {
		return e
	}
	defer s.Close()
	if e = out.Encode(map[string]string{"status": "ready"}); e != nil {
		return e
	}
	for {
		var c struct {
			Op       string
			Expected string
			Data     string
			Fault    string
		}
		e := r.Read(&c)
		if e == io.EOF {
			return nil
		}
		if e != nil {
			return e
		}
		switch {
		case c.Op == "stop" && c.Expected == "" && c.Data == "" && c.Fault == "":
			if s.Close() != nil {
				return nativejournal.ErrUnavailable
			}
			return out.Encode(map[string]string{"status": "closed"})
		case c.Op == "load" && c.Expected == "" && c.Data == "" && c.Fault == "":
			b, e := s.Load()
			if e == nativejournal.ErrFresh {
				e = out.Encode(map[string]string{"status": "not_installed"})
			} else if e == nil {
				e = out.Encode(map[string]string{"status": "loaded", "data": base64.StdEncoding.EncodeToString(b)})
			}
			if e != nil {
				return e
			}
		case c.Op == "commit" && c.Data != "" && c.Fault == "":
			b, e := base64.StdEncoding.DecodeString(c.Data)
			if e != nil || base64.StdEncoding.EncodeToString(b) != c.Data {
				return privatepipe.ErrProtocol
			}
			if e := s.Commit(c.Expected, b); e != nil {
				return e
			}
			if out.Encode(map[string]string{"status": "committed"}) != nil {
				return io.ErrClosedPipe
			}
		case c.Op == "fault" && cfg.Test && c.Data == "" && c.Expected == "":
			if s.FaultTest(c.Fault) != nil {
				return nativejournal.ErrUnavailable
			}
			if out.Encode(map[string]string{"status": "faulted"}) != nil {
				return io.ErrClosedPipe
			}
		default:
			return privatepipe.ErrProtocol
		}
	}
}
