// Standalone offline maintenance tool, not a product installer/client.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"remoteapp.local/selfhost-poc/trustinstall"
	"remoteapp.local/selfhost-poc/trustprofile"
)

// Set only by the trusted build step; empty builds fail closed. Runtime JSON,
// environment and argv cannot replace this release root/minimum epoch/helper.
var embeddedRelease string

type reply struct {
	Status string `json:"status"`
	Epoch  uint64 `json:"epoch,omitempty"`
	Reason string `json:"reason,omitempty"`
}

func main() {
	p, e := run()
	if e != nil {
		reason := "trust_rejected"
		if errors.Is(e, trustprofile.ErrDesktopLogon) {
			reason = "interactive_logon_required"
		}
		json.NewEncoder(os.Stdout).Encode(reply{Status: "failed", Reason: reason})
		os.Exit(1)
	}
	json.NewEncoder(os.Stdout).Encode(reply{Status: "usable", Epoch: p.Claims.Epoch})
}

func run() (trustprofile.Profile, error) {
	r, e := trustinstall.DecodeCompiled(embeddedRelease)
	if e != nil {
		return trustprofile.Profile{}, e
	}
	if len(os.Args) < 2 || len(os.Args) > 3 {
		return trustprofile.Profile{}, trustprofile.ErrTrust
	}
	exe, e := os.Executable()
	if e != nil {
		return trustprofile.Profile{}, e
	}
	name := "trust-journal"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	helper := filepath.Join(filepath.Dir(exe), name)
	factory, e := r.NativeFactory(helper)
	if e != nil {
		return trustprofile.Profile{}, e
	}
	owner, e := trustinstall.NewOfflineOwner(r, factory, time.Now)
	if e != nil {
		return trustprofile.Profile{}, e
	}
	switch {
	case os.Args[1] == "initialize" && len(os.Args) == 2:
		return owner.Initialize()
	case os.Args[1] == "inspect" && len(os.Args) == 2:
		return owner.Inspect()
	case os.Args[1] == "upgrade":
		p := r.Specification().Bundled
		if len(os.Args) == 3 {
			f, e := os.Open(os.Args[2])
			if e != nil {
				return trustprofile.Profile{}, e
			}
			b, e := io.ReadAll(io.LimitReader(f, 8193))
			f.Close()
			if e != nil {
				return trustprofile.Profile{}, e
			}
			p, e = trustprofile.Decode(b)
			if e != nil {
				return trustprofile.Profile{}, e
			}
		}
		return owner.Upgrade(context.Background(), p)
	case os.Args[1] == "cleanup-test" && len(os.Args) == 2 && r.Specification().Test:
		return trustprofile.Profile{}, trustprofile.DeleteTestDesktopJournal(helper, r.Scope())
	case os.Args[1] == "fault-test" && len(os.Args) == 3 && r.Specification().Test:
		j, e := factory(false)
		if e != nil {
			return trustprofile.Profile{}, e
		}
		d, ok := j.(*trustprofile.DesktopJournal)
		if !ok {
			j.Close()
			return trustprofile.Profile{}, trustprofile.ErrTrust
		}
		e = d.FaultTest(os.Args[2])
		if closed := j.Close(); e == nil {
			e = closed
		}
		return trustprofile.Profile{}, e
	default:
		return trustprofile.Profile{}, trustprofile.ErrTrust
	}
}
