package nodeproof

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"tailscale.com/client/local"
)

func TestBoundFreshChallengeTranscript(t *testing.T) {
	pub, k, _ := ed25519.GenerateKey(rand.Reader)
	c := challenge{Ticket: strings.Repeat("a", 32), Grant: strings.Repeat("b", 32), Device: identity(pub), Node: "nodekey:" + strings.Repeat("c", 64), IP: "100.120.0.2", Nonce: strings.Repeat("d", 64), Issued: time.Now().Unix()}
	sig := ed25519.Sign(k, transcript(c))
	if !ed25519.Verify(pub, transcript(c), sig) {
		t.Fatal("signature")
	}
	mutations := []challenge{c, c, c, c, c, c, c}
	mutations[0].Ticket = "other"
	mutations[1].Grant = "other"
	mutations[2].Device = "other"
	mutations[3].Node = "other"
	mutations[4].IP = "100.120.0.3"
	mutations[5].Nonce = strings.Repeat("e", 64)
	mutations[6].Issued++
	for _, m := range mutations {
		if ed25519.Verify(pub, transcript(m), sig) {
			t.Fatal("unbound field")
		}
	}
}
func TestProofFramesBounded(t *testing.T) {
	var b bytes.Buffer
	c := challenge{Nonce: strings.Repeat("a", 64)}
	if frame(&b, c) != nil {
		t.Fatal("frame")
	}
	var got challenge
	if read(&b, &got) != nil || got != c {
		t.Fatal("read")
	}
	if frame(io.Discard, strings.Repeat("a", 2049)) == nil {
		t.Fatal("large frame")
	}
	for _, bad := range [][]byte{{0, 0}, {255, 255}, {0, 2, '{'}} {
		if read(bytes.NewReader(bad), &got) == nil {
			t.Fatal("bad frame")
		}
	}
}
func TestDialSelfNeverTouchesSystemNetwork(t *testing.T) {
	called := false
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.Header.Get("Dial-Port") != "47476" {
			t.Error("port")
		}
		w.Header().Set("Dial-Self", "true")
		w.Header().Set("Dial-Addr", "127.0.0.1:1")
		w.WriteHeader(200)
	}))
	defer s.Close()
	lc := &local.Client{Dial: func(ctx context.Context, network, addr string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "tcp", strings.TrimPrefix(s.URL, "http://"))
	}}
	if c, e := DialOnlyOverlay(context.Background(), lc, "100.120.0.2", 47476); e != ErrNoOverlayRoute || c != nil || !called {
		t.Fatal("fallback permitted")
	}
}
func TestChallengeWireContainsNoPrivateKey(t *testing.T) {
	c := challenge{Nonce: strings.Repeat("a", 64)}
	b, _ := json.Marshal(c)
	if strings.Contains(string(b), "Private") {
		t.Fatal("private field")
	}
}
