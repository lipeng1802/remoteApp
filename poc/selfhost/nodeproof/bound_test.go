package nodeproof

import (
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
