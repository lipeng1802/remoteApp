// Test-only Windows desktop owner. Roots and sibling binary digests are
// compiled by trusted build; stdin borrows ONLY an already created test key.
package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"syscall"
	"time"

	"remoteapp.local/selfhost-poc/clientowner"
	"remoteapp.local/selfhost-poc/clientruntime"
	"remoteapp.local/selfhost-poc/desktopcontrol"
	"remoteapp.local/selfhost-poc/privatepipe"
	"remoteapp.local/selfhost-poc/processowner"
	"remoteapp.local/selfhost-poc/trustinstall"
	"remoteapp.local/selfhost-poc/trustprofile"
)

var embeddedRelease, embeddedSignerDigest, embeddedProductDigest string

type backend struct {
	owner    *clientowner.Owner
	release  trustinstall.Release
	factory  trustinstall.Factory
	signer   clientruntime.Signer
	open     clientruntime.SignerFactory
	product  processowner.Spec
	expected string
}

func (b *backend) Probe(ctx context.Context) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	var nonce [32]byte
	if _, e := rand.Read(nonce[:]); e != nil {
		return e
	}
	message := []byte("remoteapp-invite-request-v1\n" + hex.EncodeToString(nonce[:]))
	sig, e := b.signer.Sign(message)
	if e != nil || !ed25519.Verify(b.signer.PublicKey(), message, sig) {
		return privatepipe.ErrProtocol
	}
	// Prove this desktop owner retains the exclusive OS journal lease.
	if j, e := b.factory(false); e == nil {
		j.Close()
		return trustprofile.ErrTrust
	}
	return nil
}
func (b *backend) Start(ctx context.Context, c clientruntime.Config, out func(any) error) error {
	if !c.Server || c.Target != b.expected {
		return privatepipe.ErrProtocol
	}
	ready := make(chan error, 2)
	report := func(e error) {
		select {
		case ready <- e:
		default:
		}
	}
	product := clientruntime.Product{Spec: b.product, TargetReady: func(key, fp string) {
		if e := out(map[string]string{"status": "product_ready", "Key": key, "Fingerprint": fp}); e != nil {
			report(e)
		}
	}}
	runner := clientruntime.RunnerWithProduct(b.release, c, b.open, func(port int) { report(out(map[string]any{"status": "ready", "port": port})) }, product)
	h, e := b.owner.Start(ctx, runner)
	if e != nil {
		return e
	}
	timer := time.NewTimer(35 * time.Second)
	defer timer.Stop()
	select {
	case e = <-ready:
		if e == nil {
			return nil
		}
	case <-h.Done():
		e = h.Wait(context.Background())
		if e == nil {
			e = privatepipe.ErrProtocol
		}
	case <-ctx.Done():
		e = ctx.Err()
	case <-timer.C:
		e = context.DeadlineExceeded
	}
	// A failed startup never reports a joined session before actual completion.
	if stopped := b.owner.Stop(context.Background()); stopped != nil {
		return stopped
	}
	return e
}
func (b *backend) Stop(ctx context.Context) error { return b.owner.Stop(ctx) }
func (b *backend) Upgrade(ctx context.Context, p trustprofile.Profile) error {
	return b.owner.Upgrade(ctx, p)
}
func main() {
	e := run()
	status := "closed"
	if e != nil {
		status = "failed"
	}
	json.NewEncoder(os.Stdout).Encode(map[string]string{"status": status})
	if e != nil {
		os.Exit(1)
	}
}
func run() (result error) {
	if runtime.GOOS != "windows" || len(os.Args) != 1 {
		return privatepipe.ErrProtocol
	}
	r, e := trustinstall.DecodeCompiled(embeddedRelease)
	if e != nil || !r.Specification().Test || embeddedProductDigest == "" {
		return trustprofile.ErrTrust
	}
	var first struct{ Op, VaultScope, VaultExpected string }
	reader := privatepipe.NewReader(os.Stdin)
	if reader.Read(&first) != nil || first.Op != "initialize" || !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(first.VaultScope) || !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(first.VaultExpected) {
		return privatepipe.ErrProtocol
	}
	exe, e := os.Executable()
	if e != nil {
		return e
	}
	dir := filepath.Dir(exe)
	helper := filepath.Join(dir, "trust-journal.exe")
	factory, e := r.NativeFactory(helper)
	if e != nil {
		return e
	}
	open := clientruntime.NativeSigner(filepath.Join(dir, "identity-vault.exe"), embeddedSignerDigest, first.VaultScope, first.VaultExpected)
	signer, e := open()
	if e != nil {
		return e
	}
	initialized := false
	defer func() {
		joined, ok := signer.(interface{ CloseJoined() error })
		if !ok {
			result = privatepipe.ErrProtocol
			return
		}
		if e := joined.CloseJoined(); e != nil {
			result = e
		}
		if result == nil && initialized {
			result = trustprofile.DeleteTestDesktopJournal(helper, r.Scope())
		}
	}()
	offline, e := trustinstall.NewOfflineOwner(r, factory, time.Now)
	if e != nil {
		return e
	}
	if _, e = offline.Initialize(); e != nil {
		return e
	} // preserve partial/unowned state on failure
	initialized = true
	owner, e := clientowner.Open(r, factory, time.Now)
	if e != nil {
		return e
	}
	defer func() {
		if e := owner.Close(context.Background()); e != nil {
			result = e
		}
	}()
	b := &backend{owner: owner, release: r, factory: factory, signer: signer, open: open, expected: first.VaultExpected, product: processowner.Spec{Executable: filepath.Join(dir, "tls-product.exe"), SHA256: embeddedProductDigest}}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	pipe := `\\.\pipe\remoteapp-session-` + first.VaultScope
	ln, e := desktopcontrol.Listen(pipe)
	if e != nil {
		return e
	}
	done := make(chan struct{})
	go func() { defer close(done); desktopcontrol.Serve(ctx, ln, first.VaultScope, first.VaultExpected, b) }()
	defer func() { cancel(); ln.Close(); <-done }()
	commands := make(chan string)
	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		defer close(commands)
		var command struct{ Op string }
		if reader.Read(&command) == nil {
			select {
			case commands <- command.Op:
			case <-ctx.Done():
			}
		}
	}()
	defer func() { cancel(); os.Stdin.Close(); <-readerDone }()
	if e = json.NewEncoder(os.Stdout).Encode(map[string]any{"status": "ready", "pipe": pipe, "scope": first.VaultScope, "public_key": first.VaultExpected, "channel": r.Specification().Channel, "epoch": r.Specification().Bundled.Claims.Epoch}); e != nil {
		return e
	}
	select {
	case command, ok := <-commands:
		if !ok || command != "stop" {
			return privatepipe.ErrProtocol
		}
		return nil
	case <-ctx.Done():
		return privatepipe.ErrProtocol
	case <-done:
		return privatepipe.ErrProtocol
	}
}
