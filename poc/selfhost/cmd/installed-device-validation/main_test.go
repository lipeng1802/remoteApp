package main

import (
	"os"
	"testing"
)

func TestEmptyCompiledValidatorCannotInitializeNativeState(t *testing.T) {
	args := os.Args
	os.Args = []string{args[0]}
	defer func() { os.Args = args }()
	if run() == nil {
		t.Fatal("unconfigured validator accepted")
	}
}
