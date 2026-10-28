package main

import (
	"strings"
	"testing"
)

func TestExactSigningDomains(t *testing.T) {
	for _, raw := range []string{"", "arbitrary", "remoteapp-node-possession-v2\n{}", "remoteapp-session-product-tls-controller-v1x\n{}", "remoteapp-network-api-v1\nGET /v1/network\n{}"} {
		if allowedDomain(raw) {
			t.Fatal("unauthorized signing domain")
		}
	}
	for _, raw := range []string{"remoteapp-provision-intent-v1\n{}", "remoteapp-node-possession-v1\n{}", "remoteapp-session-product-tls-target-v1\n{}"} {
		if !allowedDomain(raw) {
			t.Fatal("known domain rejected")
		}
	}
}

func TestMaintenanceDomainsRequireExactDigest(t *testing.T) {
	for _, phase := range []string{"challenge", "authenticate", "ready", "finish", "closed"} {
		prefix := "remoteapp-local-maintenance-" + phase + "-v1\n"
		if !allowedDomain(prefix + strings.Repeat("a", 64)) {
			t.Fatal("maintenance domain denied")
		}
		for _, bad := range []string{"", "{}", strings.Repeat("a", 63), strings.Repeat("A", 64), strings.Repeat("a", 64) + "\n"} {
			if allowedDomain(prefix + bad) {
				t.Fatal("noncanonical maintenance transcript accepted")
			}
		}
	}
	if allowedDomain("remoteapp-local-maintenance-reset-v1\n" + strings.Repeat("a", 64)) {
		t.Fatal("reset signing enabled")
	}
}
