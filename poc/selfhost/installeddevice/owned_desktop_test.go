package installeddevice

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/clientruntime"
	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/privatepipe"
	"remoteapp.local/selfhost-poc/processowner"
)

func TestOwnedDesktopGUIFixture(t *testing.T) {
	arg := os.Args[len(os.Args)-1]
	if !strings.HasPrefix(arg, "owned-gui=") {
		return
	}
	dir := strings.TrimPrefix(arg, "owned-gui=")
	in := privatepipe.NewReader(os.Stdin)
	out := json.NewEncoder(os.Stdout)
	var role, permission string
	for {
		var p map[string]string
		if err := in.Read(&p); err != nil {
			os.Exit(2)
		}
		switch p["Op"] {
		case "prepare":
			role, permission = p["Role"], p["Permission"]
			out.Encode(map[string]string{"status": "desktop_prepared", "Role": role, "Permission": permission})
		case "show_invite":
			if os.WriteFile(filepath.Join(dir, "codes"), []byte(p["DeviceCode"]+p["Code"]), 0600) != nil {
				os.Exit(3)
			}
			out.Encode(map[string]string{"status": "invite_displayed", "Permission": permission})
		case "connect_intent":
			if _, err := os.Stat(filepath.Join(dir, "cancel-connect")); err == nil {
				if os.WriteFile(filepath.Join(dir, "connect-waiting"), []byte("waiting"), 0600) != nil {
					os.Exit(8)
				}
				var next map[string]string
				if in.Read(&next) == nil {
					os.Exit(9)
				}
				if os.WriteFile(filepath.Join(dir, "connect-cancelled"), []byte("cancelled"), 0600) != nil ||
					desktopTestWaitFile(dir, "release-cancel") != "release" {
					os.Exit(10)
				}
				out.Encode(map[string]string{"status": "closed"})
				os.Exit(0)
			}
			codes := desktopTestWaitFile(dir, "codes")
			if len(codes) != 20 {
				os.Exit(4)
			}
			out.Encode(map[string]string{"status": "desktop_connect", "Permission": permission, "TargetCode": codes[:12], "Code": codes[12:]})
		case "consent":
			if os.WriteFile(filepath.Join(dir, role), []byte(p["RequestID"]), 0600) != nil {
				os.Exit(5)
			}
			decision := desktopTestWaitFile(dir, role+"-decision")
			out.Encode(map[string]any{"status": "desktop_decision", "Permission": permission,
				"RequestID": p["RequestID"], "Allow": decision == "allow"})
		case "desktop_state":
			state := map[string]any{"status": "desktop_state", "frames": 0, "inputs": 0, "stop_requested": false, "ended": false, "failed": false}
			if role == "target" {
				state["held"], state["releases"] = 0, 0
			}
			out.Encode(state)
		case "stop":
			if os.WriteFile(filepath.Join(dir, role+"-closed"), nil, 0600) != nil {
				os.Exit(6)
			}
			out.Encode(map[string]string{"status": "closed"})
			os.Exit(0)
		default:
			os.Exit(7)
		}
	}
}

func desktopTestWaitFile(dir, name string) string {
	deadline := time.Now().Add(12 * time.Second)
	for time.Now().Before(deadline) {
		if b, err := os.ReadFile(filepath.Join(dir, name)); err == nil {
			return string(b)
		}
		time.Sleep(20 * time.Millisecond)
	}
	return ""
}

