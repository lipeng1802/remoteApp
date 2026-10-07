package main

import "testing"

func TestVaultConfigurationRejectsPrivateKeyAndMissingPin(t *testing.T) {
	for _, c := range []config{
		{VaultExecutable: "/not-launched", AppKey: "private", ExpectedPublic: "pin"},
		{VaultExecutable: "/not-launched"}, {VaultScope: "orphan"}, {ExpectedPublic: "orphan"},
	} {
		if _, _, err := openSigner(c); err == nil {
			t.Fatal("ambiguous identity accepted")
		}
	}
}
