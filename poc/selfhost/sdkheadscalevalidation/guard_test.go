package sdkheadscalevalidation

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"
)

func fixtureGuard() (*Guard, *int) {
	writes := new(int)
	g := &Guard{Ticket: strings.Repeat("1", 32), NodeKey: "nodekey:" + strings.Repeat("2", 64), BeforeMutation: func(string, Handles) error { return nil }}
	g.MaxExpiry = time.Now().Add(100 * time.Second).Unix()
	g.Run = func(ctx context.Context, args ...string) ([]byte, error) {
		u := User{ID: "7", Name: g.name()}
		var value any
		switch {
		case equal(args, "users", "list"):
			if g.Handles.UserID == "" {
				value = []User{}
			} else {
				value = []User{u}
			}
		case equal(args, "preauthkeys", "list"):
			value = []Key{{ID: "8", User: u, Secret: "fixture-not-real"}}
		case equal(args, "nodes", "list"):
			if g.Handles.NodeID == "" {
				value = []Node{}
			} else {
				value = []Node{{ID: "9", User: u, Key: g.NodeKey, IPs: []string{"100.120.0.9"}}}
			}
		case equal(args, "policy", "get"):
			value = map[string]any{"acls": []any{}}
		case args[0] == "users" && args[1] == "create":
			*writes++
			value = u
		case args[0] == "preauthkeys" && args[1] == "create":
			*writes++
			value = Key{ID: "8", User: u, Secret: "fixture-not-real"}
		default:
			*writes++
			value = map[string]any{}
		}
		return json.Marshal(value)
	}
	return g, writes
}
func TestGuardRefusesBroadForeignAndUnjournaledWrites(t *testing.T) {
	for _, args := range [][]string{
		{"policy", "set", "--file", "anything"}, {"policy", "check", "--file", "anything"},
		{"users", "create", "foreign"}, {"users", "destroy", "--identifier", "1", "--force"},
		{"nodes", "delete", "--identifier", "1", "--force"}, {"preauthkeys", "expire", "--id", "1"},
	} {
		g, w := fixtureGuard()
		if _, e := g.Execute(context.Background(), args...); e == nil || *w != 0 {
			t.Fatal("foreign mutation accepted")
		}
	}
	g, w := fixtureGuard()
	g.BeforeMutation = nil
	if _, e := g.Execute(context.Background(), "users", "create", g.name()); e == nil || *w != 0 {
		t.Fatal("unjournaled mutation accepted")
	}
}
func TestGuardUnknownMutationFencesCleanupAndRetry(t *testing.T) {
	g, w := fixtureGuard()
	original := g.Run
	g.Run = func(ctx context.Context, args ...string) ([]byte, error) {
		if equal(args, "users", "create", g.name()) {
			*w++
			return nil, ErrValidation
		}
		return original(ctx, args...)
	}
	if _, e := g.Execute(context.Background(), "users", "create", g.name()); e == nil {
		t.Fatal("unknown write passed")
	}
	g.Handles = Handles{UserID: "7", KeyID: "8", NodeID: "9"}
	g.expired = true
	for _, args := range [][]string{{"users", "create", g.name()}, {"preauthkeys", "expire", "--id", "8"}, {"nodes", "delete", "--identifier", "9", "--force"}, {"users", "destroy", "--identifier", "7", "--force"}} {
		if _, e := g.Execute(context.Background(), args...); e == nil {
			t.Fatal("unknown fence bypass")
		}
	}
	if *w != 1 {
		t.Fatal("unknown mutation retried")
	}
}
func TestGuardScopedOrderAndObservation(t *testing.T) {
	g, w := fixtureGuard()
	ctx := context.Background()
	if _, e := g.Execute(ctx, "users", "create", g.name()); e != nil {
		t.Fatal("create own user")
	}
	if _, e := g.Execute(ctx, "preauthkeys", "create", "--user", "7", "--expiration", "120s"); e == nil {
		t.Fatal("excessive expiry accepted")
	}
	if _, e := g.Execute(ctx, "preauthkeys", "create", "--user", "7", "--expiration", "90s"); e != nil {
		t.Fatal("create own key")
	}
	if _, e := g.Execute(ctx, "users", "destroy", "--identifier", "7", "--force"); e == nil {
		t.Fatal("destroy before expiry")
	}
	if g.RememberNode(ctx, "9") != nil {
		t.Fatal("observe own node")
	}
	if _, e := g.Execute(ctx, "nodes", "delete", "--identifier", "9", "--force"); e == nil {
		t.Fatal("delete before expiry")
	}
	if _, e := g.Execute(ctx, "preauthkeys", "expire", "--id", "8"); e != nil {
		t.Fatal("expire own key")
	}
	if _, e := g.Execute(ctx, "nodes", "delete", "--identifier", "9", "--force"); e != nil {
		t.Fatal("delete own node")
	}
	// Fixture still reports node: destroy must refuse until actual readback empty.
	if _, e := g.Execute(ctx, "users", "destroy", "--identifier", "7", "--force"); e == nil {
		t.Fatal("destroy populated user")
	}
	g.Handles.NodeID = ""
	if _, e := g.Execute(ctx, "users", "destroy", "--identifier", "7", "--force"); e != nil {
		t.Fatal("destroy empty own user")
	}
	if *w != 5 {
		t.Fatal("unexpected mutation count")
	}
}
func TestGuardExtraOwnedKeyOrWrongNodeRejectsRetirement(t *testing.T) {
	for _, wrongNode := range []bool{false, true} {
		g, w := fixtureGuard()
		g.Handles = Handles{UserID: "7", KeyID: "8", NodeID: "9"}
		g.expired = true
		original := g.Run
		g.Run = func(ctx context.Context, args ...string) ([]byte, error) {
			u := User{ID: "7", Name: g.name()}
			if !wrongNode && equal(args, "preauthkeys", "list") {
				return json.Marshal([]Key{{ID: "8", User: u}, {ID: "10", User: u}})
			}
			if wrongNode && equal(args, "nodes", "list") {
				return json.Marshal([]Node{{ID: "9", User: u, Key: "nodekey:" + strings.Repeat("3", 64)}})
			}
			return original(ctx, args...)
		}
		args := []string{"preauthkeys", "expire", "--id", "8"}
		if wrongNode {
			args = []string{"nodes", "delete", "--identifier", "9", "--force"}
		}
		if _, e := g.Execute(context.Background(), args...); e == nil || *w != 0 {
			t.Fatal("unexpected owned resource retired")
		}
	}
}
func TestSnapshotRejectsNonzeroOrUnknownPolicy(t *testing.T) {
	for _, policy := range []string{`{"acls":[{}]}`, `{"acls":null}`, `{"acls":[],"extra":true}`} {
		g, _ := fixtureGuard()
		original := g.Run
		g.Run = func(ctx context.Context, args ...string) ([]byte, error) {
			if equal(args, "policy", "get") {
				return []byte(policy), nil
			}
			return original(ctx, args...)
		}
		if _, e := Capture(context.Background(), g.Run); e == nil {
			t.Fatal("nonzero/unknown policy accepted")
		}
	}
}

