package supervisedcontrol

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/installeddevice"
	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/privatepipe"
)

func TestDesktopPrivateRoutingScopeLifecycleAndActualHandlerJoin(t *testing.T) {
	for _, scope := range []invite.DesktopPermission{invite.DesktopView, invite.DesktopControl} {
		ln, e := net.Listen("tcp4", "127.0.0.1:0")
		if e != nil {
			t.Fatal(e)
		}
		ctx, cancel := context.WithCancel(context.Background())
		b := &fixture{}
		done := make(chan struct{})
		go func() { defer close(done); ServeDesktop(ctx, ln, b, scope) }()
		t.Cleanup(func() {
			cancel()
			ln.Close()
			select {
			case <-done:
			case <-time.After(3 * time.Second):
				t.Error("desktop handlers not joined")
			}
		})
		call := func(r installeddevice.Request, want string, admitted bool) {
			before := b.calls.Load()
			c, e := net.Dial("tcp4", ln.Addr().String())
			if e != nil {
				t.Fatal(e)
			}
			defer c.Close()
			c.SetDeadline(time.Now().Add(3 * time.Second))
			if json.NewEncoder(c).Encode(r) != nil {
				t.Fatal("request write")
			}
			var raw json.RawMessage
			if privatepipe.NewReader(c).Read(&raw) != nil {
				t.Fatal("reply missing")
			}
			var status struct {
				Status string `json:"status"`
			}
			if json.Unmarshal(raw, &status) != nil || status.Status != want {
				t.Fatal("wrong private route", status.Status)
			}
			wantCalls := before
			if admitted {
				wantCalls++
			}
			if b.calls.Load() != wantCalls {
				t.Fatal("rejected command reached desktop owner")
			}
		}
		call(installeddevice.Request{Op: "device", Device: invite.Body{Action: "invite", Permission: scope}}, "device_result", true)
		call(installeddevice.Request{Op: "device", Device: invite.Body{Action: "approve", Permission: scope, RequestID: "request", Allow: true}}, "device_result", true)
		call(installeddevice.Request{Op: "device", Device: invite.Body{Action: "invite"}}, "request_rejected", false)
		other := invite.DesktopView
		if scope == other {
			other = invite.DesktopControl
		}
		call(installeddevice.Request{Op: "device", Device: invite.Body{Action: "invite", Permission: other}}, "request_rejected", false)
		call(installeddevice.Request{Op: "device", VaultScope: "other", Device: invite.Body{Action: "invite", Permission: scope}}, "request_rejected", false)
		call(installeddevice.Request{Op: "product_count"}, "request_rejected", false)
		for _, op := range []string{"open", "stop", "stop_fault", "stop_local_fault", "maintenance_prepare"} {
			call(installeddevice.Request{Op: op}, "request_rejected", false)
		}
		for _, op := range []string{"desktop_begin", "desktop_status", "desktop_state", "desktop_arm_stop"} {
			call(installeddevice.Request{Op: op}, "device_result", true)
			call(installeddevice.Request{Op: op, Device: invite.Body{Allow: true}}, "request_rejected", false)
		}
		for _, op := range []string{"quiesce_session", "revoke_session", "start_controller", "preprovision_terminal"} {
			call(installeddevice.Request{Op: op}, "device_result", true)
		}
		stalled, e := net.Dial("tcp4", ln.Addr().String())
		if e != nil {
			t.Fatal(e)
		}
		cancel()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Fatal("stalled private reader not joined")
		}
		stalled.Close()
	}
}
