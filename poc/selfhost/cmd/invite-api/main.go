// Loopback-only invitation backend fixture; never reverse proxy it publicly.
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"remoteapp.local/selfhost-poc/invite"
)

func run() error {
	if len(os.Args) != 1 {
		return fmt.Errorf("invalid_arguments")
	}
	root, err := filepath.Abs("../../artifacts/connection-poc/invitation")
	if err != nil {
		return err
	}
	store, err := invite.Open(root, time.Now)
	if err != nil {
		return err
	}
	defer store.Close()
	listener, err := net.Listen("tcp", "127.0.0.1:18445")
	if err != nil {
		return err
	}
	defer listener.Close()
	server := &http.Server{Handler: invite.Handler(store), ReadHeaderTimeout: 2 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second, IdleTimeout: 15 * time.Second, MaxHeaderBytes: 8192, ErrorLog: log.New(io.Discard, "", 0)}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	fmt.Println("PASS invitation_backend_loopback_ready")
	select {
	case err := <-done:
		if err != http.ErrServerClosed {
			return err
		}
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			server.Close()
			return err
		}
		if err := <-done; err != http.ErrServerClosed {
			return err
		}
	}
	return nil
}
func main() {
	if run() != nil {
		fmt.Println("FAIL invitation_backend")
		os.Exit(1)
	}
}
