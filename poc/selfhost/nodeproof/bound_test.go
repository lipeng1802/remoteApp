package nodeproof

import (
	"remoteapp.local/selfhost-poc/invite"
	"strings"
	"testing"
)

func TestInstalledProofBoundToOneCanonicalGrant(t *testing.T) {
	g := strings.Repeat("ab", 16)
	if !boundGrant(g, g) {
		t.Fatal("valid grant refused")
	}
	for _, p := range []struct{ expected, got string }{{g, strings.Repeat("cd", 16)}, {strings.ToUpper(g), strings.ToUpper(g)}, {"", ""}, {strings.Repeat("a", 31), strings.Repeat("a", 31)}} {
		if boundGrant(p.expected, p.got) {
			t.Fatal("cross-grant or noncanonical challenge accepted")
		}
	}
}

func TestPermittedProofBindsTicketGrantDeviceAndNode(t *testing.T) {
	c := invite.VerifierClaims{Ticket: "ticket", GrantID: "grant", Device: "device", NodeKey: "node"}
	v := challenge{Ticket: c.Ticket, Grant: c.GrantID, Device: c.Device, Node: c.NodeKey}
	if !permittedChallenge(c, v) {
		t.Fatal("expected challenge rejected")
	}
	for _, edit := range []func(*challenge){func(v *challenge) { v.Ticket = "other" }, func(v *challenge) { v.Grant = "other" }, func(v *challenge) { v.Device = "other" }, func(v *challenge) { v.Node = "other" }} {
		bad := v
		edit(&bad)
		if permittedChallenge(c, bad) {
			t.Fatal("cross-session challenge accepted")
		}
	}
}
