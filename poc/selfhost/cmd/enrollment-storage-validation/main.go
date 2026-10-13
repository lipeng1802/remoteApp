// Fresh standalone native TEST validation. No network, credentials, GUI or
// existing identities. Child probes are this exact validation executable.
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"time"

	"remoteapp.local/selfhost-poc/enrollmentstore"
	"remoteapp.local/selfhost-poc/installeddevice"
)

func main() {
	if len(os.Args) == 4 && os.Args[1] == "--probe" {
		if probe(os.Args[2], os.Args[3]) != nil {
			os.Exit(1)
		}
		fmt.Println("PROBE_PASSED")
		return
	}
	if len(os.Args) != 1 || run() != nil {
		fmt.Println("VALIDATION FAILED: native enrollment storage; preserve isolated evidence")
		os.Exit(1)
	}
	fmt.Println("VALIDATION PASSED: native enrollment journal + private node directory + cross-process exclusive lease + restart denial after attempted/registered. NOT Headscale registration, compiled device entry, product TLS/GUI or signed installer acceptance.")
}

func probe(scope, phase string) error {
	s, e := enrollmentstore.Open(scope, true)
	if phase == "locked" {
		if e == nil {
			s.Close()
			return enrollmentstore.ErrStorage
		}
		return nil
	}
	if e != nil {
		return e
	}
	defer s.Close()
	if s.State().Phase != phase || s.Check() != nil {
		return enrollmentstore.ErrStorage
	}
	if phase != "idle" && s.Begin() == nil {
		return enrollmentstore.ErrStorage
	}
	return s.Close()
}
func subprocess(scope, phase string) error {
	exe, e := os.Executable()
	if e != nil {
		return enrollmentstore.ErrStorage
	}
	c := exec.Command(exe, "--probe", scope, phase)
	var out bytes.Buffer
	c.Stdout = &out
	c.Stderr = &bytes.Buffer{}
	if c.Start() != nil {
		return enrollmentstore.ErrStorage
	}
	// Do not force kill or count a timeout as a successful native join.
	done := make(chan error, 1)
	go func() { done <- c.Wait() }()
	select {
	case e := <-done:
		if e != nil || out.String() != "PROBE_PASSED\n" {
			return enrollmentstore.ErrStorage
		}
		return nil
	case <-time.After(20 * time.Second):
		return enrollmentstore.ErrStorage
	}
}
func run() (result error) {
	var random [16]byte
	if _, e := rand.Read(random[:]); e != nil {
		return enrollmentstore.ErrStorage
	}
	scope := hex.EncodeToString(random[:])
	s, e := enrollmentstore.Initialize(scope, true)
	if e != nil {
		return e
	}
	defer func() {
		if s != nil {
			if e := s.Close(); e != nil {
				result = e
			}
		}
	}()
	if subprocess(scope, "locked") != nil {
		return enrollmentstore.ErrStorage
	}
	// Same-process recursion must also be denied on Windows mutex threads.
	if other, e := enrollmentstore.Open(scope, true); e == nil {
		other.Close()
		return enrollmentstore.ErrStorage
	}
	if s.Close() != nil || subprocess(scope, "idle") != nil {
		return enrollmentstore.ErrStorage
	}
	node, e := installeddevice.NativeEnrollmentFactory(scope, true)(context.Background())
	if e != nil || node == nil {
		if node != nil {
			node.CloseJoined()
		}
		return enrollmentstore.ErrStorage
	}
	pub := node.PublicKey()
	if subprocess(scope, "locked") != nil {
		node.CloseJoined()
		return enrollmentstore.ErrStorage
	}
	if other, e := enrollmentstore.Initialize(scope, true); e == nil {
		other.Close()
		node.CloseJoined()
		return enrollmentstore.ErrStorage
	}
	if node.CloseJoined() != nil || subprocess(scope, "attempted") != nil {
		return enrollmentstore.ErrStorage
	}
	fmt.Println("PASS attempted result survives actual process reopen; no retry")
	s, e = enrollmentstore.Open(scope, true)
	if e != nil {
		return e
	}
	// State completion fixture only: this IP is NOT observed from Headscale.
	if s.Registered(pub, "100.120.0.7") != nil || s.Close() != nil || subprocess(scope, "registered") != nil {
		return enrollmentstore.ErrStorage
	}
	fmt.Println("PASS registered receipt survives actual process reopen; native lease exclusive")
	s, e = enrollmentstore.Open(scope, true)
	if e != nil {
		return e
	}
	if s.CleanupTestAfterJoin() != nil {
		return enrollmentstore.ErrStorage
	}
	s = nil
	if other, e := enrollmentstore.Open(scope, true); e == nil {
		other.Close()
		return enrollmentstore.ErrStorage
	}
	fmt.Println("PASS exact TEST credentials and offline node key cleaned; empty directories retained")
	return nil
}
