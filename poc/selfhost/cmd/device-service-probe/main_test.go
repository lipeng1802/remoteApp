package main

import (
	"os"
	"strings"
	"testing"
)

func TestProbeRejectsUntrustedInputs(t *testing.T) {
	saved := os.Args
	defer func() { os.Args = saved }()
	for _, args := range [][]string{
		{"probe"},
		{"probe", "missing", "bad", "service", "127.0.0.1:8444"},
		{"probe", "missing", strings.Repeat("00", 32), "service", "182.92.117.114:8444"},
	} {
		os.Args = args
		if run() == nil {
			t.Fatal("accepted untrusted probe inputs")
		}
	}
}
