package main

import (
	"testing"

	"remoteapp.local/selfhost-poc/installeddevice"
	"remoteapp.local/selfhost-poc/invite"
)

func TestRelayDesktopManagementBoundary(t *testing.T) {
	for _, tc := range []struct {
		request           installeddevice.Request
		fixed, view, ctrl bool
	}{
		{installeddevice.Request{Op: "desktop_begin"}, false, true, true},
		{installeddevice.Request{Op: "desktop_status"}, false, true, true},
		{installeddevice.Request{Op: "desktop_state"}, false, true, true},
		{installeddevice.Request{Op: "desktop_arm_stop"}, false, true, true},
		{installeddevice.Request{Op: "desktop_begin", VaultScope: "selector"}, false, false, false},
		{installeddevice.Request{Op: "product_count"}, true, false, false},
		{installeddevice.Request{Op: "product_stage"}, true, true, true},
		{installeddevice.Request{Op: "revoke_session"}, true, true, true},
		{installeddevice.Request{Op: "quiesce_session"}, true, true, true},
		{installeddevice.Request{Op: "device", Device: invite.Body{Action: "invite", Permission: invite.DesktopControl}}, false, false, true},
	} {
		if validRequest(tc.request, "") != tc.fixed ||
			validRequest(tc.request, invite.DesktopView) != tc.view ||
			validRequest(tc.request, invite.DesktopControl) != tc.ctrl ||
			validRequest(tc.request, "unknown") {
			t.Errorf("incorrect routing for %s/%s", tc.request.Op, tc.request.Device.Action)
		}
	}
}
