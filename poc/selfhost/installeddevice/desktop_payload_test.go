package installeddevice

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
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
	"remoteapp.local/selfhost-poc/streamgate"
	"tailscale.com/tsnet"
)

func TestDesktopPayloadChildFixture(t *testing.T) {
	if len(os.Args) < 2 || !strings.HasPrefix(os.Args[len(os.Args)-1], "desktop-payload-") {
		return
	}
	scope := strings.TrimPrefix(os.Args[len(os.Args)-1], "desktop-payload-")
	r := privatepipe.NewReader(os.Stdin)
	var b struct {
		Fingerprint, Key, Permission string
		Port                         int
		Target                       string
	}
	if r.Read(&b) != nil || b.Permission != scope || b.Port != 32123 || !canonical(b.Fingerprint, 32) || !canonical(b.Target, 32) {
		os.Exit(2)
	}
	key, e := base64.StdEncoding.DecodeString(b.Key)
	if e != nil || len(key) != 32 {
		os.Exit(3)
	}
	json.NewEncoder(os.Stdout).Encode(map[string]string{"status": "ready", "Permission": scope})
	var stop struct{ Op string }
	if r.Read(&stop) != nil || stop.Op != "stop" {
		os.Exit(4)
	}
	json.NewEncoder(os.Stdout).Encode(map[string]string{"status": "closed"})
	os.Exit(0)
}

func TestDesktopRuntimeDispatchesSamePolicyToPinnedChildAndJoinsIt(t *testing.T) {
	exe, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	exe, e = filepath.EvalSymlinks(exe)
	if e != nil {
		t.Fatal(e)
	}
	raw, e := os.ReadFile(exe)
	if e != nil {
		t.Fatal(e)
	}
	digest := sha256.Sum256(raw)
	for _, scope := range []invite.DesktopPermission{invite.DesktopView, invite.DesktopControl} {
		pub, key, _ := ed25519.GenerateKey(rand.Reader)
		now := time.Now()
		g := invite.Grant{Claims: invite.Claims{Version: 1, ID: strings.Repeat("a", 32), Target: strings.Repeat("b", 64), Controller: strings.Repeat("c", 64), Permission: string(scope), IssuedAt: now.Unix(), ExpiresAt: now.Unix() + 300}}
		g.Signature = testSignature(key, "remoteapp-invite-grant-v1\n", g.Claims)
		binding := invite.NetworkBinding{TargetNode: "nodekey:" + strings.Repeat("1", 64), ControllerNode: "nodekey:" + strings.Repeat("2", 64), TargetIP: "100.120.0.1", ControllerIP: "100.120.0.2"}
		policy, _ := invite.DesktopGrantPolicy(scope)
		calls := 0
		api := requestFunc(func(_ context.Context, b invite.NetworkRequestBody) (invite.NetworkReply, error) {
			calls++
			if b.Action != "tls_identity" || b.Grant != g {
				t.Error("descriptor identity changed")
			}
			now := time.Now()
			c := invite.TLSIdentityClaims{Version: 1, Grant: g, Binding: binding, Fingerprint: strings.Repeat("d", 64), Challenge: b.Challenge, IssuedAt: now.UnixNano(), ExpiresAt: now.Add(2 * time.Second).UnixNano()}
			return invite.NetworkReply{TLS: &invite.TLSIdentity{Claims: c, Signature: testSignature(key, "remoteapp-tls-identity-v1\n", c)}}, nil
		})
		r := &SessionRuntime{policy: policy, peers: &SessionPeers{grant: g, binding: binding}, issuer: pub, client: api, clock: time.Now, gate: &streamgate.Gate{}, node: &tsnet.Server{}}
		other := invite.DesktopView
		if scope == other {
			other = invite.DesktopControl
		}
		if r.RunProduct(context.Background(), clientruntime.Product{}) == nil || r.RunDesktopProduct(context.Background(), other, clientruntime.Product{}) == nil || calls != 0 {
			t.Fatal("wrong local policy touched transport")
		}
		if r.RunDesktopProduct(context.Background(), scope, clientruntime.Product{Observation: &clientruntime.ProductObservation{}}) == nil || calls != 0 {
			t.Fatal("PONG telemetry admitted")
		}
		p := clientruntime.Product{Spec: processowner.Spec{Executable: exe, SHA256: hex.EncodeToString(digest[:]), Args: []string{"-test.run=^TestDesktopPayloadChildFixture$", "desktop-payload-" + string(scope)}}, ControllerKey: base64.StdEncoding.EncodeToString(make([]byte, 32))}
		child, err := r.startProduct(context.Background(), p, false, 32123, strings.Repeat("d", 64))
		if child != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			joined := child.Stop(ctx)
			cancel()
			if joined != nil {
				t.Fatal("same child did not join", joined)
			}
			select {
			case <-child.Done():
			default:
				t.Fatal("false process join")
			}
		}
		if err != nil || child == nil || calls != 1 {
			t.Fatal("runtime selected legacy product entry", err)
		}
	}
}
