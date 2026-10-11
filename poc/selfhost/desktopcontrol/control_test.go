package desktopcontrol

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"remoteapp.local/selfhost-poc/clientowner"
	"remoteapp.local/selfhost-poc/clientruntime"
	"remoteapp.local/selfhost-poc/privatepipe"
	"remoteapp.local/selfhost-poc/trustprofile"
	"sync/atomic"
	"testing"
	"time"
)

type backend struct {
	probes            atomic.Int32
	stopping, release chan struct{}
}

type observedBackend struct{ *backend }

type epochBackend struct {
	*backend
	reads atomic.Int32
}

func (b *epochBackend) InstalledEpoch(context.Context) (uint64, error) { b.reads.Add(1); return 2, nil }
func (b *epochBackend) Upgrade(context.Context, trustprofile.Profile) error {
	return ErrUpgradeCandidate
}
func TestInvalidUpgradeKeepsControlPipeAndEpochReadRejectsOverrides(t *testing.T) {
	for _, injected := range []bool{false, true} {
		b := &epochBackend{backend: &backend{}}
		c, _, _ := serveTest(t, b)
		w := json.NewEncoder(c)
		r := privatepipe.NewReader(c)
		w.Encode(map[string]string{"Scope": "scope", "Expected": "public"})
		status(t, r, "ready")
		w.Encode(Request{Op: "upgrade", Profile: trustprofile.Profile{Signature: "invalid"}})
		status(t, r, "upgrade_rejected")
		req := Request{Op: "installation_state"}
		if injected {
			req.Profile.Signature = "override"
		}
		w.Encode(req)
		if injected {
			status(t, r, "failed")
			if b.reads.Load() != 0 {
				t.Fatal("injected profile reached installed state")
			}
			continue
		}
		var reply struct {
			Status string
			Epoch  uint64
		}
		if e := r.Read(&reply); e != nil || reply.Status != "installation_state" || reply.Epoch != 2 || b.reads.Load() != 1 {
			t.Fatal("missing read-only epoch", e)
		}
	}
}

type diagnosticBackend struct {
	*backend
	failure error
}

func (b *diagnosticBackend) Start(context.Context, clientruntime.Config, func(any) error) error {
	return errors.Join(errors.New("private-secret-must-not-be-returned"), b.failure)
}
func TestStartupDiagnosticOnlyReturnsFixedStage(t *testing.T) {
	for _, tc := range []struct {
		failure error
		stage   string
	}{
		{clientruntime.ErrTLSCertificatePublication, "product_tls_publish"},
		{clientruntime.ErrTestCertificateOpen, "product_certificate_open"},
	} {
		b := &diagnosticBackend{backend: &backend{}, failure: tc.failure}
		c, _, _ := serveTest(t, b)
		w := json.NewEncoder(c)
		r := privatepipe.NewReader(c)
		w.Encode(map[string]string{"Scope": "scope", "Expected": "public"})
		status(t, r, "ready")
		w.Encode(Request{Op: "start"})
		var got map[string]string
		if e := r.Read(&got); e != nil || len(got) != 2 || got["status"] != "failed" || got["failure_stage"] != tc.stage {
			t.Fatal("unsafe or missing diagnostic", e)
		}
	}
}

type replacementBackend struct {
	*backend
	reopens atomic.Int32
	fail    bool
}

func (b *replacementBackend) ReopenFault(context.Context) error {
	b.reopens.Add(1)
	if b.fail {
		return privatepipe.ErrProtocol
	}
	return nil
}

