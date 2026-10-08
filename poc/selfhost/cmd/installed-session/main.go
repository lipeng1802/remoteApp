// Independent installed-config TLS bridge. Stdout is a trusted local pipe.
package main

import (
	"context"
	"encoding/json"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"

	"remoteapp.local/selfhost-poc/clientowner"
	"remoteapp.local/selfhost-poc/clientruntime"
	"remoteapp.local/selfhost-poc/privatepipe"
	"remoteapp.local/selfhost-poc/processowner"
	"remoteapp.local/selfhost-poc/session"
	"remoteapp.local/selfhost-poc/trustinstall"
	"remoteapp.local/selfhost-poc/trustprofile"
)

var embeddedRelease, embeddedSignerDigest, embeddedProductDigest string // trusted link step only
type request struct {
	Op                        string
	Config                    clientruntime.Config
	VaultScope, VaultExpected string
	Profile                   trustprofile.Profile
	ProductKey                string // private TEST HMAC only; not final provisioning
}

func main() {
	var mu sync.Mutex
	output := func(v any) { mu.Lock(); defer mu.Unlock(); json.NewEncoder(os.Stdout).Encode(v) }
	if run(output) != nil {
		output(map[string]string{"status": "failed"})
		os.Exit(1)
	}
	output(map[string]string{"status": "closed"})
}
func run(output func(any)) (result error) {
	r, e := trustinstall.DecodeCompiled(embeddedRelease)
	if e != nil {
		return e
	}
	exe, e := os.Executable()
	if e != nil {
		return e
	}
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	dir := filepath.Dir(exe)
	f, e := r.NativeFactory(filepath.Join(dir, "trust-journal"+suffix))
	if e != nil {
		return e
	}
	o, e := clientowner.Open(r, f, time.Now)
	if e != nil {
		return e
	}
	defer func() {
		// The parent's timeout may retain this live helper. Exiting here while
		// a product is unjoined would drop the OS journal lease prematurely.
		if e := o.Close(context.Background()); result == nil {
			result = e
		}
	}()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	// Closing only this dedicated helper's stdin joins the reader even when
	// cancellation/runner failure happens while its parent is idle.
	commands := make(chan request)
	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		defer close(commands)
		reader := privatepipe.NewReader(os.Stdin)
		for {
			var c request
			if reader.Read(&c) != nil {
				return
			}
			select {
			case commands <- c:
			case <-ctx.Done():
				return
			}
		}
	}()
	defer func() { cancel(); os.Stdin.Close(); <-readerDone }()
	var first request
	select {
	case first = <-commands:
	case <-ctx.Done():
		return nil
	}
	if first.Op != "start" || first.Profile != (trustprofile.Profile{}) {
		return session.ErrDenied
	}
	expected := first.Config.Controller
	if first.Config.Server {
		expected = first.Config.Target
	}
	if first.VaultExpected != expected {
		return session.ErrDenied
	}
	open := clientruntime.NativeSigner(filepath.Join(dir, "identity-vault"+suffix), embeddedSignerDigest, first.VaultScope, first.VaultExpected)
	ready := func(port int) { output(map[string]any{"status": "ready", "port": port}) }
	runner := clientruntime.Runner(r, first.Config, open, ready)
	if embeddedProductDigest != "" {
		// This slice's actual products are Windows target / macOS controller.
		if (runtime.GOOS == "windows") != first.Config.Server || (first.Config.Server && first.ProductKey != "") {
			return session.ErrDenied
		}
		product := clientruntime.Product{Spec: processowner.Spec{Executable: filepath.Join(dir, "tls-product"+suffix), SHA256: embeddedProductDigest}, ControllerKey: first.ProductKey, TargetReady: func(key, fp string) {
			output(map[string]string{"status": "product_ready", "Key": key, "Fingerprint": fp})
		}}
		runner = clientruntime.RunnerWithProduct(r, first.Config, open, ready, product)
	} else if first.ProductKey != "" {
		return session.ErrDenied
	}
	h, e := o.Start(ctx, runner)
	if e != nil {
		return e
	}
	select {
	case <-ctx.Done():
		return nil
	case <-h.Done():
		return h.Wait(context.Background())
	case c, ok := <-commands:
		if !ok {
			return nil
		} // EOF cancels and joins via deferred Close
		if c.Config != (clientruntime.Config{}) || c.VaultScope != "" || c.VaultExpected != "" || c.ProductKey != "" {
			return session.ErrDenied
		}
		switch c.Op {
		case "stop":
			if c.Profile != (trustprofile.Profile{}) {
				return session.ErrDenied
			}
			return nil
		case "upgrade":
			limit, stop := context.WithTimeout(context.Background(), 8*time.Second)
			defer stop()
			if e = o.Upgrade(limit, c.Profile); e != nil {
				return e
			}
			output(map[string]any{"status": "upgraded", "epoch": c.Profile.Claims.Epoch})
			return nil // explicit new process required; never reconnect
		default:
			return session.ErrDenied
		}
	}
}
