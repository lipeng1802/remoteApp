//go:build darwin || linux

package trustprofile

import (
	"errors"
	"os/exec"
	"remoteapp.local/selfhost-poc/privatepipe"
	"testing"
)

// A deliberately silent disposable process exercises the existing timeout
// failure path. Its forced abort is NOT accepted as a successful native join.
func TestSilentJournalRPCReportsUncertainTimeout(t *testing.T) {
	c := exec.Command("/bin/sleep", "20")
	in, e := c.StdinPipe()
	if e != nil {
		t.Fatal(e)
	}
	out, e := c.StdoutPipe()
	if e != nil {
		t.Fatal(e)
	}
	if e = c.Start(); e != nil {
		t.Fatal(e)
	}
	j := &DesktopJournal{cmd: c, in: in, out: out, reader: privatepipe.NewBoundedReader(out, 16384), wait: make(chan error, 1)}
	_, e = j.rpc(map[string]string{"Op": "load"})
	if !errors.Is(e, ErrTrust) || !errors.Is(e, ErrNativeHelperTimeout) || NativeFailureReason(e) != "native_helper_timeout" || !j.closed {
		t.Fatal("timeout became normal result", e)
	}
}
