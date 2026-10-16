// Privileged fixed-root directory provisioner. Distribution signing and the
// installer elevation prompt remain outside this executable.
package main

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"

	"remoteapp.local/selfhost-poc/installrelease"
)

var embeddedInstallRoot, embeddedTest string

func configuration() (string, bool, error) {
	raw, err := base64.RawURLEncoding.DecodeString(embeddedInstallRoot)
	if err != nil || base64.RawURLEncoding.EncodeToString(raw) != embeddedInstallRoot || len(raw) < 4 || len(raw) > 1024 {
		return "", false, installrelease.ErrRelease
	}
	root := string(raw)
	if !filepath.IsAbs(root) || filepath.Clean(root) != root || filepath.Dir(root) == root {
		return "", false, installrelease.ErrRelease
	}
	test, err := strconv.ParseBool(embeddedTest)
	if err != nil || strconv.FormatBool(test) != embeddedTest {
		return "", false, installrelease.ErrRelease
	}
	return root, test, nil
}

func run() (installrelease.LayoutReceipt, error) {
	if len(os.Args) != 1 {
		return installrelease.LayoutReceipt{}, installrelease.ErrRelease
	}
	root, test, err := configuration()
	if err != nil {
		return installrelease.LayoutReceipt{}, err
	}
	return installrelease.PrepareProtectedLayout(root, test)
}

func main() {
	receipt, err := run()
	if err != nil || json.NewEncoder(os.Stdout).Encode(receipt) != nil {
		os.Exit(1)
	}
}
