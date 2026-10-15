package main

import "testing"

func TestParseEpochStrictSafeInteger(t *testing.T) {
	for input, want := range map[string]uint64{"1": 1, "42": 42, "9007199254740991": 1<<53 - 1} {
		got, err := parseEpoch(input)
		if err != nil || got != want {
			t.Fatalf("parseEpoch(%q)=%d,%v", input, got, err)
		}
	}
	for _, input := range []string{"", "0", "01", "-1", "+1", "1.0", "9007199254740992", "18446744073709551615"} {
		if _, err := parseEpoch(input); err == nil {
			t.Fatal("invalid epoch accepted", input)
		}
	}
}
