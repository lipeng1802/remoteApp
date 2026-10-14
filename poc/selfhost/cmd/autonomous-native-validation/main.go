package main

import (
	"fmt"
	"os"
	"path/filepath"
	"remoteapp.local/selfhost-poc/autonomousvalidation"
	"remoteapp.local/selfhost-poc/trustinstall"
)

var embeddedRelease, embeddedSignerDigest string

func main() {
	if len(os.Args) != 1 {
		fmt.Println("FAIL autonomous_validation_arguments")
		os.Exit(1)
	}
	exe, e := os.Executable()
	if e != nil {
		os.Exit(1)
	}
	r, e := trustinstall.DecodeCompiled(embeddedRelease)
	if e != nil || autonomousvalidation.Run(r, embeddedSignerDigest, filepath.Dir(exe), func(label string) { fmt.Println("CHECK " + label) }) != nil {
		fmt.Println("VALIDATION FAILED: native signed HTTPS enrollment/binding; retain scoped evidence")
		os.Exit(1)
	}
	fmt.Println("VALIDATION PASSED: actual OS signed HTTPS approval/enroll + native real SDK + issuer verifier permit + real backend bind + joined native TEST cleanup. NOT continuous watch, payload TLS/GUI, other-platform or installer acceptance.")
}
