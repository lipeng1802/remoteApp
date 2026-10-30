package processowner

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/privatepipe"
)

// A real child process, not Headscale, native credentials or product TLS.
func TestProcessFixture(t *testing.T) {
	if len(os.Args) < 2 || !strings.HasPrefix(os.Args[len(os.Args)-1], "fixture-") {
		return
	}
	mode := strings.TrimPrefix(os.Args[len(os.Args)-1], "fixture-")
	if mode == "diagnostic" {
		os.Stderr.WriteString("PRODUCT_STAGE controller_dial\n")
		os.Stderr.WriteString(strings.Repeat("PRIVATE", 100000) + "\n")
		os.Stderr.WriteString("PRODUCT_STAGE PRIVATE\n")
		os.Stderr.WriteString("PRODUCT_STAGE controller_secret\n")
		os.Stderr.WriteString("PRODUCT_STAGE controller_tls PRIVATE\n")
		os.Stderr.WriteString("PRODUCT_STAGE controller_bridge")
	}

	if mode == "malformed" {
		os.Stdout.WriteString("{\"status\":\"ready\",\"status\":\"ready\"}\n")
		os.Exit(0)
	}
	if mode != "silent" && mode != "bridge-target" && mode != "bridge-controller" && mode != "controller" {
		json.NewEncoder(os.Stdout).Encode(map[string]string{"status": "ready"})
	}
	if mode == "crash" {
		time.Sleep(100 * time.Millisecond)
		os.Exit(7)
	}
	r := privatepipe.NewReader(os.Stdin)
	for {
		var v struct{ Op string }
		if r.Read(&v) != nil {
			os.Exit(0)
		}
		if v.Op == "stop" || v.Op == "maintenance_finish" {
			json.NewEncoder(os.Stdout).Encode(map[string]string{"status": "closed"})
			if mode == "slow" || mode == "finish-slow" {
				time.Sleep(600 * time.Millisecond)
			}
			if mode == "falseclosed" {
				os.Exit(7)
			}
			if mode == "finish-duplicate" {
				json.NewEncoder(os.Stdout).Encode(map[string]string{"status": "closed"})
			}
			os.Exit(0)
		}
		if v.Op == "count" {
			json.NewEncoder(os.Stdout).Encode(map[string]string{"status": "count"})
		}
		if v.Op == "start" {
			if mode == "controller" {
				json.NewEncoder(os.Stdout).Encode(map[string]string{"status": "ready"})
			} else {
				port := 0
				if mode == "bridge-controller" {
					port = 32123
				}
				json.NewEncoder(os.Stdout).Encode(map[string]any{"status": "ready", "port": port})
			}
		}
	}
}

func TestMaintenanceFinishWaitsForActualExitAndRetainsJoin(t *testing.T) {
	c := start(t, "finish-slow")
	ready(t, c)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if e := c.FinishMaintenance(ctx); !errors.Is(e, context.DeadlineExceeded) {
		t.Fatal("finish did not wait", e)
	}
	select {
	case <-c.Exited():
		t.Fatal("fixture exited too early")
	default:
	}
	select {
	case <-c.Done():
		t.Fatal("closed frame treated as join")
	default:
	}
	life, stop := limit()
	defer stop()
	if e := c.Stop(life); e != nil {
		t.Fatal("explicit retry did not join original finish", e)
	}
}

