// Package cleanupwitness records trusted in-process cleanup barriers for one
// TEST product lifetime. It accepts no peer frames and stores no error text,
// paths, keys or authorization material. Completion is not session permission.
package cleanupwitness

import (
	"context"
	"sync"
)

type Resource uint8

const (
	OwnerJournal Resource = iota
	BindingJournal
	CommandReader
	Runner
	Signer
	Node
	Watch
	Session
	TLSProduct
	TransportListener
	resourceCount
)

type Witness struct {
	mu                    sync.Mutex
	started, completed    uint16
	armed, sealed, failed bool
}
type contextKey struct{}

func Context(ctx context.Context, w *Witness) context.Context {
	return context.WithValue(ctx, contextKey{}, w)
}
func From(ctx context.Context) *Witness {
	if ctx == nil {
		return nil
	}
	w, _ := ctx.Value(contextKey{}).(*Witness)
	return w
}
func (w *Witness) Begin(r Resource) {
	if w == nil {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if r >= resourceCount || w.sealed || w.started&(1<<r) != 0 {
		w.failed = true
		return
	}
	w.started |= 1 << r
}
func (w *Witness) Complete(r Resource, err error) {
	if w == nil {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if r >= resourceCount || w.sealed || w.started&(1<<r) == 0 || w.completed&(1<<r) != 0 || err != nil {
		w.failed = true
		return
	}
	w.completed |= 1 << r
}

// Arm happens only after the native TEST runner's initial ready was written.
func (w *Witness) Arm(test bool) {
	if w == nil {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if !test || w.sealed || w.armed {
		w.failed = true
		return
	}
	w.armed = true
}

// Seal is called only after the root entry and every defer have returned.
func (w *Witness) Seal() {
	if w == nil {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.sealed {
		w.failed = true
	}
	w.sealed = true
}
func (w *Witness) Verified() bool {
	if w == nil {
		return false
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	const mandatory = 1<<OwnerJournal | 1<<BindingJournal | 1<<CommandReader | 1<<Runner | 1<<Signer
	return w.sealed && w.armed && !w.failed && w.started&mandatory == mandatory && w.started == w.completed
}
