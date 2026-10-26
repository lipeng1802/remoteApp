//go:build windows

package autonomousinstall

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/clientowner"
	"remoteapp.local/selfhost-poc/enrollmentstore"
	"remoteapp.local/selfhost-poc/privatepipe"
	"remoteapp.local/selfhost-poc/trustinstall"
	"remoteapp.local/selfhost-poc/trustprofile"
)

var nativeHelpers = flag.String("native-install-helpers", "", "explicit TEST helper directory")
var nativeGo = flag.String("native-install-go", "", "trusted Go executable for compiled entry validation")

func TestWindowsCompiledAutonomousInstall(t *testing.T) {
	if *nativeHelpers == "" || *nativeGo == "" {
		t.Skip("explicit native TEST helper/build paths required")
	}
	ctx := context.Background()
	dir := t.TempDir()
	digest := func(name string) string {
		b, err := os.ReadFile(filepath.Join(*nativeHelpers, name))
		if err != nil {
			t.Fatal("helper missing")
		}
		if err = os.WriteFile(filepath.Join(dir, name), b, 0600); err != nil {
			t.Fatal(err)
		}
		h := sha256.Sum256(b)
		return hex.EncodeToString(h[:])
	}
	jd, sd := digest("trust-journal.exe"), digest("identity-vault.exe")
	r1, r2 := releases(t, jd)
	journalPath := filepath.Join(dir, "trust-journal.exe")
	f1, err := r1.NativeFactory(journalPath)
	if err != nil {
		t.Fatal(err)
	}
	f2, err := r2.NativeFactory(journalPath)
	if err != nil {
		t.Fatal(err)
	}
	var raw [16]byte
	_, _ = rand.Read(raw[:])
	vaultScope := hex.EncodeToString(raw[:])
	// Explicit new TEST identity, never the existing RemoteAgent credential.
	creator, err := privatepipe.NewSigner(filepath.Join(dir, "identity-vault.exe"), vaultScope, "", true)
	if err != nil {
		t.Fatal("TEST identity create")
	}
	public := hex.EncodeToString(creator.PublicKey())
	scope, err := enrollmentstore.Scope(r1.Scope(), vaultScope, public)
	if err != nil {
		t.Fatal(err)
	}
	installed := false
	t.Cleanup(func() {
		if t.Failed() {
			_ = creator.CloseJoined()
			t.Log("FAIL: preserve isolated TEST state; no reset/retry")
			return
		}
		if installed {
			n, e := enrollmentstore.Open(scope, true)
			if e != nil {
				t.Error("cleanup open")
				return
			}
			if n.CleanupTestAfterJoin() != nil {
				t.Error("node TEST cleanup")
				return
			}
			if trustprofile.DeleteTestDesktopJournal(journalPath, r1.Scope()) != nil {
				t.Error("trust TEST cleanup")
				return
			}
		}
		if creator.DeleteCreatedIdentity() != nil {
			t.Error("identity TEST cleanup/join")
		}
	})
	module, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	for name, r := range map[string]trustinstall.Release{"install-v1.exe": r1, "install-v2.exe": r2} {
		if trustinstall.CompileAutonomousInstaller(ctx, *nativeGo, module, "windows", filepath.Join(dir, name), r.Specification(), sd, time.Now()) != nil {
			t.Fatal("compile installer")
		}
	}
	run := func(name, op, pub string, want bool) {
		t.Helper()
		body, _ := json.Marshal(map[string]string{"op": op, "vault_scope": vaultScope, "expected_public": pub})
		c := exec.Command(filepath.Join(dir, name))
		c.Stdin = bytes.NewReader(append(body, '\n'))
		var out, stderr bytes.Buffer
		c.Stdout = &out
		c.Stderr = &stderr
		if c.Start() != nil {
			t.Fatal("start installer")
		}
		done := make(chan error, 1)
		go func() { done <- c.Wait() }()
		var exit error
		select {
		case exit = <-done:
		case <-time.After(45 * time.Second):
			t.Fatal("actual exit not confirmed; preserve TEST evidence")
		}
		var reply map[string]string
		if stderr.Len() != 0 || json.Unmarshal(out.Bytes(), &reply) != nil || len(reply) != 1 {
			t.Fatal("invalid fixed receipt")
		}
		if want {
			if exit != nil || reply["status"] != "ready" {
				t.Fatal("expected ready")
			}
		} else if exit == nil || reply["status"] != "failed" {
			t.Fatal("expected rejection")
		}
	}
	run("install-v1.exe", "inspect", public, false)
	run("install-v1.exe", "upgrade", public, false)
	run("install-v1.exe", "initialize", strings.Repeat("1", 64), false)
	run("install-v1.exe", "initialize", public, true)
	installed = true
	run("install-v1.exe", "initialize", public, false)
	run("install-v1.exe", "inspect", public, true)
	t.Log("PASS native initialize, duplicate refusal, fixed identity and process join")
	// A live client lease must block maintenance; it is never force-stopped.
	live, err := clientowner.OpenExactRelease(r1, f1, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	run("install-v2.exe", "upgrade", public, false)
	if live.Close(ctx) != nil {
		t.Fatal("client close")
	}
	n, err := enrollmentstore.Open(scope, true)
	if err != nil {
		t.Fatal(err)
	}
	if n.Begin() != nil {
		t.Fatal("TEST attempted transition")
	}
	run("install-v2.exe", "upgrade", public, false)
	if n.Close() != nil {
		t.Fatal("node close")
	}
	run("install-v2.exe", "upgrade", public, true)
	run("install-v2.exe", "inspect", public, true)
	run("install-v1.exe", "upgrade", public, false)
	run("install-v1.exe", "inspect", public, false)
	run("install-v2.exe", "initialize", public, false)
	n, err = enrollmentstore.Open(scope, true)
	if err != nil {
		t.Fatal(err)
	}
	if n.State().Phase != "attempted" || n.Begin() == nil {
		t.Fatal("upgrade reset enrollment")
	}
	_ = n.Close()
	if old, e := clientowner.OpenExactRelease(r1, f1, time.Now); e == nil {
		_ = old.Close(ctx)
		t.Fatal("old runtime accepted")
	}
	current, err := clientowner.OpenExactRelease(r2, f2, time.Now)
	if err != nil {
		t.Fatal("new runtime refused")
	}
	if current.Close(ctx) != nil {
		t.Fatal("runtime join")
	}
	borrower, err := privatepipe.NewSigner(filepath.Join(dir, "identity-vault.exe"), vaultScope, public, false)
	if err != nil || hex.EncodeToString(borrower.PublicKey()) != public {
		t.Fatal("identity replaced")
	}
	if borrower.CloseJoined() != nil {
		t.Fatal("identity join")
	}
	t.Log("PASS native exclusive upgrade, terminal enrollment preservation, old profile/runtime refusal and identity retention")
}
