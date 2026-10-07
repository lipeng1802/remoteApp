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

func TestNetworkConfigurationRejectsPartialTrust(t *testing.T) {
	for _, c := range []config{{Certificate: "orphan"}, {Issuer: "orphan"}, {Endpoint: "https://127.0.0.1:1234"}, {Endpoint: "http://127.0.0.1:1234", Issuer: "invalid", Certificate: "invalid", GrantID: "grant"}} {
		if _, err := openNetwork(c, nil); err == nil {
			t.Fatal("partial/untrusted network bootstrap accepted")
		}
	}
	if client, err := openNetwork(config{}, nil); err != nil || client != nil {
		t.Fatal("legacy fixture compatibility")
	}
}
