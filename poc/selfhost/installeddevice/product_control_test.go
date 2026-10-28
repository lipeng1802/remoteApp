package installeddevice

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/clientowner"
	"remoteapp.local/selfhost-poc/invite"
)

type controlHandle struct {
	done chan struct{}
	err  error
}

func (h *controlHandle) Done() <-chan struct{}      { return h.done }
func (h *controlHandle) Wait(context.Context) error { return h.err }

type controlOwner struct {
	calls   atomic.Int32
	join    <-chan struct{}
	fail    bool
	holding atomic.Bool
}

func (o *controlOwner) Maintain(ctx context.Context, action clientowner.MaintenanceAction) error {
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
