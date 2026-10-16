// TEST-only native acceptance for the compiled fixed-root provisioner.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"

	"remoteapp.local/selfhost-poc/installrelease"
)

var embeddedInstallRoot, embeddedTest, embeddedProvisionerDigest string

func decodeReceipt(raw []byte) (installrelease.LayoutReceipt, error) {
	if len(raw) < 2 || len(raw) > 4096 || raw[len(raw)-1] != '\n' {
		return installrelease.LayoutReceipt{}, installrelease.ErrRelease
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var receipt installrelease.LayoutReceipt
	var extra struct{}
	if decoder.Decode(&receipt) != nil || decoder.Decode(&extra) != io.EOF {
		return installrelease.LayoutReceipt{}, installrelease.ErrRelease
	}
	canonical, _ := json.Marshal(receipt)
	if !bytes.Equal(raw, append(canonical, '\n')) {
		return installrelease.LayoutReceipt{}, installrelease.ErrRelease
	}
	return receipt, nil
}

func readPinned(path string) error {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() < 1 || info.Size() > 100<<20 {
		return installrelease.ErrRelease
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return installrelease.ErrRelease
	}
	digest := sha256.Sum256(raw)
	if hex.EncodeToString(digest[:]) != embeddedProvisionerDigest {
		return installrelease.ErrRelease
	}
	return nil
}

func configuration() (string, string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(embeddedInstallRoot)
	if err != nil || base64.RawURLEncoding.EncodeToString(raw) != embeddedInstallRoot || embeddedTest != "true" || len(embeddedProvisionerDigest) != 64 {
		return "", "", installrelease.ErrRelease
	}
	root := string(raw)
	if !filepath.IsAbs(root) || filepath.Clean(root) != root || filepath.Dir(root) == root {
		return "", "", installrelease.ErrRelease
	}
	suffix := ""
	if filepath.Ext(os.Args[0]) == ".exe" {
		suffix = ".exe"
	}
	return root, filepath.Join(filepath.Dir(os.Args[0]), "install-root-provisioner"+suffix), nil
}

func invoke(path string) (installrelease.LayoutReceipt, error) {
	output, err := exec.Command(path).Output()
	if err != nil {
		return installrelease.LayoutReceipt{}, installrelease.ErrRelease
	}
	return decodeReceipt(output)
}

func run() error {
	if len(os.Args) != 1 {
		return installrelease.ErrRelease
	}
	root, provisioner, err := configuration()
	if err != nil || readPinned(provisioner) != nil {
		return installrelease.ErrRelease
	}
	first, err := invoke(provisioner)
	if err != nil || installrelease.ValidateLayoutReceipt(first, root, true) != nil {
		return installrelease.ErrRelease
	}
	second, err := invoke(provisioner)
	if err != nil || !reflect.DeepEqual(first, second) || installrelease.ValidateLayoutReceipt(second, root, true) != nil {
		return installrelease.ErrRelease
	}
	return installrelease.CleanupProtectedTestLayout(second, root)
}

func main() {
	if err := run(); err != nil {
		fmt.Println("VALIDATION FAILED: protected install root provisioning")
		os.Exit(1)
	}
	fmt.Println("VALIDATION PASSED: compiled fixed-root TEST provisioner + protected owner/mode recipe + stable reopen receipt + exact empty-layout cleanup. NOT elevated production root, product payload, code signing, DMG/Inno or Windows acceptance.")
}