func TestGuardShortensExpiryAfterOwnershipChecks(t *testing.T) {
	g, w := fixtureGuard()
	g.Handles.UserID = "7"
	g.MaxExpiry = time.Now().Add(10 * time.Second).Unix()
	original := g.Run
	duration := int64(0)
	g.Run = func(ctx context.Context, args ...string) ([]byte, error) {
		if len(args) == 6 && args[0] == "preauthkeys" && args[1] == "create" {
			duration, _ = strconv.ParseInt(strings.TrimSuffix(args[5], "s"), 10, 64)
		}
		return original(ctx, args...)
	}
	if _, e := g.Execute(context.Background(), "preauthkeys", "create", "--user", "7", "--expiration", "90s"); e != nil || *w != 1 || duration <= 0 || duration > 5 {
		t.Fatal("expiry not shortened with safety margin")
	}
	g, w = fixtureGuard()
	g.Handles.UserID = "7"
	g.MaxExpiry = time.Now().Add(4 * time.Second).Unix()
	if _, e := g.Execute(context.Background(), "preauthkeys", "create", "--user", "7", "--expiration", "90s"); e == nil || *w != 0 {
		t.Fatal("expired budget minted")
	}
}

func TestSnapshotTicketRemovalPreservesOriginalFingerprint(t *testing.T) {
	baseline := Snapshot{users: []User{{ID: "1", Name: "long-lived"}}, nodes: []Node{{ID: "2", User: User{ID: "1", Name: "long-lived"}, Key: "original"}}, keys: []Key{{ID: "3", User: User{ID: "1", Name: "long-lived"}, Secret: "public fixture"}}}
	current := Snapshot{users: append(append([]User(nil), baseline.users...), User{ID: "7", Name: "remoteapp-ticket-" + strings.Repeat("1", 32)}), nodes: baseline.nodes, keys: baseline.keys}
	filtered, e := current.WithoutTicket("7", strings.Repeat("1", 32))
	if e != nil || filtered.Fingerprint() != baseline.Fingerprint() {
		t.Fatal("baseline removal mismatch")
	}
	if _, e = current.WithoutTicket("1", strings.Repeat("1", 32)); e == nil {
		t.Fatal("original resource hidden by filter")
	}
}
