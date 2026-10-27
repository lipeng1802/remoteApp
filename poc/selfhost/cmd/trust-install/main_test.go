package main

import (
	"errors"
	"testing"

	"remoteapp.local/selfhost-poc/trustprofile"
)

type exactOwner struct {
	profile trustprofile.Profile
	err     error
}

func (o exactOwner) Inspect() (trustprofile.Profile, error) { return o.profile, o.err }

func TestReconcileExactAcceptsOnlyReadBackOfUncertainCommit(t *testing.T) {
	expected := trustprofile.Profile{Claims: trustprofile.Claims{Version: 1, Epoch: 2}, Signature: "expected"}
	uncertain := errors.New("uncertain close")
	operation := func() (trustprofile.Profile, error) { return trustprofile.Profile{}, uncertain }

	if got, err := reconcileExact(exactOwner{profile: expected}, expected, operation); err != nil || got != expected {
		t.Fatal("exact durable readback rejected", err)
	}
	different := expected
	different.Claims.Epoch = 1
	if _, err := reconcileExact(exactOwner{profile: different}, expected, operation); !errors.Is(err, uncertain) {
		t.Fatal("different durable state accepted", err)
	}
	if _, err := reconcileExact(exactOwner{err: trustprofile.ErrTrust}, expected, operation); !errors.Is(err, uncertain) {
		t.Fatal("failed readback hid original error", err)
	}
}
