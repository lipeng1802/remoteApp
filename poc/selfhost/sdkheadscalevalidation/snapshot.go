package sdkheadscalevalidation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"sort"

	"remoteapp.local/selfhost-poc/headscale"
)

// Snapshot stays in memory: Keys includes secrets, never log/serialize it to
// an audit file. Only stable fields are compared; online/last_seen can change.
type Snapshot struct {
	users []User
	nodes []Node
	keys  []Key
}

func Capture(ctx context.Context, run headscale.Run) (Snapshot, error) {
	var s Snapshot
	for _, task := range []struct {
		args []string
		dest any
	}{
		{[]string{"users", "list"}, &s.users}, {[]string{"nodes", "list"}, &s.nodes}, {[]string{"preauthkeys", "list"}, &s.keys},
	} {
		b, e := run(ctx, task.args...)
		if e != nil || ctx.Err() != nil || len(b) > 1<<20 || json.Unmarshal(b, task.dest) != nil {
			return Snapshot{}, ErrValidation
		}
	}
	b, e := run(ctx, "policy", "get")
	var policy map[string]json.RawMessage
	var acls []json.RawMessage
	if e != nil || len(b) > 1<<20 || json.Unmarshal(b, &policy) != nil || len(policy) != 1 || json.Unmarshal(policy["acls"], &acls) != nil || acls == nil || len(acls) != 0 {
		return Snapshot{}, ErrValidation
	}
	for _, u := range s.users {
		if !numeric(u.ID.String()) || u.Name == "" {
			return Snapshot{}, ErrValidation
		}
	}
	for _, n := range s.nodes {
		if !numeric(n.ID.String()) || !numeric(n.User.ID.String()) || n.Key == "" {
			return Snapshot{}, ErrValidation
		}
		sort.Strings(n.IPs)
	}
	for _, k := range s.keys {
		if !numeric(k.ID.String()) || !numeric(k.User.ID.String()) || k.Secret == "" {
			return Snapshot{}, ErrValidation
		}
	}
	sort.Slice(s.users, func(i, j int) bool { return s.users[i].ID.String() < s.users[j].ID.String() })
	sort.Slice(s.nodes, func(i, j int) bool { return s.nodes[i].ID.String() < s.nodes[j].ID.String() })
	sort.Slice(s.keys, func(i, j int) bool { return s.keys[i].ID.String() < s.keys[j].ID.String() })
	return s, nil
}
func (s Snapshot) LongLivedBaseline() bool {
	return len(s.users) == 1 && len(s.nodes) == 1 && len(s.keys) == 1
}
func (s Snapshot) Same(other Snapshot) bool { return reflect.DeepEqual(s, other) }

// Digest only, never serialize the snapshot containing provider keys to disk.
func (s Snapshot) Fingerprint() string {
	b, _ := json.Marshal(struct {
		Users []User
		Nodes []Node
		Keys  []Key
	}{s.users, s.nodes, s.keys})
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func (s Snapshot) WithoutTicket(userID, ticket string) (Snapshot, error) {
	var out Snapshot
	count := 0
	for _, u := range s.users {
		if u.ID.String() == userID || u.Name == "remoteapp-ticket-"+ticket {
			if u.ID.String() != userID || u.Name != "remoteapp-ticket-"+ticket {
				return Snapshot{}, ErrValidation
			}
			count++
		} else {
			out.users = append(out.users, u)
		}
	}
	if count != 1 {
		return Snapshot{}, ErrValidation
	}
	for _, n := range s.nodes {
		if n.User.ID.String() != userID {
			out.nodes = append(out.nodes, n)
		}
	}
	for _, k := range s.keys {
		if k.User.ID.String() != userID {
			out.keys = append(out.keys, k)
		}
	}
	return out, nil
}