func TestOwnedDesktopCoordinationBindsRealAuthorityAndJoins(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		t.Fatal(err)
	}
	binary, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	pin := sha256.Sum256(binary)
	for _, decision := range []string{"allow", "deny", "cancel"} {
		t.Run(decision, func(t *testing.T) {
			release, profile := desktopHTTPSFixture(t)
			dir := t.TempDir()
			product := clientruntime.Product{Spec: processowner.Spec{Executable: exe, SHA256: hex.EncodeToString(pin[:]),
				Args: []string{"-test.run=^TestOwnedDesktopGUIFixture$", "owned-gui=" + dir}}}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			type peer struct {
				request chan Request
				reply   chan any
				done    chan error
				signer  *testSigner
			}
			start := func(target bool) peer {
				_, key, _ := ed25519.GenerateKey(rand.Reader)
				p := peer{make(chan Request), make(chan any, 1), make(chan error, 1), &testSigner{key: key, joined: make(chan struct{})}}
				run := OwnedDesktopSession(release, func() (clientruntime.Signer, error) { return p.signer, nil }, p.request,
					func(v any) error { p.reply <- v; return nil },
					func(context.Context) (EnrollmentNode, error) {
						t.Error("node opened without authenticated provision")
						return nil, invite.ErrDenied
					}, product, invite.DesktopControl, target, 0)
				go func() { p.done <- run(ctx, profile) }()
				t.Cleanup(func() {
					select {
					case <-p.signer.joined:
					case <-time.After(15 * time.Second):
						t.Error("signer not joined")
					}
				})
				select {
				case <-p.reply:
				case <-time.After(3 * time.Second):
					t.Fatal("runner did not open")
				}
				return p
			}
			call := func(p peer, r Request) any {
				t.Helper()
				select {
				case p.request <- r:
				case <-time.After(3 * time.Second):
					t.Fatal("management blocked by GUI")
				}
				select {
				case v := <-p.reply:
					return v
				case <-time.After(3 * time.Second):
					t.Fatal("management reply missing")
					return nil
				}
			}
			if decision == "cancel" {
				if os.WriteFile(filepath.Join(dir, "cancel-connect"), nil, 0600) != nil {
					t.Fatal("fixture setup")
				}
				controller := start(false)
				call(controller, Request{Op: "desktop_begin"})
				if desktopTestWaitFile(dir, "connect-waiting") != "waiting" {
					t.Fatal("connect wait did not begin")
				}
				if call(controller, Request{Op: "desktop_state"}).(map[string]string)["status"] != "product_unavailable" {
					t.Fatal("GUI telemetry touched the outstanding decision")
				}
				select {
				case controller.request <- Request{Op: "stop"}:
				case <-time.After(3 * time.Second):
					t.Fatal("stop could not cancel outstanding GUI wait")
				}
				if desktopTestWaitFile(dir, "connect-cancelled") != "cancelled" {
					t.Fatal("original child did not see cancellation")
				}
				select {
				case <-controller.signer.joined:
					t.Fatal("signer closed while the GUI was still alive")
				default:
				}
				if os.WriteFile(filepath.Join(dir, "release-cancel"), []byte("release"), 0600) != nil {
					t.Fatal("fixture release")
				}
				select {
				case err := <-controller.done:
					if err == nil {
						t.Fatal("interrupted pipe exchange was counted as normal stop")
					}
				case <-time.After(5 * time.Second):
					t.Fatal("canceled worker did not join")
				}
				return
			}
			target, controller := start(true), start(false)
			for _, p := range []peer{target, controller} {
				if call(p, Request{Op: "desktop_begin"}).(map[string]string)["status"] != "desktop_pending" {
					t.Fatal("start failed")
				}
			}
			if a, b := desktopTestWaitFile(dir, "target"), desktopTestWaitFile(dir, "controller"); a == "" || a != b {
				t.Fatal("local decisions were not bound to the same real request")
			}
			for _, p := range []peer{target, controller} {
				if v := call(p, Request{Op: "desktop_status"}).(DesktopProgress); v.Phase != "waiting" || v.RequestID == "" || v.Grant != nil {
					t.Fatal("pending GUI decision admitted")
				}
				for _, r := range []Request{{Op: "desktop_begin"}, {Op: "device", Device: invite.Body{Action: "approve", Allow: true}}} {
					if call(p, r).(map[string]string)["status"] != "request_rejected" {
						t.Fatal("management may replay or substitute consent")
					}
				}
			}
			if os.WriteFile(filepath.Join(dir, "controller-decision"), []byte("allow"), 0600) != nil ||
				os.WriteFile(filepath.Join(dir, "target-decision"), []byte(decision), 0600) != nil {
				t.Fatal("fixture decision")
			}
			final := func(p peer) DesktopProgress {
				deadline := time.Now().Add(5 * time.Second)
				for time.Now().Before(deadline) {
					if v := call(p, Request{Op: "desktop_status"}).(DesktopProgress); v.Phase != "waiting" {
						return v
					}
					time.Sleep(20 * time.Millisecond)
				}
				t.Fatal("authority decision not observed")
				return DesktopProgress{}
			}
			a, b := final(target), final(controller)
			if !a.Valid() || !b.Valid() || a.RequestID != b.RequestID || a.Phase != b.Phase {
				t.Fatal("decisions disagree")
			}
			if decision == "allow" {
				if a.Phase != "approved" || a.Grant == nil || b.Grant == nil || *a.Grant != *b.Grant {
					t.Fatal("matching approved grant missing")
				}
				g := *a.Grant
				g.Claims.ExpiresAt++
				if call(target, Request{Op: "provision", Network: invite.NetworkRequestBody{Grant: g}}).(map[string]string)["status"] != "request_rejected" {
					t.Fatal("different grant entered provision")
				}
			} else if a.Phase != "denied" || a.Grant != nil || b.Grant != nil {
				t.Fatal("target refusal became permission")
			}
			for i, p := range []peer{target, controller} {
				p.request <- Request{Op: "stop"}
				select {
				case err := <-p.done:
					if err != nil {
						t.Fatal("prepared GUI did not join")
					}
				case <-time.After(5 * time.Second):
					t.Fatal("join timeout")
				}
				name := []string{"target-closed", "controller-closed"}[i]
				if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
					t.Fatal("signer returned before GUI stop")
				}
				select {
				case <-p.signer.joined:
				default:
					t.Fatal("runner returned before signer joined")
				}
			}
		})
	}
}
