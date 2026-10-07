package main

import (
	"context"
	"testing"

	"remoteapp.local/selfhost-poc/headscale"
)

func TestPolicyZeroRequiresExplicitExactEmptyACL(t *testing.T) {
	for _, c := range []struct {
		raw string
		ok  bool
	}{
		{`{"acls":[]}`, true},
		{`{}`, false},
		{`{"acls":null}`, false},
		{`{"acls":[{"action":"accept","src":["*"],"dst":["*:*"]}]}`, false},
		{`{"acls":[],"extra":true}`, false},
		{`malformed`, false},
	} {
		d := &headscale.Driver{Run: func(context.Context, ...string) ([]byte, error) { return []byte(c.raw), nil }}
		if policyZero(d) != c.ok {
			t.Fatal("zero ACL assertion mismatch")
		}
	}
}
