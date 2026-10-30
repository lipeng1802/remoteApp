package installeddevice

import (
	"context"
	"errors"
	"net"
	"remoteapp.local/selfhost-poc/cleanupwitness"
	"testing"
)

type failedListenerClose struct{ net.Listener }

func (l failedListenerClose) Close() error {
	l.Listener.Close()
	return errors.New("close receipt unknown")
}
func TestTransportCleanupWitnessRejectsUnknownCloseAndNeverHeals(t *testing.T) {
	for _, bad := range []bool{false, true} {
		w := &cleanupwitness.Witness{}
		for _, k := range []cleanupwitness.Resource{cleanupwitness.OwnerJournal, cleanupwitness.BindingJournal, cleanupwitness.CommandReader, cleanupwitness.Runner, cleanupwitness.Signer} {
			w.Begin(k)
			w.Complete(k, nil)
		}
		w.Arm(true)
		l, e := trackedSessionListener(cleanupwitness.Context(context.Background(), w), func() (net.Listener, error) {
			raw, e := net.Listen("tcp4", "127.0.0.1:0")
			if bad {
				return failedListenerClose{raw}, e
			}
			return raw, e
		})
		if e != nil {
			t.Fatal(e)
		}
		first := l.Close()
		second := l.Close()
		if (first != nil) != bad || (second != nil) != bad {
			t.Fatal("close result healed")
		}
		w.Seal()
		if w.Verified() == bad {
			t.Fatal("unknown transport close attested")
		}
	}
}
