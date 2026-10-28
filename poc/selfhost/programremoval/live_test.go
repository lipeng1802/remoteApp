package programremoval

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/clientowner"
	"remoteapp.local/selfhost-poc/installrelease"
	"remoteapp.local/selfhost-poc/trustinstall"
	"remoteapp.local/selfhost-poc/trustprofile"
)

func waitSignal(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(8 * time.Second):
		t.Fatal("timeout: " + what)
	}
}

// Both workers own actual TLS HTTP streams. No mock completion/status reply is
// used to release the native lease. Delayed final runner return tests that a
// cancellation acknowledgment cannot authorize deletion before actual join.
func TestNativeRemovalLiveTLSJoinAndExplicitContinuation(t *testing.T) {
	for _, timeout := range []bool{false, true} {
		name := "joined"
		if timeout {
			name = "join-cancelled"
		}
		t.Run(name, func(t *testing.T) {
			r, factory, root, _, _ := nativeFixtureMode(t, false)
			o, err := clientowner.Open(r, factory, time.Now)
			if err != nil {
				t.Fatal(err)
			}
			allowJoin := make(chan struct{})
			var releaseOnce sync.Once
			release := func() { releaseOnce.Do(func() { close(allowJoin) }) }
			t.Cleanup(func() { release(); o.Close(context.Background()) })
			ready, ended := []chan struct{}{make(chan struct{}), make(chan struct{})}, []chan struct{}{make(chan struct{}), make(chan struct{})}
			servers := make([]*httptest.Server, 2)
			defer func() {
				release()
				o.Close(context.Background())
				for _, server := range servers {
					if server != nil {
						server.CloseClientConnections()
						server.Close()
					}
				}
			}()
			for i := range servers {
				servers[i] = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
					w.WriteHeader(http.StatusOK)
					w.(http.Flusher).Flush()
					close(ready[i])
					<-req.Context().Done()
					close(ended[i])
				}))
			}
			workersJoined := make(chan struct{})
			h, err := o.Start(context.Background(), func(life context.Context, _ trustprofile.Profile) error {
				var workers sync.WaitGroup
				for _, server := range servers {
					workers.Add(1)
					go func() {
						defer workers.Done()
						client := server.Client()
						defer client.CloseIdleConnections()
						req, e := http.NewRequestWithContext(life, http.MethodGet, server.URL, nil)
						if e != nil {
							return
						}
						resp, e := client.Do(req)
						if e != nil {
							return
						}
						defer resp.Body.Close()
						io.Copy(io.Discard, resp.Body)
					}()
				}
				<-life.Done()
				workers.Wait()
				close(workersJoined)
				<-allowJoin
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			for i := range ready {
				waitSignal(t, ready[i], "actual TLS stream ready")
			}
			// A coordinator for another compiled release cannot retire this owner.
			wrong := r.Specification()
			wrong.Channel += "-other"
			encoded, e := trustinstall.EncodeForTrustedBuild(wrong, time.Now())
			if e != nil {
				t.Fatal(e)
			}
			other, e := trustinstall.DecodeCompiled(encoded)
			if e != nil {
				t.Fatal(e)
			}
			if _, _, e = QuiesceAndRemoveTest(context.Background(), other, factory, root, o); e == nil {
				t.Fatal("wrong release accepted")
			}
			select {
			case <-workersJoined:
				t.Fatal("wrong release cancelled live owner")
			default:
			}
			life, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			type reply struct {
				expected, result trustprofile.MaintenanceState
				err              error
			}
			done := make(chan reply, 1)
			go func() {
				expected, result, e := QuiesceAndRemoveTest(life, r, factory, root, o)
				done <- reply{expected, result, e}
			}()
			waitSignal(t, workersJoined, "cancelled TLS workers joined")
			for _, ch := range ended {
				waitSignal(t, ch, "server observes TLS request end")
			}
			if h.State().Joined {
				t.Fatal("runner still held but reported joined")
			}
			if _, e = os.Stat(filepath.Join(root, "bootstrap", installrelease.BootstrapPointerName)); e != nil {
				t.Fatal("files removed before join")
			}
			if _, e = os.Lstat(filepath.Join(root, installrelease.RemovalPlanName)); !os.IsNotExist(e) {
				t.Fatal("plan written before join")
			}
			if competing, e := factory(false); e == nil {
				competing.Close()
				t.Fatal("native lease released before join")
			}
			var expected, result trustprofile.MaintenanceState
			if timeout {
				cancel()
				select {
				case reply := <-done:
					if reply.err == nil || reply.expected.Transaction != "" {
						t.Fatal("join timeout authorized removal")
					}
				case <-time.After(3 * time.Second):
					t.Fatal("join deadline ignored")
				}
				release()
				if err = o.MaintainDurableState(context.Background(), r, func(_ context.Context, s trustprofile.MaintenanceState) error { expected = s; return nil }); err != nil {
					t.Fatal(err)
				}
				if err = o.Close(context.Background()); err != nil {
					t.Fatal(err)
				}
				result, err = runExpectedAfterReceipt(t, r, factory, root, expected)
			} else {
				select {
				case <-done:
					t.Fatal("removal returned before runner join")
				default:
				}
				release()
				reply := <-done
				expected, result, err = reply.expected, reply.result, reply.err
				if err == installrelease.ErrRemovalReceipt {
					result, err = runExpectedAfterReceipt(t, r, factory, root, expected)
				}
			}
			if err != nil || result.State != "files_removed" || result.Transaction != expected.Transaction || result.Epoch != 2 || !h.State().Joined {
				t.Fatal("joined removal", result, err)
			}
			if _, e = clientowner.Open(r, factory, time.Now); e == nil {
				t.Fatal("product active after removal")
			}
			t.Log("CHECK two actual TLS streams stopped; final runner join and native lease required; exact durable transaction handoff; program deletion after join PASS")
		})
	}
}