func TestMaintenanceFinishRejectsFailedDuplicateAndPriorStop(t *testing.T) {
	for _, mode := range []string{"falseclosed", "finish-duplicate", "normal"} {
		t.Run(mode, func(t *testing.T) {
			c := start(t, mode)
			ready(t, c)
			ctx, cancel := limit()
			defer cancel()
			if mode == "normal" {
				if e := c.Stop(ctx); e != nil {
					t.Fatal(e)
				}
			}
			if e := c.FinishMaintenance(ctx); e == nil {
				t.Fatal("invalid finish accepted")
			}
			select {
			case <-c.Done():
			case <-ctx.Done():
				t.Fatal("failed finish not joined")
			}
		})
	}
}
func spec(t *testing.T, mode string) Spec {
	t.Helper()
	exe, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	exe, e = filepath.EvalSymlinks(exe)
	if e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(exe)
	if e != nil {
		t.Fatal(e)
	}
	h := sha256.Sum256(b)
	return Spec{Executable: exe, SHA256: hex.EncodeToString(h[:]), Args: []string{"-test.run=^TestProcessFixture$", "fixture-" + mode}}
}
func start(t *testing.T, mode string) *Child {
	t.Helper()
	c, e := Start(spec(t, mode))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		c.Stop(ctx)
		select {
		case <-c.Done():
		default:
			c.cmd.Process.Kill()
			c.in.Close()
			t.Error("fixture cleanup required forced kill; not successful join")
		}
	})
	return c
}
func ready(t *testing.T, c *Child) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	v, e := c.Exchange(ctx, nil)
	if e != nil || string(v) != "{\"status\":\"ready\"}" {
		t.Fatalf("ready unavailable: %v", e)
	}
}
func limit() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 5*time.Second)
}

func TestChildGracefulStopJoinsAndRejectsReuse(t *testing.T) {
	c := start(t, "normal")
	ready(t, c)
	ctx, cancel := limit()
	defer cancel()
	if _, e := c.Exchange(ctx, map[string]string{"Op": "count"}); e != nil {
		t.Fatal(e)
	}
	if e := c.Stop(ctx); e != nil {
		t.Fatal(e)
	}
	select {
	case <-c.readDone:
	default:
		t.Fatal("reader not joined")
	}
	select {
	case <-c.Exited():
	default:
		t.Fatal("process not joined")
	}
	if _, e := c.Exchange(ctx, map[string]string{"Op": "count"}); e == nil {
		t.Fatal("reused closed child")
	}
}
func TestChildDeadlineRetainsUntilExitThenRetryJoins(t *testing.T) {
	c := start(t, "slow")
	ready(t, c)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if e := c.Stop(ctx); !errors.Is(e, context.DeadlineExceeded) {
		t.Fatalf("expected deadline: %v", e)
	}
	select {
	case <-c.Done():
		t.Fatal("closed ack incorrectly substituted for process join")
	default:
	}
	ctx2, cancel2 := limit()
	defer cancel2()
	if e := c.Stop(ctx2); e != nil {
		t.Fatal(e)
	}
}
func TestChildClosedReplyWithNonzeroExitFails(t *testing.T) {
	c := start(t, "falseclosed")
	ready(t, c)
	ctx, cancel := limit()
	defer cancel()
	if e := c.Stop(ctx); !errors.Is(e, ErrProcess) {
		t.Fatalf("false closed accepted: %v", e)
	}
}
func TestChildTimeoutCannotReuseLateReply(t *testing.T) {
	c := start(t, "silent")
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, e := c.Exchange(ctx, nil); !errors.Is(e, context.DeadlineExceeded) {
		t.Fatal(e)
	}
	ctx2, cancel2 := limit()
	defer cancel2()
	if _, e := c.Exchange(ctx, nil); e == nil {
		t.Fatal("timeout reuse accepted")
	}
	if e := c.Stop(ctx2); !errors.Is(e, ErrProcess) {
		t.Fatal("timeout reported clean")
	}
}
func TestChildPinAndDuplicateFrameFailClosed(t *testing.T) {
	s := spec(t, "normal")
	s.SHA256 = strings.Repeat("0", 64)
	if c, e := Start(s); e == nil || c != nil {
		t.Fatal("wrong digest launched")
	}
	c := start(t, "malformed")
	ctx, cancel := limit()
	defer cancel()
	if _, e := c.Exchange(ctx, nil); e == nil {
		t.Fatal("duplicate accepted")
	}
	if e := c.Stop(ctx); e == nil {
		t.Fatal("malformed output reported clean")
	}
}
func TestPairChildCrashStopsOtherAndFails(t *testing.T) {
	product := start(t, "normal")
	ready(t, product)
	bridge := start(t, "crash")
	ready(t, bridge)
	p, e := NewPair(bridge, product)
	if e != nil {
		t.Fatal(e)
	}
	select {
	case <-p.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("paired cleanup did not join")
	}
	ctx, cancel := limit()
	defer cancel()
	if e := p.Close(ctx); e == nil {
		t.Fatal("crashed pair reported clean")
	}
	select {
	case <-product.Done():
	default:
		t.Fatal("product survived bridge crash")
	}
}
func TestPairGracefulCloseJoinsBoth(t *testing.T) {
	bridge := start(t, "normal")
	ready(t, bridge)
	product := start(t, "slow")
	ready(t, product)
	p, e := NewPair(bridge, product)
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if e := p.Close(ctx); !errors.Is(e, context.DeadlineExceeded) {
		t.Fatal(e)
	}
	ctx2, cancel2 := limit()
	defer cancel2()
	if e := p.Close(ctx2); e != nil {
		t.Fatal(e)
	}
	select {
	case <-p.monitorDone:
	default:
		t.Fatal("pair monitor not joined")
	}
	for _, c := range []*Child{bridge, product} {
		select {
		case <-c.Done():
		default:
			t.Fatal("not joined")
		}
	}
}

