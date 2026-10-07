package main

import "testing"

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
