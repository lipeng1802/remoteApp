package productmaintenance

import (
	"context"
	"encoding/json"
	"remoteapp.local/selfhost-poc/processowner"
	"strings"
	"sync"
)

// ArmedRemoval owns the same hash-pinned independent client from arm through
// terminal receipt/EOF/actual exit. Unknown reads never close stdin or replay.
type ArmedRemoval struct {
	child               *processowner.Child
	mu                  sync.Mutex
	armed               chan struct{}
	done                chan struct{}
	armErr, errorResult error
	attempted           bool
	status              string
	receipt             RemovalReceipt
}
type RemovalReceipt struct {
	Status      string `json:"status"`
	Epoch       uint64 `json:"epoch"`
	Transaction string `json:"transaction"`
	Preserved   bool   `json:"preserved"`
	Joined      bool   `json:"joined"`
	ZeroExit    bool   `json:"zero_exit"`
}

func StartArmedRemoval(ctx context.Context, spec processowner.Spec) (*ArmedRemoval, error) {
	child, e := processowner.Start(spec)
	if e != nil {
		return nil, e
	}
	a := &ArmedRemoval{child: child, armed: make(chan struct{}), done: make(chan struct{}), status: "arming"}
	go func() {
		raw, e := child.Exchange(context.Background(), map[string]string{"Op": "arm"})
		var reply struct {
			Status string `json:"status"`
		}
		if e == nil && (decode(raw, &reply) != nil || reply.Status != "removal_armed") {
			e = ErrProduct
		}
		a.mu.Lock()
		a.armErr = e
		if e == nil {
			a.status = "armed"
		} else {
			a.status = "unknown"
		}
		a.mu.Unlock()
		close(a.armed)
	}()
	select {
	case <-a.armed:
		return a, a.armErr
	case <-ctx.Done():
		return a, ctx.Err()
	}
}
func (a *ArmedRemoval) PID() int     { return a.child.PID() }
func (a *ArmedRemoval) Begin() error { return a.begin("remove") }
func (a *ArmedRemoval) begin(op string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	select {
	case <-a.armed:
	default:
		return ErrProduct
	}
	if a.armErr != nil || a.attempted {
		return ErrProduct
	}
	a.attempted = true
	a.status = "pending"
	go a.finish(op)
	return nil
}
func (a *ArmedRemoval) finish(op string) {
	raw, e := a.child.Exchange(context.Background(), map[string]string{"Op": op})
	var receipt RemovalReceipt
	if e == nil {
		if op == "cancel" {
			var v struct {
				Status string `json:"status"`
			}
			if decode(raw, &v) != nil || v.Status != "removal_cancelled" {
				e = ErrProduct
			}
		} else {
			var v struct {
				Status      string `json:"status"`
				Epoch       uint64 `json:"epoch"`
				Transaction string `json:"transaction"`
				Preserved   bool   `json:"preserved"`
			}
			if decode(raw, &v) != nil || v.Status != "files_removed" || v.Epoch == 0 || !v.Preserved || len(v.Transaction) != 32 || strings.Trim(v.Transaction, "0123456789abcdef") != "" || v.Epoch > 1<<53-1 {
				e = ErrProduct
			} else {
				receipt = RemovalReceipt{Status: v.Status, Epoch: v.Epoch, Transaction: v.Transaction, Preserved: true, Joined: true, ZeroExit: true}
			}
		}
	}
	if e == nil {
		e = a.child.JoinAfterReceipt(context.Background())
	}
	a.mu.Lock()
	a.errorResult = e
	if e != nil {
		a.status = "unknown"
	} else if op == "cancel" {
		a.status = "cancelled"
	} else {
		a.status = "files_removed"
		a.receipt = receipt
	}
	a.mu.Unlock()
	close(a.done)
}
func (a *ArmedRemoval) Status() any {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.status == "files_removed" {
		return a.receipt
	}
	return map[string]string{"status": "removal_" + a.status}
}
func (a *ArmedRemoval) Close(ctx context.Context) error {
	select {
	case <-a.armed:
	case <-ctx.Done():
		return ctx.Err()
	}
	a.mu.Lock()
	failed := a.armErr
	attempted := a.attempted
	a.mu.Unlock()
	if failed != nil {
		return failed
	}
	if !attempted {
		if e := a.begin("cancel"); e != nil {
			return e
		}
	}
	select {
	case <-a.done:
		a.mu.Lock()
		defer a.mu.Unlock()
		return a.errorResult
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (a *ArmedRemoval) MarshalStatus() json.RawMessage {
	raw, _ := json.Marshal(a.Status())
	return raw
}