func runExpectedAfterReceipt(t *testing.T, r trustinstall.Release, f trustinstall.Factory, root string, expected trustprofile.MaintenanceState) (trustprofile.MaintenanceState, error) {
	t.Helper()
	for attempt := 0; attempt < 3; attempt++ {
		s, e := RunExpectedTest(context.Background(), r, f, root, expected)
		if e != installrelease.ErrRemovalReceipt {
			return s, e
		}
	}
	return trustprofile.MaintenanceState{}, installrelease.ErrRemovalReceipt
}

func TestNativeRemovalExactHandoffRejectsWrongTransaction(t *testing.T) {
	r, f, root, _, _ := nativeFixture(t)
	j, err := f(false)
	if err != nil {
		t.Fatal(err)
	}
	expected, err := trustprofile.ReadMaintenance(r.Anchor(), j, time.Now)
	j.Close()
	if err != nil {
		t.Fatal(err)
	}
	wrong := expected
	wrong.Transaction = strings.Repeat("f", 32)
	if _, err = RunExpectedTest(context.Background(), r, f, root, wrong); err == nil {
		t.Fatal("foreign maintenance receipt")
	}
	wrong = expected
	wrong.Epoch++
	if _, err = RunExpectedTest(context.Background(), r, f, root, wrong); err == nil {
		t.Fatal("foreign epoch")
	}
	if _, err = os.Lstat(filepath.Join(root, installrelease.RemovalPlanName)); !os.IsNotExist(err) {
		t.Fatal("wrong receipt wrote plan")
	}
	if result, e := runExpectedAfterReceipt(t, r, f, root, expected); e != nil || result.State != "files_removed" || result.Transaction != expected.Transaction {
		t.Fatal(result, e)
	}
}

func TestNativeRemovalRunnerFaultNeverDeletes(t *testing.T) {
	r, f, root, _, _ := nativeFixtureMode(t, false)
	o, err := clientowner.Open(r, f, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close(context.Background())
	ready := make(chan struct{})
	h, err := o.Start(context.Background(), func(life context.Context, _ trustprofile.Profile) error {
		close(ready)
		<-life.Done()
		return errors.New("TEST runner join fault")
	})
	if err != nil {
		t.Fatal(err)
	}
	waitSignal(t, ready, "runner ready")
	expected, _, err := QuiesceAndRemoveTest(context.Background(), r, f, root, o)
	if err == nil || expected.Transaction != "" || !h.State().Joined || !h.State().Fault {
		t.Fatal("fault authorized deletion", err)
	}
	if _, err = os.Stat(filepath.Join(root, "bootstrap", installrelease.BootstrapPointerName)); err != nil {
		t.Fatal("fault deleted program file")
	}
	if _, err = os.Lstat(filepath.Join(root, installrelease.RemovalPlanName)); !os.IsNotExist(err) {
		t.Fatal("fault wrote plan")
	}
	if competing, e := f(false); e == nil {
		competing.Close()
		t.Fatal("fault automatically released live lease")
	}
	// Explicit close can retire a fully joined faulty owner, but must report
	// the fault. It does not authorize automatic removal or active startup.
	if err = o.Close(context.Background()); err == nil {
		t.Fatal("fault close reported success")
	}
	j, err := f(false)
	if err != nil {
		t.Fatal(err)
	}
	state, err := trustprofile.ReadMaintenance(r.Anchor(), j, time.Now)
	j.Close()
	if err != nil || state.State != "pending" || state.Epoch != 2 {
		t.Fatal(state, err)
	}
}
