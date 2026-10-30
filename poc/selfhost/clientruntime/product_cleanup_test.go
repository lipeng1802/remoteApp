package clientruntime

import (
	"context"
	"remoteapp.local/selfhost-poc/cleanupwitness"
	"testing"
	"time"
)

func TestActualTLSChildCleanupWitnessRequiresNormalExit(t *testing.T) {
	for _, mode := range []string{"controller", "controller-nonzero"} {
		t.Run(mode, func(t *testing.T) {
			w := &cleanupwitness.Witness{}
			for _, k := range []cleanupwitness.Resource{cleanupwitness.OwnerJournal, cleanupwitness.BindingJournal, cleanupwitness.CommandReader, cleanupwitness.Runner, cleanupwitness.Signer} {
				w.Begin(k)
				w.Complete(k, nil)
			}
			w.Arm(true)
			api, issuer := productFixture(t)
			cfg := api.c
			cfg.Server = false
			p := Product{Spec: productSpec(t, mode), ControllerKey: testHMAC}
			child, e := p.start(cleanupwitness.Context(context.Background(), w), api, issuer, cfg, time.Now, 32123)
			if e != nil {
				t.Fatal(e)
			}
			e = child.Stop(context.Background())
			if (e == nil) != (mode == "controller") {
				t.Fatal("unexpected child result", e)
			}
			w.Seal()
			if w.Verified() != (mode == "controller") {
				t.Fatal("actual nonzero/normal cleanup conflated")
			}
		})
	}
}
