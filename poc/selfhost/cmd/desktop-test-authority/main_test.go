//go:build darwin || linux

package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/broker"
	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/productprofile"
)

func stageRoot(t *testing.T) string {
	t.Helper()
	parent, err := os.MkdirTemp("/tmp", "remoteapp-da-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(parent) })
	actual, err := filepath.EvalSymlinks(parent)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(actual, "remoteapp-desktop-test-1234567890abcdef")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chown(root, -1, os.Getegid()); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestStageRetainsStartedWithoutProvider(t *testing.T) {
	root := stageRoot(t)
	ctx := context.Background()
	if prepare(root) != nil || prepare(root) == nil {
		t.Fatal("stage must be prepared exactly once")
	}
	if retire(ctx, root) == nil || serve(ctx, root) == nil {
		t.Fatal("retired a never-started stage or started without provider")
	}
	if !marker(root, startedFile, "started") {
		t.Fatal("failed start evidence missing")
	}
	for _, name := range []string{readyFile, joinedFile, retiredFile} {
		if _, err := os.Lstat(filepath.Join(root, name)); !os.IsNotExist(err) {
			t.Fatal("false receipt", name)
		}
	}
	if serve(ctx, root) == nil || retire(ctx, root) == nil {
		t.Fatal("failed stage was reused")
	}
}

func TestEndpointSignedLoopbackAndJoinedBroker(t *testing.T) {
	for _, fault := range []bool{false, true} {
		t.Run(map[bool]string{false: "cancel", true: "network-fault"}[fault], func(t *testing.T) {
			root := stageRoot(t)
			if err := prepare(root); err != nil {
				t.Fatal(err)
			}
			store, _, err := openStage(root)
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			ctx, stop := context.WithTimeout(context.Background(), 5*time.Second)
			defer stop()
			networkDone := make(chan struct{})
			done := make(chan error, 1)
			go func() {
				done <- serveEndpoint(ctx, root, hex.EncodeToString(store.PublicKey()), &broker.Authority{Store: store}, networkDone)
			}()
			joined := false
			defer func() {
				stop()
				if !joined {
					<-done
				}
			}()
			var receipt readyReceipt
			for receipt.Port == 0 {
				if raw, err := os.ReadFile(filepath.Join(root, readyFile)); err == nil {
					if json.Unmarshal(raw, &receipt) != nil {
						t.Fatal("invalid ready receipt")
					}
					break
				}
				select {
				case err := <-done:
					joined = true
					t.Fatal("endpoint returned before readiness", err)
				case <-ctx.Done():
					t.Fatal("readiness timeout")
				case <-time.After(10 * time.Millisecond):
				}
			}
			raw, err := json.Marshal(receipt.Profile)
			if err != nil {
				t.Fatal(err)
			}
			if claims, err := productprofile.DesktopClaims(raw, receipt.ProfileRoot, time.Now()); err != nil ||
				claims.Issuer != hex.EncodeToString(store.PublicKey()) {
				t.Fatal("builder rejected ready profile", err)
			}
			rootKey, _ := hex.DecodeString(receipt.ProfileRoot)
			_, key, err := ed25519.GenerateKey(rand.Reader)
			if err != nil {
				t.Fatal(err)
			}
			signer, err := invite.NewMemorySigner(key)
			if err != nil {
				t.Fatal(err)
			}
			client, err := invite.NewTestTunnelNetworkClientOnPort(receipt.Profile, rootKey, productprofile.DesktopService, 1, signer, receipt.Port)
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			if result, err := client.DeviceRequest(ctx, invite.Body{Action: "register"}); err != nil || result.Status != "registered" {
				t.Fatal("signed HTTPS registration failed", err)
			}
			if fault {
				close(networkDone)
			} else {
				stop()
			}
			select {
			case err := <-done:
				joined = true
				if (err != nil) != fault {
					t.Fatal("unexpected endpoint result", err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("endpoint failed to join")
			}
			if _, err := os.Lstat(filepath.Join(root, "broker.sock")); !os.IsNotExist(err) {
				t.Fatal("broker socket still present")
			}
		})
	}
}
