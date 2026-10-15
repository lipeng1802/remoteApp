// Runs a compiled TEST-only native autonomous installation upgrade lifecycle.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"remoteapp.local/selfhost-poc/privatepipe"
)

type reply struct {
	Status          string `json:"status"`
	PublicKey       string `json:"public_key"`
	EnrollmentState string `json:"enrollment_state"`
	Epoch           uint64 `json:"epoch"`
	Stage           string `json:"stage"`
	Reason          string `json:"reason"`
}

func command(path string, input []byte, args ...string) (reply, error) {
	cmd := exec.Command(path, args...)
	if input != nil {
		cmd.Stdin = bytes.NewReader(input)
	}
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	if err := cmd.Start(); err != nil {
		return reply{}, err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		var value reply
		if json.Unmarshal(bytes.TrimSpace(output.Bytes()), &value) != nil {
			return reply{}, fmt.Errorf("invalid reply")
		}
		return value, err
	case <-time.After(20 * time.Second):
		_ = cmd.Process.Kill()
		<-done
		return reply{}, fmt.Errorf("timeout")
	}
}

func read(path string, limit int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || !st.Mode().IsRegular() || st.Size() < 1 || st.Size() > limit {
		return nil, fmt.Errorf("invalid fixture")
	}
	raw, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil || int64(len(raw)) > limit {
		return nil, fmt.Errorf("invalid fixture")
	}
	return raw, nil
}

func product(path string) error {
	cmd := exec.Command(path)
	cmd.Stdin = bytes.NewBufferString("{\"Op\":\"open\"}\n{\"Op\":\"stop\"}\n")
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			return err
		}
	case <-time.After(20 * time.Second):
		_ = cmd.Process.Kill()
		<-done
		return fmt.Errorf("product timeout")
	}
	reader := privatepipe.NewBoundedReader(bytes.NewReader(output.Bytes()), 4096)
	var ready, closed reply
	var extra any
	if reader.Read(&ready) != nil || ready.Status != "ready" || reader.Read(&closed) != nil || closed.Status != "closed" || reader.Read(&extra) != io.EOF {
		return fmt.Errorf("invalid product lifecycle")
	}
	return nil
}

