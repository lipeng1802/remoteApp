package headscale

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"remoteapp.local/selfhost-poc/invite"
)

func record() invite.ProvisionRecord {
	return invite.ProvisionRecord{ID: strings.Repeat("a", 32), NodeKey: "nodekey:" + strings.Repeat("b", 64), UserID: "1", Expires: 10090}
}
func TestMintAbsoluteSingleUse(t *testing.T) {
	r := record()
	var commands [][]string
	d := Driver{Now: func() time.Time { return time.Unix(10000, 0) }, Run: func(ctx context.Context, a ...string) ([]byte, error) {
		commands = append(commands, a)
		switch strings.Join(a[:2], " ") {
		case "users list":
			return []byte("null"), nil
		case "users create":
			return []byte(`{"id":"1","name":"remoteapp-ticket-` + r.ID + `"}`), nil
		case "preauthkeys create":
			return []byte(`{"id":"2","user":{"id":"1","name":"remoteapp-ticket-` + r.ID + `"},"key":"secret","reusable":false,"expiration":"1970-01-01T02:48:09Z"}`), nil
		}
		return nil, errors.New("bad command")
	}}
	c, e := d.Mint(context.Background(), r)
	if e != nil || c.Secret != "secret" {
		t.Fatal("mint denied", e)
	}
	last := strings.Join(commands[2], " ")
	if strings.Contains(last, "reusable") || !strings.HasSuffix(last, "89s") {
		t.Fatal("not single use/absolute expiry")
	}
}
func TestMintNeverRetriesExistingTicket(t *testing.T) {
	r := record()
	calls := 0
	d := Driver{Now: func() time.Time { return time.Unix(10000, 0) }, Run: func(context.Context, ...string) ([]byte, error) {
		calls++
		return []byte(`[{"id":"1","name":"remoteapp-ticket-` + r.ID + `"}]`), nil
	}}
	if _, e := d.Mint(context.Background(), r); e == nil || calls != 1 {
		t.Fatal("retry")
	}
}
func TestObserveAndProofFailClosed(t *testing.T) {
	r := record()
	registeredKey := r.NodeKey
	d := Driver{Run: func(context.Context, ...string) ([]byte, error) {
		return []byte(`[{"id":"3","user":{"id":"1","name":"remoteapp-ticket-` + r.ID + `"},"node_key":"` + registeredKey + `","ip_addresses":["100.120.0.1","fd7a::1"]}]`), nil
	}}
	o, e := d.Observe(context.Background(), r)
	if e != nil || o.IP != "100.120.0.1" {
		t.Fatal(e)
	}
	if d.Verify(context.Background(), r, o) {
		t.Fatal("row must not be possession proof")
	}
	d.Proof = func(context.Context, invite.ProvisionRecord, invite.ProvisionObservation) bool { return true }
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if d.Verify(ctx, r, o) {
		t.Fatal("cancelled proof")
	}
	r.NodeKey = "nodekey:" + strings.Repeat("c", 64)
	if _, e = d.Observe(context.Background(), r); e == nil {
		t.Fatal("wrong key")
	}
}
func TestCleanupTicketScopeAndOrphan(t *testing.T) {
	r := record()
	r.UserID = ""
	var calls []string
	d := Driver{Run: func(ctx context.Context, a ...string) ([]byte, error) {
		calls = append(calls, strings.Join(a, " "))
		switch strings.Join(a[:2], " ") {
		case "users list":
			return []byte(`[{"id":"7","name":"remoteapp-ticket-` + r.ID + `"},{"id":"8","name":"other"}]`), nil
		case "preauthkeys list":
			return []byte(`[{"id":"9","user":{"id":"7"}},{"id":"10","user":{"id":"8"}}]`), nil
		case "nodes list":
			return []byte(`[{"id":"11","user":{"id":"7"}},{"id":"12","user":{"id":"8"}}]`), nil
		}
		return nil, nil
	}}
	if e := d.Cleanup(context.Background(), r); e != nil {
		t.Fatal(e)
	}
	all := strings.Join(calls, "\n")
	for _, bad := range []string{"--id 10", "--identifier 12", "--identifier 8"} {
		if strings.Contains(all, bad) {
			t.Fatal("unrelated cleanup")
		}
	}
	for _, want := range []string{"--id 9", "--identifier 11", "--identifier 7"} {
		if !strings.Contains(all, want) {
			t.Fatal("missing orphan cleanup")
		}
	}
}
func TestPolicyExplicitEmptyReadbackAndBoundary(t *testing.T) {
	dir := t.TempDir()
	os.Chmod(dir, 0700)
	var policy []byte
	sets := 0
	d := Driver{PolicyMode: "db", PolicyDir: dir, Run: func(ctx context.Context, a ...string) ([]byte, error) {
		if a[1] == "get" {
			return policy, nil
		}
		b, e := os.ReadFile(a[3])
		if e != nil {
			return nil, e
		}
		policy = b
		if a[1] == "set" {
			sets++
		}
		return nil, nil
	}}
	if e := d.ReplaceRules(context.Background(), nil); e != nil || string(policy) != `{"acls":[]}` {
		t.Fatal("not explicit empty", e)
	}
	if e := d.ReplaceRules(context.Background(), []invite.NetworkRule{{Source: "100.120.0.2", Destination: "100.120.0.1:47476"}}); e != nil {
		t.Fatal(e)
	}
	var p map[string]any
	if json.Unmarshal(policy, &p) != nil || sets != 2 {
		t.Fatal("policy")
	}
	for _, r := range []invite.NetworkRule{{Source: "*", Destination: "100.120.0.1:47476"}, {Source: "100.120.0.2", Destination: "100.120.0.1:*"}, {Source: "100.73.4.118", Destination: "100.120.0.1:47476"}} {
		if d.ReplaceRules(context.Background(), []invite.NetworkRule{r}) == nil {
			t.Fatal("unsafe rule")
		}
	}
	d.PolicyMode = "file"
	if d.ReplaceRules(context.Background(), nil) == nil {
		t.Fatal("file mode accepted")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatal("temporary policy leaked")
	}
}

