package main

import (
	"encoding/base64"
	"testing"
)

func TestCompiledMaintenanceAddressRejectsExternalAndNoncanonical(t *testing.T) {
	for _, a := range []string{"example.com:19445", "192.0.2.1:19445", "127.0.0.1:0", "127.0.0.1:443", "127.0.0.1:019445", "[::1]:19445"} {
		if _, e := maintenanceAddress(base64.RawURLEncoding.EncodeToString([]byte(a))); e == nil {
			t.Fatal("bad compiled endpoint", a)
		}
	}
	for _, a := range []string{"127.0.0.1:19445", "127.0.0.1:65535"} {
		got, e := maintenanceAddress(base64.RawURLEncoding.EncodeToString([]byte(a)))
		if e != nil || got != a {
			t.Fatal(got, e)
		}
	}
	if got, e := maintenanceAddress(""); e != nil || got != "127.0.0.1:0" {
		t.Fatal("legacy endpoint changed")
	}
}
