package main

import (
	"os"
	"remoteapp.local/selfhost-poc/installeddevice"
	"remoteapp.local/selfhost-poc/privatepipe"
	"strings"
	"testing"
)

func TestUnconfiguredEntryCannotOpenOrOutputPrivateData(t *testing.T) {
	args := os.Args
	os.Args = []string{args[0]}
	defer func() { os.Args = args }()
	embeddedRelease = ""
	if run(func(any) error { t.Fatal("unexpected output"); return nil }) == nil {
		t.Fatal("empty compiled trust")
	}
}

func TestStrictPipeRejectsRuntimeTrustAndKeyInjection(t *testing.T) {
	for _, field := range []string{"Endpoint", "Certificate", "Issuer", "ReleaseRoot", "ServiceProfile", "MinimumEpoch", "AppKey", "VaultExecutable", "CreateIdentity", "Grant", "Profile"} {
		var request installeddevice.Request
		if privatepipe.NewReader(strings.NewReader(`{"Op":"open","`+field+`":"override"}`+"\n")).Read(&request) == nil {
			t.Fatal("accepted injection", field)
		}
	}
}
