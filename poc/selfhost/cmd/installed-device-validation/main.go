package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"remoteapp.local/selfhost-poc/devicevalidation"
	"remoteapp.local/selfhost-poc/trustinstall"
)

var embeddedRelease, embeddedSignerDigest, embeddedClientDigest, embeddedFixtureTLS string

func main() {
	if run() != nil {
		fmt.Println("VALIDATION FAILED: installed device native acceptance; preserve test evidence")
		os.Exit(1)
	}
	fmt.Println("VALIDATION PASSED: actual compiled device client + native trust lease + OS signature direct local HTTPS + stable restart + joined test cleanup. NOT public service, Headscale, product TLS/GUI or signed installer acceptance.")
}

func run() error {
	if len(os.Args) != 1 {
		return devicevalidation.ErrValidation
	}
	release, err := trustinstall.DecodeCompiled(embeddedRelease)
	if err != nil {
		return devicevalidation.ErrValidation
	}
	raw, err := base64.StdEncoding.DecodeString(embeddedFixtureTLS)
	if err != nil || len(raw) > 16384 {
		return devicevalidation.ErrValidation
	}
	var fixture devicevalidation.TLSFixture
	if json.Unmarshal(raw, &fixture) != nil {
		return devicevalidation.ErrValidation
	}
	exe, err := os.Executable()
	if err != nil {
		return devicevalidation.ErrValidation
	}
	return devicevalidation.Run(release, fixture, embeddedSignerDigest, embeddedClientDigest, filepath.Dir(exe), func(label string) { fmt.Println("PASS " + label) })
}