func TestReplacementAckIsDistinctIdleAndRejectsAuthorityOverrides(t *testing.T) {
	for _, scenario := range []string{"valid", "config", "profile", "failed", "unsupported"} {
		t.Run(scenario, func(t *testing.T) {
			b := &replacementBackend{backend: &backend{}, fail: scenario == "failed"}
			var implementation Backend = b
			if scenario == "unsupported" {
				implementation = &backend{}
			}
			c, _, _ := serveTest(t, implementation)
			w := json.NewEncoder(c)
			r := privatepipe.NewReader(c)
			w.Encode(map[string]string{"Scope": "scope", "Expected": "public"})
			status(t, r, "ready")
			req := Request{Op: "reopen_fault"}
			if scenario == "config" {
				req.Config.Server = true
			}
			if scenario == "profile" {
				req.Profile.Signature = "injected"
			}
			w.Encode(req)
			if scenario != "valid" {
				status(t, r, "failed")
			} else {
				var ack struct {
					Status  string
					Started *bool
				}
				if e := r.Read(&ack); e != nil || ack.Status != "owner_reopened" || ack.Started == nil || *ack.Started {
					t.Fatal("not idle replacement ACK", e)
				}
			}
			want := int32(0)
			if scenario == "valid" || scenario == "failed" {
				want = 1
			}
			if b.reopens.Load() != want {
				t.Fatal("authority input reached replacement")
			}
		})
	}
}

func (b *observedBackend) SessionState() clientowner.SessionState {
	return clientowner.SessionState{Status: "session_state", Joined: true, Fault: true}
}
func (b *observedBackend) RetireFault(context.Context) error { return nil }

func TestFaultRetirementHasDistinctAckAndRejectsAuthorityInput(t *testing.T) {
	for _, injected := range []bool{false, true} {
		b := &observedBackend{&backend{stopping: make(chan struct{}), release: make(chan struct{})}}
		c, _, _ := serveTest(t, b)
		w := json.NewEncoder(c)
		r := privatepipe.NewReader(c)
		w.Encode(map[string]string{"Scope": "scope", "Expected": "public"})
		status(t, r, "ready")
		req := Request{Op: "retire_fault"}
		if injected {
			req.Config.Server = true
		}
		w.Encode(req)
		if injected {
			status(t, r, "failed")
			continue
		}
		var state clientowner.SessionState
		if e := r.Read(&state); e != nil || state.Status != "fault_retired" || !state.Joined || !state.Fault {
			t.Fatal("fault retirement ACK", e)
		}
		select {
		case <-b.stopping:
			t.Fatal("normal Stop invoked")
		default:
		}
	}
}

func TestSessionStateIsReadOnlyAndDoesNotStop(t *testing.T) {
	b := &observedBackend{&backend{stopping: make(chan struct{}), release: make(chan struct{})}}
	c, _, _ := serveTest(t, b)
	w := json.NewEncoder(c)
	r := privatepipe.NewReader(c)
	w.Encode(map[string]string{"Scope": "scope", "Expected": "public"})
	status(t, r, "ready")
	w.Encode(Request{Op: "session_state"})
	var state clientowner.SessionState
	if e := r.Read(&state); e != nil || !state.Joined || !state.Fault || state.Status != "session_state" {
		t.Fatal("missing actual completion state", e)
	}
	select {
	case <-b.stopping:
		t.Fatal("observation stopped session")
	default:
	}
	w.Encode(Request{Op: "probe"})
	status(t, r, "probe_passed")
}

func TestSessionStateUnsupportedOrInjectedConfigFailsClosed(t *testing.T) {
	for _, req := range []Request{{Op: "session_state"}, {Op: "session_state", Config: clientruntime.Config{Server: true}}, {Op: "session_state", Profile: trustprofile.Profile{Signature: "untrusted"}}} {
		c, _, _ := serveTest(t, &backend{})
		w := json.NewEncoder(c)
		r := privatepipe.NewReader(c)
		w.Encode(map[string]string{"Scope": "scope", "Expected": "public"})
		status(t, r, "ready")
		w.Encode(req)
		status(t, r, "failed")
	}
}

