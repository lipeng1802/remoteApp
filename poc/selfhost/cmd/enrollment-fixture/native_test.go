package main

import (
	"strings"
	"testing"
)

func TestNativePublicIdentityBootstrap(t *testing.T) {
	a, b := strings.Repeat("ab", 32), strings.Repeat("cd", 32)
	if _, _, e := nativePublics(a, b); e != nil {
		t.Fatal(e)
	}
	for _, pair := range [][2]string{{a, a}, {strings.ToUpper(a), b}, {a, strings.Repeat("cd", 64)}, {"", b}, {a, "invalid"}} {
		if _, _, e := nativePublics(pair[0], pair[1]); e == nil {
			t.Fatal("ambiguous or private bootstrap accepted")
		}
	}
}
