package programremoval

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/clientowner"
	"remoteapp.local/selfhost-poc/clientruntime"
	"remoteapp.local/selfhost-poc/installrelease"
	"remoteapp.local/selfhost-poc/localmaintenance"
	"remoteapp.local/selfhost-poc/privatepipe"
	"remoteapp.local/selfhost-poc/trustinstall"
	"remoteapp.local/selfhost-poc/trustprofile"
)

type abortFinishSigner struct{ localmaintenance.Signer }

func (s abortFinishSigner) Sign(message []byte) ([]byte, error) {
	if bytes.HasPrefix(message, []byte("remoteapp-local-maintenance-finish-v1\n")) {
		return nil, errors.New("TEST explicit client interruption")
	}
	return s.Signer.Sign(message)
}

type authenticatedClientReply struct {
	Status      string
	Epoch       uint64
	Transaction string
}

// Independent TEST process. Environment carries public random fixture metadata,
// never a seed/auth token. The child only reopens the native identity; it cannot
// initialize it. Actual application wiring must use trusted installed binding.
func TestAuthenticatedRemovalClientFixture(t *testing.T) {
	encoded := os.Getenv("REMOTEAPP_AUTH_MAINT_RELEASE")
	if encoded == "" {
		return
	}
	r, err := trustinstall.DecodeCompiled(encoded)
	if err != nil || !r.Specification().Test {
		t.Fatal("TEST release")
	}
	vault := os.Getenv("REMOTEAPP_VAULT_TEST_EXECUTABLE")
	raw, err := os.ReadFile(vault)
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(raw)
	pub := os.Getenv("REMOTEAPP_AUTH_MAINT_PUBLIC")
	signer, err := clientruntime.NativeSigner(vault, hex.EncodeToString(h[:]), r.Scope(), pub)()
	if err != nil {
		t.Fatal(err)
	}
	var requestSigner localmaintenance.Signer = signer
	abort := os.Getenv("REMOTEAPP_AUTH_MAINT_MODE") == "abort-finish"
	if abort {
		requestSigner = abortFinishSigner{signer}
	}
	public, _ := hex.DecodeString(pub)
	expected, err := localmaintenance.RequestTest(context.Background(), r, os.Getenv("REMOTEAPP_AUTH_MAINT_ADDRESS"), public, requestSigner)
	if closeErr := signer.(*privatepipe.Signer).CloseJoined(); closeErr != nil {
		t.Fatal(closeErr)
	}
	if abort {
		if err == nil {
			t.Fatal("interrupted client succeeded")
		}
		json.NewEncoder(os.Stdout).Encode(authenticatedClientReply{Status: "rejected"})
		os.Exit(0)
	}
	if err != nil {
		t.Fatal("authenticated maintenance", err)
	}
	factory, err := r.NativeFactory(os.Getenv("REMOTEAPP_JOURNAL_TEST_EXECUTABLE"))
	if err != nil {
		t.Fatal(err)
	}
	state, err := runExpectedAfterReceipt(t, r, factory, os.Getenv("REMOTEAPP_AUTH_MAINT_ROOT"), expected)
	if err != nil || state.State != "files_removed" {
		t.Fatal("same-transaction deletion", err)
	}
	json.NewEncoder(os.Stdout).Encode(authenticatedClientReply{state.State, state.Epoch, state.Transaction})
	os.Exit(0) // all native signer/journal children have explicitly joined
}