func (b *backend) Probe(context.Context) error { b.probes.Add(1); return nil }
func (b *backend) Count(context.Context) (clientruntime.ProductCount, error) {
	return clientruntime.ProductCount{Status: "product_count", Pongs: 3}, nil
}
func (b *backend) Start(ctx context.Context, c clientruntime.Config, out func(any) error) error {
	if !c.Server {
		return privatepipe.ErrProtocol
	}
	if e := out(map[string]string{"status": "product_ready", "Key": "private-test-key"}); e != nil {
		return e
	}
	return out(map[string]any{"status": "ready", "port": 47476})
}
func (b *backend) Stop(ctx context.Context) error {
	close(b.stopping)
	select {
	case <-b.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (b *backend) Upgrade(context.Context, trustprofile.Profile) error {
	return privatepipe.ErrProtocol
}
func serveTest(t *testing.T, b Backend) (net.Conn, context.CancelFunc, <-chan struct{}) {
	t.Helper()
	ln, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); Serve(ctx, ln, "scope", "public", b) }()
	c, e := net.Dial("tcp", ln.Addr().String())
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		c.Close()
		cancel()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("server did not join")
		}
	})
	return c, cancel, done
}
func status(t *testing.T, r *privatepipe.Reader, want string) {
	t.Helper()
	var reply struct {
		Status, Key string
		Port        int
		PublicKey   string `json:"public_key"`
	}
	if e := r.Read(&reply); e != nil || reply.Status != want {
		t.Fatalf("reply %v %s, want %s", e, reply.Status, want)
	}
}
func TestPrivateStartAndStopAckOnlyAfterJoin(t *testing.T) {
	b := &backend{stopping: make(chan struct{}), release: make(chan struct{})}
	c, _, _ := serveTest(t, b)
	w := json.NewEncoder(c)
	r := privatepipe.NewReader(c)
	w.Encode(map[string]string{"Scope": "scope", "Expected": "public"})
	status(t, r, "ready")
	w.Encode(Request{Op: "probe"})
	status(t, r, "probe_passed")
	w.Encode(Request{Op: "start", Config: clientruntime.Config{Server: true}})
	status(t, r, "product_ready")
	status(t, r, "ready")
	for i := 0; i < 2; i++ {
		w.Encode(Request{Op: "count"})
		var count clientruntime.ProductCount
		if e := r.Read(&count); e != nil || count.Status != "product_count" || count.Pongs != 3 {
			t.Fatal("count relay", e)
		}
	}
	w.Encode(Request{Op: "stop_session"})
	<-b.stopping
	c.SetReadDeadline(time.Now().Add(20 * time.Millisecond))
	var raw json.RawMessage
	if json.NewDecoder(c).Decode(&raw) == nil {
		t.Fatal("ack before join")
	}
	c.SetReadDeadline(time.Time{})
	close(b.release)
	status(t, r, "session_closed")
}
func TestRejectUnknownFieldsAndWrongIdentity(t *testing.T) {
	for _, hello := range []string{`{"Scope":"wrong","Expected":"public"}`, `{"Scope":"scope","Expected":"public","Create":true}`} {
		b := &backend{}
		c, _, _ := serveTest(t, b)
		c.Write([]byte(hello + "\n"))
		status(t, privatepipe.NewReader(c), "failed")
		if b.probes.Load() != 0 {
			t.Fatal("backend called")
		}
	}
	b := &backend{}
	c, _, _ := serveTest(t, b)
	w := json.NewEncoder(c)
	r := privatepipe.NewReader(c)
	w.Encode(map[string]string{"Scope": "scope", "Expected": "public"})
	status(t, r, "ready")
	c.Write([]byte("{\"Op\":\"probe\",\"Executable\":\"untrusted\"}\n"))
	var raw json.RawMessage
	if r.Read(&raw) == nil {
		t.Fatal("unknown field accepted")
	}
}
func TestCancellationJoinsIdleConnections(t *testing.T) {
	b := &backend{}
	c, cancel, done := serveTest(t, b)
	json.NewEncoder(c).Encode(map[string]string{"Scope": "scope", "Expected": "public"})
	status(t, privatepipe.NewReader(c), "ready")
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("idle workers not joined")
	}
}

func TestCountRejectsRuntimeAndProfileOverrides(t *testing.T) {
	for _, req := range []Request{
		{Op: "count", Config: clientruntime.Config{Server: true}},
		{Op: "count", Profile: trustprofile.Profile{Signature: "untrusted"}},
	} {
		c, _, _ := serveTest(t, &backend{})
		w := json.NewEncoder(c)
		r := privatepipe.NewReader(c)
		w.Encode(map[string]string{"Scope": "scope", "Expected": "public"})
		status(t, r, "ready")
		w.Encode(req)
		status(t, r, "failed")
	}
}