func run() (result error) {
	if len(os.Args) != 1 {
		return fmt.Errorf("no arguments permitted")
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	dir := filepath.Dir(exe)
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	manifest, err := read(filepath.Join(dir, "SHA256.json"), 8192)
	if err != nil {
		return err
	}
	var hashes map[string]string
	if len(manifest) > 8192 || json.Unmarshal(manifest, &hashes) != nil {
		return fmt.Errorf("invalid manifest")
	}
	expected := []string{
		"trust-journal" + suffix, "identity-vault" + suffix, "tls-product" + suffix,
		"trust-install-v1" + suffix, "trust-install-v2" + suffix,
		"autonomous-enrollment-install-v1" + suffix, "autonomous-enrollment-install-v2" + suffix,
		"installed-autonomous-product-v1" + suffix, "installed-autonomous-product-v2" + suffix,
		"autonomous-upgrade-validation" + suffix, "old-profile.json",
	}
	if len(hashes) != len(expected) {
		return fmt.Errorf("incomplete manifest")
	}
	for _, name := range expected {
		want, ok := hashes[name]
		if !ok {
			return fmt.Errorf("missing fixture hash")
		}
		raw, err := read(filepath.Join(dir, name), 100<<20)
		h := sha256.Sum256(raw)
		if err != nil || hex.EncodeToString(h[:]) != want {
			return fmt.Errorf("fixture hash mismatch")
		}
	}
	tool := func(name string) string { return filepath.Join(dir, name+suffix) }
	trustCreated, autonomousCreated, reservationCreated := false, false, false
	defer func() {
		if reservationCreated {
			if value, err := command(tool("autonomous-enrollment-install-v2"), nil, "cleanup-reservation-test"); err != nil || value.Status != "cleaned" {
				if result == nil {
					result = fmt.Errorf("reservation cleanup")
				}
			}
		}
		if autonomousCreated {
			if value, err := command(tool("autonomous-enrollment-install-v2"), nil, "cleanup-test"); err != nil || value.Status != "cleaned" {
				if result == nil {
					result = fmt.Errorf("autonomous cleanup")
				}
			}
		}
		if trustCreated {
			if value, err := command(tool("trust-install-v1"), nil, "cleanup-test"); err != nil || value.Status != "usable" {
				if result == nil {
					result = fmt.Errorf("trust cleanup")
				}
			}
		}
	}()
	positive := func(name string, epoch uint64, args ...string) (reply, error) {
		value, err := command(tool(name), nil, args...)
		if err != nil || value.Status != "usable" || (epoch != 0 && value.Epoch != epoch) {
			return value, fmt.Errorf("positive command rejected")
		}
		return value, nil
	}
	negative := func(name string, args ...string) error {
		value, err := command(tool(name), nil, args...)
		if err == nil || value.Status != "failed" {
			return fmt.Errorf("negative command accepted")
		}
		return nil
	}
	if _, err = positive("trust-install-v1", 1, "initialize"); err != nil {
		return err
	}
	trustCreated = true
	if value, statusErr := command(tool("autonomous-enrollment-install-v1"), nil, "status"); statusErr != nil || value.Status != "not_installed" || value.Stage != "" {
		return fmt.Errorf("fresh status")
	}
	first, err := positive("autonomous-enrollment-install-v1", 0, "initialize")
	if err != nil || len(first.PublicKey) != 64 || first.EnrollmentState != "idle" {
		return fmt.Errorf("initial autonomous identity")
	}
	autonomousCreated = true
	if value, statusErr := command(tool("autonomous-enrollment-install-v1"), nil, "status"); statusErr != nil || value.Status != "usable" || value.PublicKey != first.PublicKey || value.EnrollmentState != "idle" {
		return fmt.Errorf("installed status")
	}
	if err = negative("autonomous-enrollment-install-v1", "initialize"); err != nil {
		return err
	}
	if err = negative("trust-install-v2", "inspect"); err != nil {
		return err
	}
	if err = negative("autonomous-enrollment-install-v2", "inspect"); err != nil {
		return err
	}
	if _, err = positive("trust-install-v2", 2, "upgrade"); err != nil {
		return err
	}
	for _, name := range []string{"autonomous-enrollment-install-v2", "autonomous-enrollment-install-v1"} {
		value, err := positive(name, 0, "inspect")
		if err != nil || value.PublicKey != first.PublicKey || value.EnrollmentState != "idle" {
			return fmt.Errorf("identity changed across upgrade")
		}
		value, err = command(tool(name), nil, "status")
		if err != nil || value.Status != "usable" || value.PublicKey != first.PublicKey || value.EnrollmentState != "idle" {
			return fmt.Errorf("status changed across upgrade")
		}
	}
	if err = negative("trust-install-v1", "upgrade"); err != nil {
		return err
	}
	if err = negative("trust-install-v2", "upgrade", filepath.Join(dir, "old-profile.json")); err != nil {
		return err
	}
	if _, err = positive("trust-install-v2", 2, "inspect"); err != nil {
		return err
	}
	if err = product(tool("installed-autonomous-product-v2")); err != nil {
		return err
	}
	if err = product(tool("installed-autonomous-product-v1")); err != nil {
		return err
	}
	if value, cleanupErr := command(tool("autonomous-enrollment-install-v2"), nil, "cleanup-test"); cleanupErr != nil || value.Status != "cleaned" {
		return fmt.Errorf("autonomous cleanup")
	}
	autonomousCreated = false
	if value, reserveErr := command(tool("autonomous-enrollment-install-v2"), nil, "reserve-test"); reserveErr != nil || value.Status != "maintenance_required" || value.Stage != "binding_pending" {
		return fmt.Errorf("pending reservation")
	}
	reservationCreated = true
	if value, statusErr := command(tool("autonomous-enrollment-install-v2"), nil, "status"); statusErr != nil || value.Status != "maintenance_required" || value.Stage != "binding_pending" {
		return fmt.Errorf("pending status")
	}
	if err = negative("autonomous-enrollment-install-v2", "initialize"); err != nil {
		return err
	}
	if value, cleanupErr := command(tool("autonomous-enrollment-install-v2"), nil, "cleanup-reservation-test"); cleanupErr != nil || value.Status != "cleaned" {
		return fmt.Errorf("reservation cleanup")
	}
	reservationCreated = false
	if value, statusErr := command(tool("autonomous-enrollment-install-v2"), nil, "status"); statusErr != nil || value.Status != "not_installed" || value.Stage != "" {
		return fmt.Errorf("clean status")
	}
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Println("VALIDATION FAILED: autonomous native upgrade lifecycle")
		os.Exit(1)
	}
	fmt.Println("VALIDATION PASSED: native autonomous identity survives signed epoch upgrade; read-only status distinguishes absent/usable/pending; interrupted TEST install stays closed until exact cleanup; both compiled generations restart and join. NOT cross-machine Headscale, GUI or signed installer acceptance.")
}
