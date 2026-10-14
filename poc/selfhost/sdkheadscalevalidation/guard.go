// Package sdkheadscalevalidation is operator-only acceptance tooling. It is NOT
// a client credential supplier and must never be reachable through product IPC.
package sdkheadscalevalidation

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"remoteapp.local/selfhost-poc/headscale"
)

var ErrValidation = errors.New("scoped Headscale validation unavailable")

type User struct {
	ID   json.Number `json:"id"`
	Name string      `json:"name"`
}
type Node struct {
	ID   json.Number `json:"id"`
	User User        `json:"user"`
	Key  string      `json:"node_key"`
	IPs  []string    `json:"ip_addresses"`
}
type Key struct {
	ID     json.Number `json:"id"`
	User   User        `json:"user"`
	Secret string      `json:"key"`
}

// Handles are public recovery metadata. NEVER add registration secrets here.
type Handles struct{ UserID, KeyID, NodeID string }

// Guard limits ALL mutations, including cleanup, to the current exact ticket.
// Read-only CLI replies can contain secrets and must remain inside this process.
// BeforeMutation must persist the public intent; errors fence further writes.
type Guard struct {
	Run             headscale.Run
	Ticket, NodeKey string
	MaxExpiry       int64
	Handles         Handles
	BeforeMutation  func(string, Handles) error
	failed          bool
	expired         bool
}

