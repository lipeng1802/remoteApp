package session

import (
	"context"
	"io"
	"os"
	"remoteapp.local/selfhost-poc/startuptrace"
	"strings"
	"testing"
	"time"
)

func TestProductSecretFixedTraceIsTestOnlyAndDoesNotChangeProof(t *testing.T) {
	for _, mode := range []string{"disabled", "success", "signer"} {
		t.Run(mode, func(t *testing.T) {
			f := portableSetup(t)
			server, controller := sockets(t)
			if mode == "signer" {
				f.server.Key = nil
				f.server.Signer = failingSigner{pub: f.server.Binding.Target}
			}
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			ctx = startuptrace.Context(ctx, mode != "disabled")
			reader, writer, e := os.Pipe()
			if e != nil {
				t.Fatal(e)
			}
			original := os.Stderr
			os.Stderr = writer
			defer func() { os.Stderr = original; reader.Close(); writer.Close() }()
			done := make(chan error, 1)
			go func() {
				done <- DeliverProductSecret(ctx, server, f.server, f.portableCheck, productSecretTestKey, strings.Repeat("c", 64))
			}()
			key, _, controllerErr := ReceiveProductSecret(ctx, controller, f.probe, f.portableCheck)
			targetErr := <-done
			os.Stderr = original
			writer.Close()
			raw, e := io.ReadAll(reader)
			if e != nil {
				t.Fatal(e)
			}
			text := string(raw)
			if strings.Contains(text, productSecretTestKey) || strings.Contains(text, strings.Repeat("c", 64)) {
				t.Fatal("material leaked into telemetry")
			}
			for _, line := range strings.Split(strings.TrimSpace(text), "\n") {
				if line == "" {
					continue
				}
				stage := strings.TrimPrefix(line, "PRODUCT_STAGE ")
				if stage == line || !startuptrace.Valid(stage) {
					t.Fatal("unknown/raw diagnostic")
				}
			}
			if mode == "disabled" && text != "" {
				t.Fatal("production diagnostic enabled")
			}
			if mode == "signer" {
				if controllerErr == nil || targetErr == nil || key != "" || !strings.Contains(text, "PRODUCT_STAGE secret_target_sign\n") || strings.Contains(text, "PRODUCT_STAGE secret_target_welcome\n") {
					t.Fatal("signer failure stage/proof mismatch")
				}
			} else {
				if controllerErr != nil || targetErr != nil || key != productSecretTestKey {
					t.Fatal("mutual proof changed")
				}
				if mode == "success" && (!strings.Contains(text, "PRODUCT_STAGE secret_target_send\n") || !strings.Contains(text, "PRODUCT_STAGE secret_controller_canonical\n")) {
					t.Fatal("successful stage absent")
				}
			}
		})
	}
}
