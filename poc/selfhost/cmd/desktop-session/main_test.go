package main

import (
	"context"
	"remoteapp.local/selfhost-poc/clientruntime"
	"testing"
)

func TestDesktopStartRejectsRoleAndIdentityBeforeResources(t *testing.T) {
	b := &backend{expected: "desktop-identity"}
	for _, c := range []clientruntime.Config{{}, {Server: true, Target: "other"}, {Server: false, Target: "desktop-identity"}} {
		if b.Start(context.Background(), c, func(any) error { t.Fatal("unexpected readiness"); return nil }) == nil {
			t.Fatal("unbound role accepted")
		}
	}
}
func TestDesktopEntryRejectsUncompiledRelease(t *testing.T) {
	if run() == nil {
		t.Fatal("uncompiled desktop entry accepted")
	}
}
