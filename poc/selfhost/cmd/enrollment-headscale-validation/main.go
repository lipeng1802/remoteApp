// Explicit Mac operator acceptance; SSH credentials never become product IPC.
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"remoteapp.local/selfhost-poc/clientruntime"
	"remoteapp.local/selfhost-poc/enrollmentstore"
	"remoteapp.local/selfhost-poc/headscale"
	"remoteapp.local/selfhost-poc/installeddevice"
	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/nodeidentity"
	guard "remoteapp.local/selfhost-poc/sdkheadscalevalidation"
)

const expectedStore = "5c40c3230bd331b5ff47c9aefb18dd16c7f4e3d4d19f1ae18caed4caf1db33ca"

type audit struct {
	Version                          int
	Ticket, Scope, NodePublic, Stage string
	BaselineFingerprint              string
	SDKJoined                        bool
	Handles                          guard.Handles
}
type limited struct{ bytes.Buffer }

func (b *limited) Write(p []byte) (int, error) {
	if b.Len()+len(p) > 1<<20 {
		return 0, guard.ErrValidation
	}
	return b.Buffer.Write(p)
}

// No CommandContext kill: a timed out/failed SSH response is NOT proof that the
// remote mutation joined. Unknown writes fence Guard and retain recovery intent.
func remote(ctx context.Context, command string) ([]byte, error) {
	if ctx.Err() != nil {
		return nil, guard.ErrValidation
	}
	c := exec.Command("/usr/bin/ssh", "-o", "BatchMode=yes", "-o", "IdentitiesOnly=yes", "-o", "StrictHostKeyChecking=yes", "-o", "ConnectTimeout=8", "-o", "ServerAliveInterval=5", "-o", "ServerAliveCountMax=3", "-i", "/Users/lipeng/.ssh/remoteapp_poc_server_ed25519", "root@182.92.117.114", command)
	var out limited
	c.Stdout = &out
	c.Stderr = io.Discard
	if c.Run() != nil || ctx.Err() != nil {
		return nil, guard.ErrValidation
	}
	return out.Bytes(), nil
}
func runCLI(ctx context.Context, args ...string) ([]byte, error) {
	// Guard generates this whitelist. Reject shell syntax even on read-only calls.
	for _, arg := range args {
		if arg == "" {
			return nil, guard.ErrValidation
		}
		for _, c := range arg {
			if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
				return nil, guard.ErrValidation
			}
		}
	}
	return remote(ctx, "env LC_ALL=C /opt/remoteapp-poc/headscale -c /etc/remoteapp-poc/config.yaml -o json "+strings.Join(args, " "))
}
func unchangedStore(ctx context.Context) bool {
	b, e := remote(ctx, "env LC_ALL=C sha256sum /var/lib/remoteapp-authority/state.json")
	parts := strings.Fields(string(b))
	return e == nil && len(parts) == 2 && parts[0] == expectedStore && parts[1] == "/var/lib/remoteapp-authority/state.json"
}
func persist(dir string, a audit) error {
	b, e := json.Marshal(a)
	if e != nil {
		return guard.ErrValidation
	}
	f, e := os.CreateTemp(dir, ".audit-")
	if e != nil {
		return guard.ErrValidation
	}
	path := f.Name()
	defer os.Remove(path)
	if _, e = f.Write(b); e != nil {
		f.Close()
		return guard.ErrValidation
	}
	syncErr := f.Sync()
	closeErr := f.Close()
	if syncErr != nil || closeErr != nil || os.Rename(path, filepath.Join(dir, "public-recovery.json")) != nil {
		return guard.ErrValidation
	}
	d, e := os.Open(dir)
	if e != nil {
		return guard.ErrValidation
	}
	defer d.Close()
	if d.Sync() != nil {
		return guard.ErrValidation
	}
	got, e := os.ReadFile(filepath.Join(dir, "public-recovery.json"))
	if e != nil || !bytes.Equal(got, b) {
		return guard.ErrValidation
	}
	return nil
}
func main() {
	if runtime.GOOS != "darwin" || !((len(os.Args) == 2 && os.Args[1] == "--run-scoped-headscale") || (len(os.Args) == 3 && os.Args[1] == "--retire-joined-test")) {
		fmt.Println("REFUSED: explicit Mac operator scoped Headscale acceptance required")
		os.Exit(1)
	}
	if os.Args[1] == "--retire-joined-test" {
		if recoverJoined(os.Args[2]) != nil {
			fmt.Println("RECOVERY FAILED: retain exact TEST evidence; no new Mint")
			os.Exit(1)
		}
		fmt.Println("RECOVERY PASSED: explicitly retired joined TEST ticket; baseline and formal Store unchanged; native TEST cleanup. NOT registration acceptance.")
		return
	}
	if run() != nil {
		fmt.Println("VALIDATION FAILED: scoped actual Headscale; preserve public recovery intent and native TEST evidence")
		os.Exit(1)
	}
	fmt.Println("VALIDATION PASSED: native TEST journal + actual Headscale Mint/SDK Up/Observe + SDK join + exact scoped retirement + unchanged long-lived resources/zero ACL/formal Store + TEST cleanup. NOT product credential supply, signed grant/enroll, live proof/bind/watch, GUI, Windows or installer acceptance.")
}