func TestEndpointTargetPreparationAndBothJoins(t *testing.T) {
	prepared := false
	ctx, cancel := limit()
	defer cancel()
	l, e := StartEndpoint(ctx, Plan{Target: true, Bridge: spec(t, "bridge-target"), Product: spec(t, "normal"), BridgeStart: map[string]string{"Op": "start"}, PrepareTarget: func(ctx context.Context, c *Child, v json.RawMessage) error {
		prepared = true
		_, e := c.Exchange(ctx, map[string]string{"Op": "count"})
		return e
	}})
	if e != nil || !prepared || l.Pair == nil {
		t.Fatal("target launch unavailable", e)
	}
	if e := l.Close(ctx); e != nil {
		t.Fatal(e)
	}
}
func TestEndpointControllerConfigFromBridgeReady(t *testing.T) {
	ctx, cancel := limit()
	defer cancel()
	configured := false
	l, e := StartEndpoint(ctx, Plan{Bridge: spec(t, "bridge-controller"), Product: spec(t, "controller"), BridgeStart: map[string]string{"Op": "start"}, ControllerConfig: func(ctx context.Context, v json.RawMessage) (any, error) {
		configured = bridgeReady(v, false)
		return map[string]string{"Op": "start"}, nil
	}})
	if e != nil || !configured {
		t.Fatal("controller launch unavailable", e)
	}
	if e := l.Close(ctx); e != nil {
		t.Fatal(e)
	}
}
func TestEndpointPreparationFailureJoinsPartialProduct(t *testing.T) {
	ctx, cancel := limit()
	defer cancel()
	l, e := StartEndpoint(ctx, Plan{Target: true, Bridge: spec(t, "bridge-target"), Product: spec(t, "normal"), BridgeStart: map[string]string{"Op": "start"}, PrepareTarget: func(context.Context, *Child, json.RawMessage) error { return ErrProcess }})
	if e == nil || l.Bridge != nil || l.Product == nil {
		t.Fatal("failed preparation launched bridge")
	}
	select {
	case <-l.Product.Done():
	default:
		t.Fatal("partial product not joined")
	}
}

func TestFixedStageCaptureIsOptInAndJoined(t *testing.T) {
	for _, capture := range []bool{false, true} {
		s := spec(t, "diagnostic")
		s.CaptureProductStage = capture
		c, e := Start(s)
		if e != nil {
			t.Fatal(e)
		}
		ready(t, c)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		e = c.Stop(ctx)
		cancel()
		if e != nil {
			t.Fatal(e)
		}
		want := "unreported"
		if capture {
			want = "controller_secret"
		}
		if c.ProductStage() != want {
			t.Fatalf("unexpected fixed stage %q", c.ProductStage())
		}
		select {
		case <-c.diagnosticDone:
		default:
			t.Fatal("diagnostic reader not joined")
		}
	}
}
