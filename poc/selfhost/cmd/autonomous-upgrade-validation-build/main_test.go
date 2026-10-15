package main

import "testing"

func TestTargetIsExplicit(t *testing.T) {
	if suffix, cgo, arch, err := target("windows"); err != nil || suffix != ".exe" || cgo != "0" || arch != "amd64" {
		t.Fatal("windows target")
	}
	if suffix, cgo, _, err := target("darwin"); err != nil || suffix != "" || cgo != "1" {
		t.Fatal("darwin target")
	}
	if _, _, _, err := target("linux"); err == nil {
		t.Fatal("implicit target accepted")
	}
}
