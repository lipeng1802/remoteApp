package invite

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDevicePublicKeySignedBindingAndLegacyUpgrade(t *testing.T) {
	f := newFixture(t)
	pub, e := f.s.DevicePublicKey(f.target.DeviceID)
	if e != nil {
		t.Fatal(e)
	}
	if !bytes.Equal(pub, f.owner[32:]) {
		t.Fatal("wrong bound key")
	}
	pub[0] ^= 1
	again, _ := f.s.DevicePublicKey(f.target.DeviceID)
	if bytes.Equal(pub, again) {
		t.Fatal("returned mutable state")
	}
	f.restart(t)
	if _, e = f.s.DevicePublicKey(f.target.DeviceID); e != nil {
		t.Fatal("lost on restart")
	}
	f.s.mu.Lock()
	d := f.s.state.Devices[f.target.DeviceID]
	d.PublicKey = ""
	f.s.state.Devices[f.target.DeviceID] = d
	if f.s.save(f.s.state) != nil {
		t.Fatal("save")
	}
	f.s.mu.Unlock()
	f.restart(t)
	if _, e = f.s.DevicePublicKey(f.target.DeviceID); e == nil {
		t.Fatal("legacy key inferred")
	}
	if _, e = f.s.Execute(f.envelope(f.owner, Body{Action: "register"})); e != nil {
		t.Fatal(e)
	}
	if _, e = f.s.DevicePublicKey(f.target.DeviceID); e != nil {
		t.Fatal("explicit signed register did not upgrade")
	}
	if _, e = f.s.DevicePublicKey(f.controller.DeviceID); e != nil {
		t.Fatal(e)
	}
	if _, e = f.s.DevicePublicKey("unknown"); e == nil {
		t.Fatal("unknown accepted")
	}
}
func TestDevicePublicKeyTamperOnDiskFailsClosed(t *testing.T) {
	f := newFixture(t)
	root := f.s.root
	f.s.Close()
	b, e := os.ReadFile(filepath.Join(root, "state.json"))
	if e != nil {
		t.Fatal(e)
	}
	var st snapshot
	if json.Unmarshal(b, &st) != nil {
		t.Fatal("decode")
	}
	d := st.Devices[f.target.DeviceID]
	d.PublicKey = string(bytes.Repeat([]byte("0"), 64))
	st.Devices[f.target.DeviceID] = d
	b, _ = json.Marshal(st)
	if os.WriteFile(filepath.Join(root, "state.json"), b, 0600) != nil {
		t.Fatal("write")
	}
	if s, e := Open(root, f.s.clock); e == nil {
		s.Close()
		t.Fatal("wrong key-device binding loaded")
	}
}
