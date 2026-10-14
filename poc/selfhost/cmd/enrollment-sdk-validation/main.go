// Standalone TEST native lease + actual SDK cancellation/join validation.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	"remoteapp.local/selfhost-poc/enrollmentstore"
	"remoteapp.local/selfhost-poc/installeddevice"
)

func main() {
	if len(os.Args) != 1 || run() != nil {
		fmt.Println("VALIDATION FAILED: actual SDK cancellation/join; preserve isolated TEST evidence")
		os.Exit(1)
	}
	fmt.Println("VALIDATION PASSED: native TEST lease + actual SDK Up failure/cancellation + synchronous SDK close + persisted attempted/retry refusal + joined cleanup. NOT successful Headscale enrollment, signed HTTPS registration, device-client IPC, GUI or installer acceptance.")
}

func run() (result error) {
	denied := enrollmentstore.ErrStorage
	listener, err := net.Listen("tcp4", "127.0.0.1:18443")
	if err != nil {
		return denied
	} // Never reuse or stop an existing listener.
	var requests atomic.Int64
	server := &http.Server{ReadHeaderTimeout: time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Do not inspect/log bodies, private node identity or supplied key.
		requests.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	})}
	served := make(chan error, 1)
	go func() { served <- server.Serve(listener) }()
	fixtureJoined := false
	closeFixture := func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		shutdownErr := server.Shutdown(ctx)
		serveErr := <-served
		fixtureJoined = true
		if shutdownErr != nil || serveErr != http.ErrServerClosed {
			return denied
		}
		return nil
	}
	defer func() {
		if !fixtureJoined && closeFixture() != nil {
			result = denied
		}
	}()
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return denied
	}
	scope := hex.EncodeToString(random[:])
	s, err := enrollmentstore.Initialize(scope, true)
	if err != nil {
		return denied
	}
	if s.Close() != nil {
		return denied
	}
	node, err := installeddevice.NativeSDKRejectionValidationFactory(scope)(context.Background())
	if err != nil || node == nil {
		if node != nil {
			node.CloseJoined()
		}
		return denied
	}
	joined := false
	defer func() {
		if !joined && node.CloseJoined() != nil {
			result = denied
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	// Public invalid fixture text, not a minted credential. No public service
	// request or successful registration is possible through this fixture.
	key, address, upErr := node.Register(ctx, "remoteapp-invalid-test-key")
	cancel()
	if upErr == nil || key != "" || address != "" || requests.Load() == 0 {
		return denied
	}
	fmt.Println("PASS actual SDK contacted fixed failure fixture; registration rejected")
	if other, err := enrollmentstore.Open(scope, true); err == nil {
		other.Close()
		return denied
	}
	// Wait for actual synchronous SDK close. No timeout-as-success, kill,
	// stale-lock removal or native lease release before the SDK joins.
	if node.CloseJoined() != nil {
		return denied
	}
	joined = true
	if closeFixture() != nil {
		return denied
	}
	fmt.Println("PASS actual SDK close returned before native lease reopened")
	s, err = enrollmentstore.Open(scope, true)
	if err != nil {
		return denied
	}
	defer s.Close()
	if s.State().Phase != "attempted" || s.Check() != nil || s.Begin() == nil {
		return denied
	}
	if s.Close() != nil {
		return denied
	}
	if retry, err := installeddevice.NativeSDKRejectionValidationFactory(scope)(context.Background()); err == nil || retry != nil {
		if retry != nil {
			retry.CloseJoined()
		}
		return denied
	}
	s, err = enrollmentstore.Open(scope, true)
	if err != nil {
		return denied
	}
	defer s.Close()
	if s.CleanupTestAfterJoin() != nil {
		return denied
	}
	fmt.Println("PASS attempted result retained and retry denied; exact TEST cleanup completed")
	return nil
}
