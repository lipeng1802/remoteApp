package main

import "testing"

func TestNetworkExposureRequiresOneExactOperatorMode(t *testing.T) {
	base := make([]string, 8)
	for _, tc := range []struct {
		flag            string
		private, public bool
	}{{"", false, false}, {"--private-network", true, false}, {"--public-network", false, true}} {
		args := append([]string(nil), base...)
		if tc.flag != "" {
			args = append(args, tc.flag)
		}
		private, public, err := networkMode(args)
		if err != nil || private != tc.private || public != tc.public {
			t.Fatal("mode mismatch")
		}
	}
	for _, args := range [][]string{nil, append(base, "--public"), append(base, "--PUBLIC-network"), append(base, "--private-network", "--public-network")} {
		if _, _, err := networkMode(args); err == nil {
			t.Fatal("ambiguous exposure accepted")
		}
	}
}
