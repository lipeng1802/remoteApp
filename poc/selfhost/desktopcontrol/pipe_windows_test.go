package desktopcontrol

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"remoteapp.local/selfhost-poc/privatepipe"
	"testing"
	"time"
)

func TestWindowsPipeNamespaceCollisionAndProbe(t *testing.T) {
	var nonce [16]byte
	rand.Read(nonce[:])
	scope := hex.EncodeToString(nonce[:])
	name := `\\.\pipe\remoteapp-session-` + scope
	for _, bad := range []string{name + "x", `\\server\pipe\remoteapp-session-` + scope, `\\.\pipe\remoteapp-vault-` + scope} {
		if validPipe(bad) {
			t.Fatal("foreign pipe accepted")
		}
	}
	ln, e := Listen(name)
	if e != nil {
		t.Fatal(e)
	}
	if other, e := Listen(name); e == nil {
		other.Close()
		ln.Close()
		t.Fatal("first instance collision allowed")
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	b := &backend{}
	go func() { defer close(done); Serve(ctx, ln, scope, "public", b) }()
	defer func() { cancel(); <-done }()
	limit, stop := context.WithTimeout(ctx, 3*time.Second)
	defer stop()
	c, e := Dial(limit, name)
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	w := json.NewEncoder(c)
	r := privatepipe.NewReader(c)
	w.Encode(map[string]string{"Scope": scope, "Expected": "public"})
	status(t, r, "ready")
	w.Encode(Request{Op: "probe"})
	status(t, r, "probe_passed")
}
