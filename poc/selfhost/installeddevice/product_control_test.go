package installeddevice

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/clientowner"
	"remoteapp.local/selfhost-poc/clientruntime"
	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/trustprofile"
)

type controlHandle struct {
	done chan struct{}
	err  error
}

func TestProductCountIsTestOnlySelectorFreeAndNonterminal(t *testing.T) {
	for _, mode := range []string{"available", "not_started", "disabled", "override"} {
		t.Run(mode, func(t *testing.T) {
			o := &controlOwner{}
			h := &controlHandle{done: make(chan struct{})}
			requests := make(chan Request, 2)
			commands := make(chan Request)
			count := Request{Op: "product_count"}
			if mode == "override" {
				count.ExpectedPublic = "override"
			}
			requests <- count
			var observe func(context.Context) (clientruntime.ProductCount, error)
			if mode != "disabled" {
				observe = func(context.Context) (clientruntime.ProductCount, error) {
					if mode == "not_started" {
						return clientruntime.ProductCount{}, errors.New("not bound")
					}
					return clientruntime.ProductCount{Status: "product_count", Pongs: 5}, nil
				}
			}
			if mode == "available" || mode == "not_started" {
				requests <- Request{Op: "stop"}
				go func() { <-commands; close(h.done) }()
			}
			outputs := 0
			e := controlProductObserved(context.Background(), o, h, requests, commands, func(v any) error { outputs++; return nil }, nil, observe)
			if mode == "available" || mode == "not_started" {
				if e != nil || outputs != 1 {
					t.Fatal(e)
				}
			} else if e == nil || outputs != 0 {
				t.Fatal("unsafe count reached producer")
			}
			if o.calls.Load() != 0 {
				t.Fatal("telemetry entered maintenance")
			}
		})
	}
}

