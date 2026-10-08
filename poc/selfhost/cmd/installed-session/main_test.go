package main

import "testing"

func TestUncompiledEntryFailsClosed(t *testing.T) {
	if embeddedRelease != "" || embeddedSignerDigest != "" {
		t.Fatal("test must use ordinary empty build")
	}
	outputs := 0
	if e := run(func(any) { outputs++ }); e == nil || outputs != 0 {
		t.Fatal("uncompiled client admitted")
	}
}
