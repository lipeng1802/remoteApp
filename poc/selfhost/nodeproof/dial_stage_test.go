package nodeproof

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"remoteapp.local/selfhost-poc/startuptrace"
	"strings"
	"tailscale.com/client/local"
	"testing"
	"time"
)

// Real LocalAPI HTTP transport: fixed diagnostics never expose response bodies,
// and forbidden Dial-Self cannot become an OS/system socket fallback.
func TestOverlayUpgradeDiagnosticsPreserveDenial(t *testing.T) {
	for _, item := range []struct {
		name, stage string
		status      int
		self        bool
	}{
		{"self", "overlay_route_missing", 200, true},
		{"denied", "overlay_permission_denied", 403, false},
		{"unavailable", "overlay_upgrade_rejected", 502, false},
	} {
		t.Run(item.name, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Dial-Host") != "100.120.0.2" || r.Header.Get("Dial-Port") != "47476" {
					t.Error("destination changed")
				}
				if item.self {
					w.Header().Set("Dial-Self", "true")
					w.Header().Set("Dial-Addr", "127.0.0.1:1")
				}
				w.WriteHeader(item.status)
				io.WriteString(w, "PRIVATE RESPONSE")
			}))
			defer s.Close()
			lc := &local.Client{Dial: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, "tcp", strings.TrimPrefix(s.URL, "http://"))
			}}
			r, w, e := os.Pipe()
			if e != nil {
				t.Fatal(e)
			}
			defer r.Close()
			old := os.Stderr
			os.Stderr = w
			defer func() { os.Stderr = old; w.Close() }()
			conn, err := DialOnlyOverlay(startuptrace.Context(context.Background(), true), lc, "100.120.0.2", 47476)
			w.Close()
			os.Stderr = old
			b, e := io.ReadAll(r)
			if e != nil || string(b) != "PRODUCT_STAGE "+item.stage+"\n" {
				t.Fatal("unsafe/incorrect stage")
			}
			expected := ErrOverlayDial
			if item.self {
				expected = ErrNoOverlayRoute
			}
			if conn != nil || err != expected {
				t.Fatal("denial changed")
			}
		})
	}
}

func TestSuccessfulOverlayUpgradeStillCarriesSocketPayload(t *testing.T) {
	done := make(chan struct{})
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(done)
		w.Header().Set("Connection", "upgrade")
		w.Header().Set("Upgrade", "ts-dial")
		w.WriteHeader(http.StatusSwitchingProtocols)
		conn, rw, e := w.(http.Hijacker).Hijack()
		if e != nil {
			t.Error(e)
			return
		}
		defer conn.Close()
		if e = rw.Flush(); e != nil {
			t.Error(e)
			return
		}
		buf := make([]byte, 1)
		if _, e = io.ReadFull(rw, buf); e != nil {
			t.Error(e)
			return
		}
		conn.Write(buf)
	}))
	defer s.Close()
	lc := &local.Client{Dial: func(ctx context.Context, network, addr string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "tcp", strings.TrimPrefix(s.URL, "http://"))
	}}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conn, e := DialOnlyOverlay(ctx, lc, "100.120.0.2", 47476)
	if e != nil {
		t.Fatal(e)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(time.Second))
	if _, e = conn.Write([]byte("P")); e != nil {
		t.Fatal(e)
	}
	buf := make([]byte, 1)
	if _, e = io.ReadFull(conn, buf); e != nil || string(buf) != "P" {
		t.Fatal("upgraded socket broken", e)
	}
	<-done
}

func TestUpgradeFailureClassificationHasNoRawText(t *testing.T) {
	for _, v := range []struct {
		code int
		body string
		want startuptrace.Stage
	}{
		{502, "dial failure: dial tcp PRIVATE:47476: connect: connection refused\n", startuptrace.OverlayConnectRefused},
		{502, "dial failure: PRIVATE: i/o timeout\n", startuptrace.OverlayConnectTimeout},
		{502, "dial failure: PRIVATE context deadline exceeded\n", startuptrace.OverlayConnectTimeout},
		{502, "dial failure: PRIVATE\n", startuptrace.OverlayConnectFailed},
		{502, "resolve failure: PRIVATE\n", startuptrace.OverlayResolveFailed},
		{502, "PRIVATE connection refused", startuptrace.OverlayUpgradeRejected},
		{400, "bad ts-dial upgrade\n", startuptrace.OverlayBadUpgrade},
		{400, "PRIVATE", startuptrace.OverlayUpgradeRejected},
		{401, "dial failure: PRIVATE connection refused", startuptrace.OverlayUpgradeRejected},
	} {
		if got := classifyUpgradeFailure(v.code, v.body); got != v.want || !startuptrace.Valid(string(got)) {
			t.Fatal("nonfixed or incorrect classification")
		}
	}
}

type noDiagnosticBody struct{ reads int }

func (b *noDiagnosticBody) Read(p []byte) (int, error) { b.reads++; return 0, io.EOF }
func (b *noDiagnosticBody) Close() error               { return nil }

type diagnosticResponse struct{ body io.ReadCloser }

func (r diagnosticResponse) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: 502, Header: make(http.Header), Body: r.body}, nil
}
func TestProductionDoesNotReadDiagnosticErrorBody(t *testing.T) {
	b := &noDiagnosticBody{}
	lc := &local.Client{Transport: diagnosticResponse{body: b}, OmitAuth: true}
	conn, e := DialOnlyOverlay(context.Background(), lc, "100.120.0.2", 47476)
	if e != ErrOverlayDial || conn != nil || b.reads != 0 {
		t.Fatal("TEST diagnostic enabled in production")
	}
}

type countedDiagnosticBody struct {
	source *strings.Reader
	bytes  int
}

func (b *countedDiagnosticBody) Read(p []byte) (int, error) {
	n, e := b.source.Read(p)
	b.bytes += n
	return n, e
}
func (b *countedDiagnosticBody) Close() error { return nil }
func TestTESTDiagnosticBodyIsBoundedAndOversizeUnclassified(t *testing.T) {
	b := &countedDiagnosticBody{source: strings.NewReader("dial failure: connection refused " + strings.Repeat("PRIVATE", 10000))}
	lc := &local.Client{Transport: diagnosticResponse{body: b}, OmitAuth: true}
	r, w, e := os.Pipe()
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	old := os.Stderr
	os.Stderr = w
	defer func() { os.Stderr = old; w.Close() }()
	conn, e := DialOnlyOverlay(startuptrace.Context(context.Background(), true), lc, "100.120.0.2", 47476)
	w.Close()
	os.Stderr = old
	raw, readErr := io.ReadAll(r)
	if e != ErrOverlayDial || conn != nil || b.bytes != 513 || readErr != nil || string(raw) != "PRODUCT_STAGE overlay_upgrade_rejected\n" {
		t.Fatal("unbounded or unsafe diagnostic")
	}
}