func TestProductPrepareRequiresExactFrameAndExplicitStateAdapter(t *testing.T) {
	for _, mode := range []string{"unavailable", "override", "valid", "bad_finish"} {
		t.Run(mode, func(t *testing.T) {
			o := &controlOwner{}
			h := &controlHandle{done: make(chan struct{})}
			requests := make(chan Request, 2)
			begin := Request{Op: "maintenance_prepare"}
			if mode == "override" {
				begin.ExpectedPublic = "override"
			}
			requests <- begin
			finish := Request{Op: "maintenance_finish"}
			if mode == "bad_finish" {
				finish.Op = "stop"
			}
			requests <- finish
			calls := 0
			var prepare func(context.Context, func(context.Context, trustprofile.MaintenanceState) error) error
			if mode != "unavailable" {
				prepare = func(ctx context.Context, action func(context.Context, trustprofile.MaintenanceState) error) error {
					calls++
					return action(ctx, trustprofile.MaintenanceState{State: "pending", Epoch: 2, Transaction: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"})
				}
			}
			outputs := 0
			e := controlProduct(context.Background(), o, h, requests, make(chan Request), func(v any) error {
				outputs++
				r := v.(map[string]any)
				if r["transaction"] != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" || r["joined"] != true || len(r) != 4 {
					t.Fatal(r)
				}
				return nil
			}, prepare)
			if mode == "valid" {
				if e != nil || calls != 1 || outputs != 1 {
					t.Fatal(e)
				}
			} else if e == nil {
				t.Fatal("invalid prepare accepted")
			}
			if (mode == "unavailable" || mode == "override") && calls != 0 {
				t.Fatal("override reached durable owner")
			}
		})
	}
}

func (h *controlHandle) Done() <-chan struct{}      { return h.done }
func (h *controlHandle) Wait(context.Context) error { return h.err }

type controlOwner struct {
	calls   atomic.Int32
	join    <-chan struct{}
	fail    bool
	holding atomic.Bool
}

func (o *controlOwner) MaintainDurable(ctx context.Context, action clientowner.MaintenanceAction) error {
	o.calls.Add(1)
	if o.fail {
		return errors.New("unjoined fault")
	}
	if o.join != nil {
		select {
		case <-o.join:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	o.holding.Store(true)
	defer o.holding.Store(false)
	return action(ctx, 2)
}

func TestProductMaintenanceWaitsForJoinAndExplicitFinish(t *testing.T) {
	join := make(chan struct{})
	o := &controlOwner{join: join}
	h := &controlHandle{done: make(chan struct{})}
	requests := make(chan Request, 2)
	commands := make(chan Request)
	requests <- Request{Op: "maintenance_begin"}
	output := make(chan any, 1)
	done := make(chan error, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		done <- ControlProduct(ctx, o, h, requests, commands, func(v any) error { output <- v; return nil })
	}()
	select {
	case <-output:
		t.Fatal("ready before join")
	case <-time.After(10 * time.Millisecond):
	}
	close(join)
	select {
	case value := <-output:
		r := value.(map[string]any)
		if r["status"] != "maintenance_ready" || r["epoch"] != uint64(2) || r["joined"] != true || len(r) != 3 || !o.holding.Load() {
			t.Fatal("invalid held readiness")
		}
	case <-time.After(time.Second):
		t.Fatal("no readiness")
	}
	select {
	case <-done:
		t.Fatal("lease ended before finish")
	default:
	}
	requests <- Request{Op: "maintenance_finish"}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if o.calls.Load() != 1 || o.holding.Load() {
		t.Fatal("incorrect owner lifetime")
	}
}

func TestProductMaintenanceRejectsOverridesAndBrokenHandshake(t *testing.T) {
	for _, scenario := range []string{"begin_override", "early_finish", "finish_override", "closed_pipe", "owner_fault", "output_fault"} {
		t.Run(scenario, func(t *testing.T) {
			o := &controlOwner{fail: scenario == "owner_fault"}
			h := &controlHandle{done: make(chan struct{})}
			requests := make(chan Request, 2)
			begin := Request{Op: "maintenance_begin"}
			if scenario == "begin_override" {
				begin.Device = invite.Body{Action: "register"}
			}
			if scenario == "early_finish" {
				begin.Op = "maintenance_finish"
			}
			requests <- begin
			if scenario == "finish_override" {
				requests <- Request{Op: "maintenance_finish", ExpectedPublic: "override"}
			} else {
				close(requests)
			}
			err := ControlProduct(context.Background(), o, h, requests, make(chan Request), func(any) error {
				if scenario == "output_fault" {
					return errors.New("write failed")
				}
				return nil
			})
			if err == nil {
				t.Fatal("accepted invalid handshake")
			}
			if (scenario == "begin_override" || scenario == "early_finish") && o.calls.Load() != 0 {
				t.Fatal("override reached owner")
			}
			if o.holding.Load() {
				t.Fatal("handshake did not return")
			}
		})
	}
}

func TestProductControlPreservesNormalCommandsAndExit(t *testing.T) {
	o := &controlOwner{}
	h := &controlHandle{done: make(chan struct{})}
	requests := make(chan Request, 1)
	commands := make(chan Request)
	requests <- Request{Op: "stop"}
	done := make(chan error, 1)
	go func() {
		done <- ControlProduct(context.Background(), o, h, requests, commands, func(any) error { return nil })
	}()
	if req := <-commands; req != (Request{Op: "stop"}) {
		t.Fatal("normal command changed")
	}
	close(requests) // parent EOF can precede the runner's actual completion
	select {
	case err := <-done:
		t.Fatal("returned before runner joined", err)
	case <-time.After(20 * time.Millisecond):
	}
	close(h.done)
	if err := <-done; err != nil || o.calls.Load() != 0 {
		t.Fatal("normal exit changed", err)
	}
}

func TestProductMaintenanceCancellationReleasesHeldHandshake(t *testing.T) {
	o := &controlOwner{}
	h := &controlHandle{done: make(chan struct{})}
	requests := make(chan Request, 1)
	requests <- Request{Op: "maintenance_begin"}
	ready := make(chan struct{})
	done := make(chan error, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		done <- ControlProduct(ctx, o, h, requests, make(chan Request), func(any) error { close(ready); return nil })
	}()
	<-ready
	if !o.holding.Load() {
		t.Fatal("handshake not held")
	}
	cancel()
	select {
	case err := <-done:
		if err == nil || o.holding.Load() {
			t.Fatal("cancellation accepted as maintenance success")
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled handshake not joined")
	}
}
