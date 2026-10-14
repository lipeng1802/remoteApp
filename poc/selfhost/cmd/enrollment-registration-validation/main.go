package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"remoteapp.local/selfhost-poc/devicevalidation"
	"remoteapp.local/selfhost-poc/registrationvalidation"
	"remoteapp.local/selfhost-poc/trustinstall"
)

var embeddedRelease, embeddedSignerDigest, embeddedFixtureTLS string

func main() {
	if run() != nil {
		fmt.Println("VALIDATION FAILED: native registration combination; preserve TEST evidence")
		os.Exit(1)
	}
	fmt.Println("VALIDATION PASSED: native profile/node leases + OS signed grant clock/HTTPS enroll + internal fixture credential consumption + durable result/restart refusal + joined cleanup. NOT real Headscale Up, device-client IPC, production signing/GUI or installer acceptance.")
}
func run() error {
	if len(os.Args) != 1 {
		return registrationvalidation.ErrValidation
	}
	r, e := trustinstall.DecodeCompiled(embeddedRelease)
	if e != nil {
		return e
	}
	b, e := base64.StdEncoding.DecodeString(embeddedFixtureTLS)
	if e != nil || len(b) > 16384 {
		return registrationvalidation.ErrValidation
	}
	var f devicevalidation.TLSFixture
	if json.Unmarshal(b, &f) != nil {
		return registrationvalidation.ErrValidation
	}
	exe, e := os.Executable()
	if e != nil {
		return e
	}
	return registrationvalidation.Run(r, f, embeddedSignerDigest, filepath.Dir(exe), func(s string) { fmt.Println("PASS " + s) })
}