func TestProviderTimestampAndExpiryFailClosed(t *testing.T) {
	var ts wireTime
	if json.Unmarshal([]byte(`{"seconds":10089,"nanos":1}`), &ts) != nil || ts.Unix() != 10089 {
		t.Fatal("protobuf time")
	}
	for _, b := range []string{`null`, `{}`, `{"seconds":10089,"nanos":-1}`, `{"seconds":10089,"nanos":1000000000}`} {
		if json.Unmarshal([]byte(b), &ts) == nil {
			t.Fatal("invalid timestamp")
		}
	}
	r := record()
	for _, expiry := range []string{"1970-01-01T02:48:11Z", "1970-01-01T02:46:40Z"} {
		d := Driver{Now: func() time.Time { return time.Unix(10000, 0) }, Run: func(ctx context.Context, a ...string) ([]byte, error) {
			if a[1] == "list" {
				return []byte("null"), nil
			}
			if a[0] == "users" {
				return []byte(`{"id":1,"name":"remoteapp-ticket-` + r.ID + `"}`), nil
			}
			return []byte(`{"id":2,"user":{"id":1,"name":"remoteapp-ticket-` + r.ID + `"},"key":"secret","expiration":"` + expiry + `"}`), nil
		}}
		if c, e := d.Mint(context.Background(), r); e == nil || c.Secret != "" {
			t.Fatal("unsafe expiry issued")
		}
	}
}
func TestPolicyFailureAndSanitizedProviderErrors(t *testing.T) {
	dir := t.TempDir()
	os.Chmod(dir, 0700)
	d := Driver{PolicyMode: "db", PolicyDir: dir, Run: func(context.Context, ...string) ([]byte, error) { return []byte("secret"), errors.New("secret") }}
	e := d.ReplaceRules(context.Background(), nil)
	if e == nil || strings.Contains(e.Error(), "secret") {
		t.Fatal("secret error")
	}
	d.Run = func(ctx context.Context, a ...string) ([]byte, error) {
		if a[1] == "get" {
			return []byte(`{"acls":[{"action":"accept","src":["*"],"dst":["*:*"]}]}`), nil
		}
		return nil, nil
	}
	if d.ReplaceRules(context.Background(), nil) == nil {
		t.Fatal("readback mismatch")
	}
}