func numeric(s string) bool {
	n, e := strconv.ParseUint(s, 10, 64)
	return e == nil && n > 0 && strconv.FormatUint(n, 10) == s
}
func (g *Guard) name() string { return "remoteapp-ticket-" + g.Ticket }
func (g *Guard) valid() bool {
	if len(g.Ticket) != 32 || len(g.NodeKey) != 72 || !strings.HasPrefix(g.NodeKey, "nodekey:") {
		return false
	}
	for _, s := range []string{g.Ticket, g.NodeKey[8:]} {
		for _, c := range s {
			if !strings.ContainsRune("0123456789abcdef", c) {
				return false
			}
		}
	}
	return true
}
func equal(a []string, b ...string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
func (g *Guard) owned(ctx context.Context) error {
	var users []User
	b, e := g.read(ctx, "users", "list")
	if e != nil || json.Unmarshal(b, &users) != nil {
		return ErrValidation
	}
	count := 0
	for _, u := range users {
		if u.ID.String() == g.Handles.UserID || u.Name == g.name() {
			if u.ID.String() != g.Handles.UserID || u.Name != g.name() {
				return ErrValidation
			}
			count++
		}
	}
	if count != 1 {
		return ErrValidation
	}
	return nil
}
func (g *Guard) read(ctx context.Context, args ...string) ([]byte, error) {
	if g.Run == nil || ctx.Err() != nil {
		return nil, ErrValidation
	}
	b, e := g.Run(ctx, args...)
	if e != nil || ctx.Err() != nil || len(b) > 1<<20 {
		return nil, ErrValidation
	}
	return b, nil
}

func (g *Guard) Execute(ctx context.Context, args ...string) ([]byte, error) {
	if !g.valid() || g.Run == nil || ctx.Err() != nil {
		return nil, ErrValidation
	}
	for _, kind := range []string{"users", "nodes", "preauthkeys"} {
		if equal(args, kind, "list") {
			return g.read(ctx, args...)
		}
	}
	if equal(args, "policy", "get") {
		return g.read(ctx, args...)
	}
	if g.failed || g.BeforeMutation == nil {
		return nil, ErrValidation
	}
	stage := ""
	switch {
	case equal(args, "users", "create", g.name()) && g.Handles.UserID == "":
		b, e := g.read(ctx, "users", "list")
		var users []User
		if e != nil || json.Unmarshal(b, &users) != nil {
			return nil, ErrValidation
		}
		for _, u := range users {
			if u.Name == g.name() {
				return nil, ErrValidation
			}
		}
		stage = "user_create"
	case len(args) == 6 && equal(args[:4], "preauthkeys", "create", "--user", g.Handles.UserID) && args[4] == "--expiration" && g.Handles.KeyID == "" && numeric(g.Handles.UserID):
		duration := strings.TrimSuffix(args[5], "s")
		if !strings.HasSuffix(args[5], "s") || !numeric(duration) {
			return nil, ErrValidation
		}
		n, _ := strconv.Atoi(duration)
		if n > 119 || g.owned(ctx) != nil {
			return nil, ErrValidation
		}
		stage = "key_create"
	case equal(args, "preauthkeys", "expire", "--id", g.Handles.KeyID) && numeric(g.Handles.KeyID):
		if g.owned(ctx) != nil || g.checkKey(ctx) != nil {
			return nil, ErrValidation
		}
		stage = "key_expire"
	case equal(args, "nodes", "delete", "--identifier", g.Handles.NodeID, "--force") && numeric(g.Handles.NodeID) && g.expired:
		if g.owned(ctx) != nil || g.checkNode(ctx) != nil {
			return nil, ErrValidation
		}
		stage = "node_delete"
	case equal(args, "users", "destroy", "--identifier", g.Handles.UserID, "--force") && numeric(g.Handles.UserID) && g.expired:
		if g.owned(ctx) != nil || g.checkKey(ctx) != nil {
			return nil, ErrValidation
		}
		b, e := g.read(ctx, "nodes", "list")
		var nodes []Node
		if e != nil || json.Unmarshal(b, &nodes) != nil {
			return nil, ErrValidation
		}
		for _, n := range nodes {
			if n.User.ID.String() == g.Handles.UserID {
				return nil, ErrValidation
			}
		}
		stage = "user_destroy"
	default:
		return nil, ErrValidation // NO policy set/check, service or broad delete.
	}
	if g.BeforeMutation(stage, g.Handles) != nil {
		g.failed = true
		return nil, ErrValidation
	}
	if stage == "key_create" {
		// Extra ownership reads and SSH transport can exceed the local driver's
		// one-second margin. Shorten, never extend, its requested duration.
		remaining := int64(time.Until(time.Unix(g.MaxExpiry, 0))/time.Second) - 5
		requested, _ := strconv.ParseInt(strings.TrimSuffix(args[5], "s"), 10, 64)
		if remaining <= 0 || g.MaxExpiry > time.Now().Add(120*time.Second).Unix() {
			return nil, ErrValidation
		}
		if remaining < requested {
			args = append([]string(nil), args...)
			args[5] = strconv.FormatInt(remaining, 10) + "s"
		}
	}
	b, e := g.read(ctx, args...)
	if e != nil {
		g.failed = true
		return nil, ErrValidation
	} // unknown write: never retry/auto-clean
	if stage == "user_create" {
		var u User
		if json.Unmarshal(b, &u) != nil || u.Name != g.name() || !numeric(u.ID.String()) {
			g.failed = true
			return nil, ErrValidation
		}
		g.Handles.UserID = u.ID.String()
	}
	if stage == "key_create" {
		var k Key
		if json.Unmarshal(b, &k) != nil || k.User.Name != g.name() || k.User.ID.String() != g.Handles.UserID || !numeric(k.ID.String()) {
			g.failed = true
			return nil, ErrValidation
		}
		g.Handles.KeyID = k.ID.String()
	}
	if stage == "key_expire" {
		g.expired = true
	}
	if g.BeforeMutation(stage+"_acknowledged", g.Handles) != nil {
		g.failed = true
		return nil, ErrValidation
	}
	return b, nil
}
func (g *Guard) checkKey(ctx context.Context) error {
	b, e := g.read(ctx, "preauthkeys", "list")
	var keys []Key
	if e != nil || json.Unmarshal(b, &keys) != nil {
		return ErrValidation
	}
	count := 0
	for _, k := range keys {
		if k.User.ID.String() == g.Handles.UserID {
			if k.ID.String() != g.Handles.KeyID || k.User.Name != g.name() {
				return ErrValidation
			}
			count++
		}
	}
	if count != 1 {
		return ErrValidation
	}
	return nil
}
func (g *Guard) checkNode(ctx context.Context) error {
	b, e := g.read(ctx, "nodes", "list")
	var nodes []Node
	if e != nil || json.Unmarshal(b, &nodes) != nil {
		return ErrValidation
	}
	count := 0
	for _, n := range nodes {
		if n.User.ID.String() == g.Handles.UserID {
			if n.ID.String() != g.Handles.NodeID || n.User.Name != g.name() || n.Key != g.NodeKey {
				return ErrValidation
			}
			count++
		}
	}
	if count != 1 {
		return ErrValidation
	}
	return nil
}
func (g *Guard) RememberNode(ctx context.Context, id string) error {
	if g.failed || !numeric(id) || (g.Handles.NodeID != "" && g.Handles.NodeID != id) {
		return ErrValidation
	}
	g.Handles.NodeID = id
	if g.checkNode(ctx) != nil {
		g.failed = true
		return ErrValidation
	}
	if g.BeforeMutation == nil || g.BeforeMutation("node_observed", g.Handles) != nil {
		g.failed = true
		return ErrValidation
	}
	return nil
}
