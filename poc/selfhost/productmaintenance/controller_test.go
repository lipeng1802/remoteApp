package productmaintenance

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/privatepipe"
	"remoteapp.local/selfhost-poc/processowner"
	"remoteapp.local/selfhost-poc/trustinstall"
)

// Real executable fixture exercises closed-vs-exit, not native product stores.
func TestProductProcessFixture(t *testing.T) {
	if len(os.Args) < 2 || !strings.HasPrefix(os.Args[len(os.Args)-1], "r58-fixture-") {
		return
	}
	mode := strings.TrimPrefix(os.Args[len(os.Args)-1], "r58-fixture-")
	json.NewEncoder(os.Stdout).Encode(map[string]string{"status": "ready"})
	reader := privatepipe.NewReader(os.Stdin)
	var command struct{ Op string }
	if reader.Read(&command) != nil {
		os.Exit(8)
	}
	if command.Op != "stop" && command.Op != "maintenance_finish" {
		os.Exit(9)
	}
	if mode == "extra" {
		json.NewEncoder(os.Stdout).Encode(map[string]any{"status": "closed", "joined": true})
	} else {
		json.NewEncoder(os.Stdout).Encode(map[string]string{"status": "closed"})
	}
	if mode == "slow" {
		time.Sleep(500 * time.Millisecond)
	}
	if mode == "duplicate" {
		json.NewEncoder(os.Stdout).Encode(map[string]string{"status": "closed"})
	}
	if mode == "nonzero" {
		os.Exit(7)
	}
	os.Exit(0)
}

type testSigner struct{ closes atomic.Int32 }

func (s *testSigner) PublicKey() ed25519.PublicKey { return make([]byte, 32) }
func (s *testSigner) Sign([]byte) ([]byte, error)  { return nil, ErrProduct }
func (s *testSigner) CloseJoined() error           { s.closes.Add(1); return nil }

func controllerFixture(t *testing.T, mode string) (*Controller, *testSigner, *atomic.Int32) {
	t.Helper()
	exe, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	exe, e = filepath.EvalSymlinks(exe)
	if e != nil {
		t.Fatal(e)
	}
	raw, e := os.ReadFile(exe)
	if e != nil {
		t.Fatal(e)
	}
	hash := sha256.Sum256(raw)
	child, e := processowner.Start(processowner.Spec{Executable: exe, SHA256: hex.EncodeToString(hash[:]), Args: []string{"-test.run=^TestProductProcessFixture$", "r58-fixture-" + mode}})
	if e != nil {
		t.Fatal(e)
	}
	life, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, e = child.Exchange(life, nil); e != nil {
		t.Fatal(e)
	}
	signer := &testSigner{}
	checks := &atomic.Int32{}
	c := &Controller{child: child, signer: signer, prepared: true, serial: make(chan struct{}, 1), trust: func(bool) (trustinstall.Journal, error) { checks.Add(1); return nil, ErrProduct }}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		c.Release(ctx)
		select {
		case <-child.Done():
		default:
			t.Error("fixture child not joined")
		}
	})
	return c, signer, checks
}

func TestProductCloseTimeoutCannotBecomeSuccessAfterLateExit(t *testing.T) {
	c, signer, checks := controllerFixture(t, "slow")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if e := c.Close(ctx); !errors.Is(e, context.DeadlineExceeded) {
		t.Fatal(e)
	}
	select {
	case <-c.Exited():
		t.Fatal("closed treated as actual exit")
	default:
	}
	if checks.Load() != 0 || signer.closes.Load() != 0 {
		t.Fatal("native check or signer release before exit")
	}
	if e := c.Release(context.Background()); e != nil {
		t.Fatal(e)
	}
	if signer.closes.Load() != 1 {
		t.Fatal("signer not joined exactly once")
	}
	if e := c.Close(context.Background()); e == nil {
		t.Fatal("late exit rearmed success")
	}
}

func TestProductCloseRejectsFaultyExitAndExtraReplies(t *testing.T) {
	for _, mode := range []string{"nonzero", "duplicate", "extra"} {
		t.Run(mode, func(t *testing.T) {
			c, signer, checks := controllerFixture(t, mode)
			if e := c.Close(context.Background()); e == nil {
				t.Fatal("faulty process authorized close")
			}
			if checks.Load() != 0 {
				t.Fatal("faulty exit reached native handoff")
			}
			if e := c.Release(context.Background()); e == nil {
				t.Fatal("fault hidden by teardown")
			}
			if signer.closes.Load() != 1 {
				t.Fatal("signer retained after actual failed join")
			}
		})
	}
}

func TestProductCloseRequiresNativePostExitVerification(t *testing.T) {
	c, _, checks := controllerFixture(t, "normal")
	if e := c.Close(context.Background()); e == nil {
		t.Fatal("exit alone authorized deletion")
	}
	if checks.Load() != 1 {
		t.Fatal("missing native recheck")
	}
	select {
	case <-c.Exited():
	default:
		t.Fatal("native recheck before exit")
	}
	if e := c.Close(context.Background()); e == nil || checks.Load() != 1 {
		t.Fatal("postcheck failure retried as success")
	}
}

func TestProductReleaseRetainsSignerOnJoinDeadline(t *testing.T) {
	c, signer, _ := controllerFixture(t, "slow")
	c.prepared = false
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if e := c.Release(ctx); !errors.Is(e, context.DeadlineExceeded) {
		t.Fatal(e)
	}
	if signer.closes.Load() != 0 {
		t.Fatal("signer closed before child join")
	}
	if e := c.Release(context.Background()); e != nil {
		t.Fatal(e)
	}
	if signer.closes.Load() != 1 {
		t.Fatal("signer close count")
	}
}
