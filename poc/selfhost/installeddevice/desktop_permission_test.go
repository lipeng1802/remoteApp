package installeddevice

import (
	"testing"

	"remoteapp.local/selfhost-poc/invite"
)

func TestDesktopScopeCannotUpgradeLegacyBusinessIPC(t *testing.T) {
	for _, action := range []string{"register", "invite", "claim", "incoming", "approve", "status", "revoke"} {
		for _, scope := range []invite.DesktopPermission{invite.DesktopView, invite.DesktopControl, "fixed-payload", "unknown"} {
			r := Request{Op: "device", Device: invite.Body{Action: action, Permission: scope}}
			if validCommand(r) || ValidSupervisedCommand(r) {
				t.Fatalf("legacy IPC accepted nonempty permission for %s", action)
			}
		}
	}
}
