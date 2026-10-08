package processowner

import (
	"context"
	"sync"
)

// Pair owns already-started bridge and product children. The trusted launcher
// controls role-dependent startup and validates readiness/descriptors first.
// No helper argv, roots, certificates or HMACs are accepted from peer input.
type Pair struct {
	bridge, product       *Child
	once                  sync.Once
	done                  chan struct{}
	stopping, monitorDone chan struct{}
	err                   error
}

func NewPair(bridge, product *Child) (*Pair, error) {
	if bridge == nil || product == nil || bridge == product {
		return nil, ErrProcess
	}
	p := &Pair{bridge: bridge, product: product, done: make(chan struct{}), stopping: make(chan struct{}), monitorDone: make(chan struct{})}
	go func() {
		defer close(p.monitorDone)
		select {
		case <-bridge.Exited():
		case <-product.Exited():
		case <-bridge.readDone:
		case <-product.readDone:
		case <-p.stopping:
			return
		}
		p.begin(true)
	}()
	return p, nil
}
func (p *Pair) begin(unexpected bool) {
	p.once.Do(func() {
		close(p.stopping)
		go func() {
			if unexpected {
				p.err = ErrProcess
			}
			// Stop both concurrently: a hung bridge must not leave product serving.
			var wg sync.WaitGroup
			errs := make(chan error, 2)
			for _, c := range []*Child{p.bridge, p.product} {
				wg.Add(1)
				go func(c *Child) { defer wg.Done(); errs <- c.Stop(context.Background()) }(c)
			}
			wg.Wait()
			close(errs)
			for e := range errs {
				if e != nil {
					p.err = ErrProcess
				}
			}
			<-p.monitorDone
			close(p.done)
		}()
	})
}
func (p *Pair) Close(ctx context.Context) error {
	if ctx == nil {
		return ErrProcess
	}
	p.begin(false)
	select {
	case <-p.done:
		return p.err
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (p *Pair) Done() <-chan struct{} { return p.done }
