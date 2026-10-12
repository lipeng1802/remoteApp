package devicevalidation

import (
	"encoding/json"
	"os"
	"os/exec"
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/installeddevice"
	"remoteapp.local/selfhost-poc/privatepipe"
)

func TestDedicatedStopInputJoinFixture(t *testing.T) {
	if os.Getenv("REMOTEAPP_DEVICE_STOP_FIXTURE") != "1" {
		return
	}
	commands, done := make(chan installeddevice.Request), make(chan struct{})
	go func() {
		defer close(done)
		reader := privatepipe.NewReader(os.Stdin)
		for {
			var r installeddevice.Request
			if reader.Read(&r) != nil {
				return
			}
			commands <- r
		}
	}()
	json.NewEncoder(os.Stdout).Encode(map[string]string{"status": "ready"})
	if r := <-commands; r.Op != "stop" {
		os.Exit(1)
	}
	os.Stdin.Close()
	<-done
	json.NewEncoder(os.Stdout).Encode(map[string]string{"status": "closed"})
	os.Exit(0)
}

func TestStopHalfCloseStillRequiresClosedAndActualZeroExit(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	c := &child{cmd: exec.Command(exe, "-test.run=^TestDedicatedStopInputJoinFixture$")}
	c.cmd.Env = append(os.Environ(), "REMOTEAPP_DEVICE_STOP_FIXTURE=1")
	c.in, err = c.cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	c.out, err = c.cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = c.cmd.Start(); err != nil {
		t.Fatal(err)
	}
	c.encode = json.NewEncoder(c.in)
	c.read = privatepipe.NewReader(c.out)
	c.timer = time.AfterFunc(5*time.Second, func() { c.expired.Store(true); c.in.Close() })
	defer c.cleanup()
	if _, err = c.receipt("ready"); err != nil {
		t.Fatal(err)
	}
	if c.stop() != nil {
		t.Fatal("dedicated input half-close did not join child")
	}
	if !c.waited || c.expired.Load() {
		t.Fatal("timeout is not actual exit evidence")
	}
}