func run() (result error) {
	denied := guard.ErrValidation
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	baseline, e := guard.Capture(ctx, runCLI)
	if e != nil || !baseline.LongLivedBaseline() || !unchangedStore(ctx) {
		return denied
	}
	var raw [32]byte
	if _, e = rand.Read(raw[:]); e != nil {
		return denied
	}
	a := audit{Version: 1, Ticket: hex.EncodeToString(raw[:16]), Scope: hex.EncodeToString(raw[16:]), Stage: "prepared", BaselineFingerprint: baseline.Fingerprint()}
	exe, e := os.Executable()
	if e != nil {
		return denied
	}
	dir, e := os.MkdirTemp(filepath.Dir(exe), "headscale-sdk-r28-")
	if e != nil {
		return denied
	}
	if persist(dir, a) != nil {
		return denied
	}
	fmt.Println("RECOVERY_METADATA: " + filepath.Join(dir, "public-recovery.json")) // public only
	s, e := enrollmentstore.Initialize(a.Scope, true)
	if e != nil {
		return denied
	}
	if s.Close() != nil {
		return denied
	}
	node, e := installeddevice.NativeEnrollmentFactory(a.Scope, true)(ctx)
	if e != nil || node == nil {
		if node != nil {
			node.CloseJoined()
		}
		return denied
	}
	joined := false
	defer func() {
		if !joined {
			if node.CloseJoined() != nil {
				result = denied
				return
			}
			joined = true
			a.SDKJoined = true
			if persist(dir, a) != nil {
				result = denied
			}
		}
	}()
	a.NodePublic = node.PublicKey()
	a.Stage = "native_attempted"
	if persist(dir, a) != nil {
		return denied
	}
	g := &guard.Guard{Run: runCLI, Ticket: a.Ticket, NodeKey: a.NodePublic, BeforeMutation: func(stage string, h guard.Handles) error { a.Stage = stage; a.Handles = h; return persist(dir, a) }}
	d := &headscale.Driver{Run: g.Execute}
	record := invite.ProvisionRecord{ID: a.Ticket, NodeKey: a.NodePublic, Expires: time.Now().Add(100 * time.Second).Unix()}
	g.MaxExpiry = record.Expires
	credential, e := d.Mint(ctx, record)
	if e != nil {
		return denied
	} // No auto-clean/retry after an unknown Mint.
	record.UserID = credential.UserID
	record.KeyID = credential.KeyID
	a.Stage = "mint_validated"
	a.Handles = g.Handles
	if persist(dir, a) != nil {
		return denied
	}
	upCtx, upCancel := context.WithTimeout(ctx, 35*time.Second)
	key, ip, upErr := node.Register(upCtx, credential.Secret)
	credential.Secret = ""
	upCancel()
	if upErr != nil || key != a.NodePublic {
		return denied
	} // Preserve server resources; no speculative retirement.
	observation, e := d.Observe(ctx, record)
	if e != nil || observation.NodeKey != key || observation.IP != ip || g.RememberNode(ctx, observation.NodeID) != nil {
		return denied
	}
	fmt.Println("PASS real Headscale SDK registration and control-plane observation agree")
	if node.CloseJoined() != nil {
		return denied
	}
	joined = true
	a.SDKJoined = true
	a.Stage = "sdk_joined"
	a.Handles = g.Handles
	if persist(dir, a) != nil {
		return denied
	}
	s, e = enrollmentstore.Open(a.Scope, true)
	if e != nil {
		return denied
	}
	defer s.Close()
	if s.State().Phase != "registered" || s.State().NodeKey != key || s.State().Address != ip || s.Check() != nil || s.Begin() == nil {
		return denied
	}
	if s.Close() != nil {
		return denied
	}
	if retry, err := installeddevice.NativeEnrollmentFactory(a.Scope, true)(ctx); err == nil || retry != nil {
		if retry != nil {
			retry.CloseJoined()
		}
		return denied
	}
	// Cleanup addresses only the exact observed ticket/user/key/node, after join.
	if d.Cleanup(ctx, record) != nil {
		return denied
	}
	final, e := guard.Capture(ctx, runCLI)
	if e != nil || !baseline.Same(final) || !unchangedStore(ctx) {
		return denied
	}
	fmt.Println("PASS exact ticket retired; original resources, zero ACL and formal Store unchanged")
	s, e = enrollmentstore.Open(a.Scope, true)
	if e != nil {
		return denied
	}
	defer s.Close()
	if s.CleanupTestAfterJoin() != nil {
		return denied
	}
	a.Stage = "verified_cleaned"
	a.Handles = g.Handles
	if persist(dir, a) != nil {
		return denied
	}
	fmt.Println("PASS registered state/retry refusal and joined native TEST cleanup")
	return nil
}