func TestNativeRemovalIndependentAuthenticatedClient(t *testing.T) {
	for _, mode := range []string{"complete", "abort-finish"} {
		t.Run(mode, func(t *testing.T) {
			r, factory, root, _, _ := nativeFixtureMode(t, false)
			vault := os.Getenv("REMOTEAPP_VAULT_TEST_EXECUTABLE")
			if vault == "" {
				t.Fatal("native vault helper required")
			}
			raw, err := os.ReadFile(vault)
			if err != nil {
				t.Fatal(err)
			}
			h := sha256.Sum256(raw)
			digest := hex.EncodeToString(h[:])
			signer, err := clientruntime.CreateNativeIdentity(vault, digest, r.Scope())
			if err != nil {
				t.Fatal(err)
			}
			public := signer.PublicKey()
			pub := hex.EncodeToString(public)
			defer signer.CloseJoined()
			t.Cleanup(func() {
				if e := clientruntime.DeleteNativeTestIdentity(vault, digest, r.Scope(), pub); e != nil {
					t.Error(e)
				}
			})
			o, err := clientowner.Open(r, factory, time.Now)
			if err != nil {
				t.Fatal(err)
			}
			allow := make(chan struct{})
			var once sync.Once
			release := func() { once.Do(func() { close(allow) }) }
			ready, networkJoined := make(chan struct{}), make(chan struct{})
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				w.WriteHeader(200)
				w.(http.Flusher).Flush()
				close(ready)
				<-req.Context().Done()
			}))
			defer func() { release(); o.Close(context.Background()); server.CloseClientConnections(); server.Close() }()
			handle, err := o.Start(context.Background(), func(life context.Context, _ trustprofile.Profile) error {
				client := server.Client()
				defer client.CloseIdleConnections()
				req, e := http.NewRequestWithContext(life, http.MethodGet, server.URL, nil)
				if e != nil {
					return e
				}
				response, e := client.Do(req)
				if e != nil {
					return e
				}
				io.Copy(io.Discard, response.Body)
				response.Body.Close()
				close(networkJoined)
				<-allow
				if life.Err() == nil {
					return errors.New("TEST stream ended without cancellation")
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			waitSignal(t, ready, "actual TLS stream")
			listener, err := net.Listen("tcp4", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			endpoint, err := localmaintenance.NewTest(r, o, signer, listener)
			if err != nil {
				listener.Close()
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
			defer cancel()
			type serveReply struct {
				state trustprofile.MaintenanceState
				err   error
			}
			served := make(chan serveReply, 1)
			go func() { state, e := endpoint.Serve(ctx); served <- serveReply{state, e} }()
			defer endpoint.Close()
			// Invalid peer cannot cancel a genuine active owner or write pending.
			invalid, err := net.Dial("tcp", listener.Addr().String())
			if err != nil {
				t.Fatal(err)
			}
			invalid.SetDeadline(time.Now().Add(2 * time.Second))
			invalid.Write([]byte("{\"op\":\"hello\",\"path\":\"remove\"}\n"))
			io.ReadAll(invalid)
			invalid.Close()
			select {
			case <-networkJoined:
				t.Fatal("unauthenticated peer cancelled TLS")
			default:
			}
			encoded, err := trustinstall.EncodeForTrustedBuild(r.Specification(), time.Now())
			if err != nil {
				t.Fatal(err)
			}
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestAuthenticatedRemovalClientFixture$")
			cmd.Env = append(os.Environ(), "REMOTEAPP_AUTH_MAINT_RELEASE="+encoded, "REMOTEAPP_AUTH_MAINT_PUBLIC="+pub, "REMOTEAPP_AUTH_MAINT_ADDRESS="+listener.Addr().String(), "REMOTEAPP_AUTH_MAINT_ROOT="+root, "REMOTEAPP_AUTH_MAINT_MODE="+mode)
			type childReply struct {
				out []byte
				err error
			}
			child := make(chan childReply, 1)
			go func() { out, e := cmd.CombinedOutput(); child <- childReply{out, e} }()
			waitSignal(t, networkJoined, "authenticated peer cancels TLS")
			if handle.State().Joined {
				t.Fatal("runner not yet joined")
			}
			if _, e := os.Stat(filepath.Join(root, "bootstrap", installrelease.BootstrapPointerName)); e != nil {
				t.Fatal("deleted before join")
			}
			if _, e := os.Lstat(filepath.Join(root, installrelease.RemovalPlanName)); !os.IsNotExist(e) {
				t.Fatal("plan before join")
			}
			if competing, e := factory(false); e == nil {
				competing.Close()
				t.Fatal("released live lease before join")
			}
			release()
			var result childReply
			select {
			case result = <-child:
			case <-ctx.Done():
				t.Fatal("independent client did not exit")
			}
			var reply authenticatedClientReply
			if result.err != nil || json.Unmarshal(result.out, &reply) != nil {
				t.Fatalf("independent client %v %s", result.err, result.out)
			}
			var serverResult serveReply
			select {
			case serverResult = <-served:
			case <-ctx.Done():
				t.Fatal("endpoint did not join")
			}
			if mode == "complete" {
				if serverResult.err != nil || reply.Status != "files_removed" || reply.Epoch != 2 || reply.Transaction != serverResult.state.Transaction {
					t.Fatal("authenticated transaction mismatch", reply, serverResult.err)
				}
			} else {
				if serverResult.err == nil || reply.Status != "rejected" {
					t.Fatal("interrupted finish accepted")
				}
				if competing, e := factory(false); e == nil {
					competing.Close()
					t.Fatal("disconnect automatically released owner")
				}
				if _, e := os.Lstat(filepath.Join(root, installrelease.RemovalPlanName)); !os.IsNotExist(e) {
					t.Fatal("interrupted finish wrote plan")
				}
				if e := o.Close(context.Background()); e != nil {
					t.Fatal(e)
				}
			}
			journal, err := factory(false)
			if err != nil {
				t.Fatal(err)
			}
			state, err := trustprofile.ReadMaintenance(r.Anchor(), journal, time.Now)
			journal.Close()
			want := "files_removed"
			if mode == "abort-finish" {
				want = "pending"
			}
			if err != nil || state.State != want || state.Epoch != 2 || state.Transaction != serverResult.state.Transaction {
				t.Fatal(state, err)
			}
			message := []byte("remoteapp-invite-request-v1\n" + strings.Repeat("a", 64))
			signature, err := signer.Sign(message)
			if err != nil || !ed25519.Verify(public, message, signature) {
				t.Fatal("OS identity changed", err)
			}
			t.Log("CHECK independent process reopens OS identity; mutual nonce/role-bound proof; actual TLS cancel/join; exact native handoff or interrupted finish retains lease; identity/epoch2 retained PASS")
		})
	}
}

func TestCompiledAuthenticatedRemovalEntry(t *testing.T) {
	goexe, module := os.Getenv("REMOTEAPP_REMOVAL_BUILD_GO"), os.Getenv("REMOTEAPP_REMOVAL_BUILD_MODULE")
	if goexe == "" || module == "" {
		t.Skip("explicit local compiler/module required")
	}
	r, factory, root, _, _ := nativeFixtureMode(t, false)
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	suffix := ""
	if strings.HasSuffix(os.Getenv("REMOTEAPP_JOURNAL_TEST_EXECUTABLE"), ".exe") {
		suffix = ".exe"
	}
	for name, source := range map[string]string{"trust-journal": os.Getenv("REMOTEAPP_JOURNAL_TEST_EXECUTABLE"), "identity-vault": os.Getenv("REMOTEAPP_VAULT_TEST_EXECUTABLE")} {
		raw, e := os.ReadFile(source)
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(filepath.Join(dir, name+suffix), raw, 0500); e != nil {
			t.Fatal(e)
		}
	}
	vault := filepath.Join(dir, "identity-vault"+suffix)
	raw, err := os.ReadFile(vault)
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(raw)
	digest := hex.EncodeToString(h[:])
	signer, err := clientruntime.CreateNativeIdentity(vault, digest, r.Scope())
	if err != nil {
		t.Fatal(err)
	}
	public := hex.EncodeToString(signer.PublicKey())
	defer signer.CloseJoined()
	t.Cleanup(func() {
		if e := clientruntime.DeleteNativeTestIdentity(vault, digest, r.Scope(), public); e != nil {
			t.Error(e)
		}
	})
	o, err := clientowner.Open(r, factory, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close(context.Background())
	handle, err := o.Start(context.Background(), func(life context.Context, _ trustprofile.Profile) error { <-life.Done(); return nil })
	if err != nil {
		t.Fatal(err)
	}
	l, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	endpoint, err := localmaintenance.NewTest(r, o, signer, l)
	if err != nil {
		l.Close()
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	type reply struct {
		state trustprofile.MaintenanceState
		err   error
	}
	done := make(chan reply, 1)
	go func() { s, e := endpoint.Serve(ctx); done <- reply{s, e}; close(done) }()
	defer func() {
		endpoint.Close()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("endpoint not joined")
		}
	}()
	exe := filepath.Join(dir, "authenticated-removal-test"+suffix)
	target := "darwin"
	if suffix != "" {
		target = "windows"
	}
	if err = trustinstall.CompileAuthenticatedRemovalTest(ctx, goexe, module, target, exe, r.Specification(), root, l.Addr().String(), r.Scope(), public, digest, time.Now()); err != nil {
		t.Fatal(err)
	}
	if target == "darwin" {
		if out, e := exec.Command("/usr/bin/codesign", "--force", "--sign", "-", exe).CombinedOutput(); e != nil {
			t.Fatalf("ad-hoc TEST signing %v %s", e, out)
		}
	}
	if out, e := exec.CommandContext(ctx, exe, "--address", l.Addr().String()).CombinedOutput(); e == nil || !bytes.Contains(out, []byte(`"status":"failed"`)) {
		t.Fatalf("runtime selector accepted %v %s", e, out)
	}
	if handle.State().Joined {
		t.Fatal("argument rejection cancelled owner")
	}
	out, err := exec.CommandContext(ctx, exe).CombinedOutput()
	var result authenticatedClientReply
	if err != nil || json.Unmarshal(out, &result) != nil || result.Status != "files_removed" || result.Epoch != 2 {
		t.Fatalf("compiled independent entry %v %s", err, out)
	}
	serverResult := <-done
	if serverResult.err != nil || serverResult.state.Transaction != result.Transaction || !handle.State().Joined {
		t.Fatal("compiled entry transaction", serverResult.err)
	}
	t.Log("CHECK actual fixed compiled independent client + pinned native helpers + runtime override rejected + authenticated owner join + same-transaction file removal PASS")
}
