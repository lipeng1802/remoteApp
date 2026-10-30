package cleanupwitness

import (
	"context"
	"errors"
	"sync"
	"testing"
)

func ready(w *Witness) {
	for _, r := range []Resource{OwnerJournal, BindingJournal, CommandReader, Runner, Signer} {
		w.Begin(r)
		w.Complete(r, nil)
	}
	w.Arm(true)
}
func TestOnlySealedCompleteSingleLifetimeCanAttest(t *testing.T) {
	for _, mode := range []string{"complete", "pending", "cleanup-error", "duplicate-start", "duplicate-complete", "unarmed", "production", "late"} {
		t.Run(mode, func(t *testing.T) {
			w := &Witness{}
			ready(w)
			switch mode {
			case "pending":
				w.Begin(Node)
			case "cleanup-error":
				w.Begin(Node)
				w.Complete(Node, errors.New("private"))
			case "duplicate-start":
				w.Begin(Runner)
			case "duplicate-complete":
				w.Complete(Runner, nil)
			case "unarmed":
				w = &Witness{}
			case "production":
				w.Arm(false)
			case "late":
				w.Begin(TLSProduct)
			}
			if w.Verified() {
				t.Fatal("unsealed proof")
			}
			w.Seal()
			if mode == "late" {
				w.Complete(TLSProduct, nil)
			}
			if w.Verified() != (mode == "complete") {
				t.Fatal("pending/failed/late proof accepted")
			}
		})
	}
}
func TestContextAndConcurrentIndependentBarriers(t *testing.T) {
	w := &Witness{}
	ctx := Context(context.Background(), w)
	if From(ctx) != w || From(context.Background()) != nil {
		t.Fatal("untrusted witness wiring")
	}
	var wg sync.WaitGroup
	for r := Resource(0); r < resourceCount; r++ {
		wg.Add(1)
		go func(r Resource) { defer wg.Done(); w.Begin(r); w.Complete(r, nil) }(r)
	}
	wg.Wait()
	w.Arm(true)
	w.Seal()
	if !w.Verified() {
		t.Fatal("actual concurrent barrier completion lost")
	}
}