func recoverJoined(path string) error {
	denied := guard.ErrValidation
	exe, e := os.Executable()
	if e != nil {
		return denied
	}
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || filepath.Base(path) != "public-recovery.json" || filepath.Dir(filepath.Dir(path)) != filepath.Dir(exe) || !strings.HasPrefix(filepath.Base(filepath.Dir(path)), "headscale-sdk-r28-") {
		return denied
	}
	real, e := filepath.EvalSymlinks(path)
	if e != nil || real != path {
		return denied
	}
	b, e := os.ReadFile(path)
	if e != nil || len(b) > 4096 {
		return denied
	}
	var a audit
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&a) != nil || a.Version != 1 || !a.SDKJoined || len(a.BaselineFingerprint) != 64 {
		return denied
	}
	switch a.Stage {
	case "key_create_acknowledged", "mint_validated", "node_observed", "sdk_joined":
	case "user_destroy_acknowledged":
	default:
		return denied
	}
	s, e := enrollmentstore.Open(a.Scope, true)
	if e != nil {
		return denied
	}
	key, e := nodeidentity.Prepare(s.NodeDir(), clientruntime.ControlURL)
	if e != nil || key != a.NodePublic || (s.State().Phase != "attempted" && s.State().Phase != "registered") || s.Check() != nil {
		s.Close()
		return denied
	}
	if s.Close() != nil {
		return denied
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	if a.Stage == "user_destroy_acknowledged" {
		final, e := guard.Capture(ctx, runCLI)
		if e != nil || final.Fingerprint() != a.BaselineFingerprint || !unchangedStore(ctx) {
			return denied
		}
		s, e = enrollmentstore.Open(a.Scope, true)
		if e != nil {
			return denied
		}
		defer s.Close()
		if s.CleanupTestAfterJoin() != nil {
			return denied
		}
		a.Stage = "verified_recovered"
		return persist(filepath.Dir(path), a)
	}
	g := &guard.Guard{Run: runCLI, Ticket: a.Ticket, NodeKey: a.NodePublic, Handles: a.Handles, BeforeMutation: func(stage string, h guard.Handles) error {
		a.Stage = stage
		a.Handles = h
		return persist(filepath.Dir(path), a)
	}}
	current, e := guard.Capture(ctx, g.Execute)
	if e != nil || !unchangedStore(ctx) {
		return denied
	}
	original, e := current.WithoutTicket(a.Handles.UserID, a.Ticket)
	if e != nil || !original.LongLivedBaseline() || original.Fingerprint() != a.BaselineFingerprint {
		return denied
	}
	// Observe only this exact user; any extra/mismatched node refuses cleanup.
	data, e := g.Execute(ctx, "nodes", "list")
	var nodes []guard.Node
	if e != nil || json.Unmarshal(data, &nodes) != nil {
		return denied
	}
	count := 0
	for _, n := range nodes {
		if n.User.ID.String() == a.Handles.UserID {
			count++
			if count > 1 || n.Key != a.NodePublic || g.RememberNode(ctx, n.ID.String()) != nil {
				return denied
			}
		}
	}
	d := &headscale.Driver{Run: g.Execute}
	if d.Cleanup(ctx, invite.ProvisionRecord{ID: a.Ticket, NodeKey: a.NodePublic, UserID: a.Handles.UserID}) != nil {
		return denied
	}
	final, e := guard.Capture(ctx, g.Execute)
	if e != nil || final.Fingerprint() != a.BaselineFingerprint || !unchangedStore(ctx) {
		return denied
	}
	s, e = enrollmentstore.Open(a.Scope, true)
	if e != nil {
		return denied
	}
	defer s.Close()
	if s.CleanupTestAfterJoin() != nil {
		return denied
	}
	a.Stage = "verified_recovered"
	a.Handles = g.Handles
	return persist(filepath.Dir(path), a)
}
