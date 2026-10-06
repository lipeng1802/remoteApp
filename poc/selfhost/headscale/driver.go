// Package headscale implements the privileged, server-local provisioning driver.
// It must never be exposed as a public HTTP handler.
package headscale

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"remoteapp.local/selfhost-poc/invite"
)

var ErrDriver = errors.New("headscale operation failed")

// Run is a trusted administrator transport. Never return its raw output in logs:
// preauthkeys JSON contains secrets. Arguments never contain registration keys.
type Run func(context.Context, ...string) ([]byte, error)
type LiveProof func(context.Context, invite.ProvisionRecord, invite.ProvisionObservation) bool

type Driver struct {
	Run Run
	// PolicyDir is private (0700), on the same server as the CLI. PolicyMode must
	// be db: file mode cannot acknowledge an atomic live policy replacement.
	PolicyDir, PolicyMode string
	// Proof must independently authenticate a live tsnet peer AND its app device.
	// A control-plane row, helper self-report or an IP alone is not a proof.
	Proof LiveProof
	Now   func() time.Time
	mu    sync.Mutex
}

// LocalRunner is deliberately pinned to the isolated PoC installation.
func LocalRunner() Run {
	return func(ctx context.Context, args ...string) ([]byte, error) {
		args = append([]string{"-c", "/etc/remoteapp-poc/config.yaml", "-o", "json"}, args...)
		cmd := exec.CommandContext(ctx, "/opt/remoteapp-poc/headscale", args...)
		var out limitedBuffer
		cmd.Stdout = &out
		// Do not propagate stderr: provider errors can include secret fields.
		if cmd.Run() != nil {
			return nil, ErrDriver
		}
		return out.Bytes(), nil
	}
}

type limitedBuffer struct{ bytes.Buffer }

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > 1<<20 {
		return 0, ErrDriver
	}
	return b.Buffer.Write(p)
}
func (d *Driver) call(ctx context.Context, dest any, args ...string) error {
	if d.Run == nil || ctx.Err() != nil {
		return ErrDriver
	}
	b, err := d.Run(ctx, args...)
	if err != nil || ctx.Err() != nil || len(b) > 1<<20 {
		return ErrDriver
	}
	if dest != nil && json.Unmarshal(b, dest) != nil {
		return ErrDriver
	}
	return nil
}
func (d *Driver) now() time.Time {
	if d.Now != nil {
		return d.Now()
	}
	return time.Now()
}
func ticket(r invite.ProvisionRecord) (string, error) {
	if len(r.ID) != 32 {
		return "", ErrDriver
	}
	for _, c := range r.ID {
		if !strings.ContainsRune("0123456789abcdef", c) {
			return "", ErrDriver
		}
	}
	return "remoteapp-ticket-" + r.ID, nil
}

type user struct {
	ID   json.Number `json:"id"`
	Name string      `json:"name"`
}
type authKey struct {
	ID         json.Number `json:"id"`
	User       user        `json:"user"`
	Key        string      `json:"key"`
	Reusable   bool        `json:"reusable"`
	Expiration wireTime    `json:"expiration"`
}

// Headscale's CLI uses encoding/json on protobuf messages, hence Timestamp is
// {seconds,nanos}, not necessarily the RFC3339 used by its HTTP API.
type wireTime struct{ time.Time }

func (t *wireTime) UnmarshalJSON(b []byte) error {
	if len(b) > 0 && b[0] == '"' {
		return json.Unmarshal(b, &t.Time)
	}
	var v struct {
		Seconds json.Number `json:"seconds"`
		Nanos   int64       `json:"nanos"`
	}
	if json.Unmarshal(b, &v) != nil {
		return ErrDriver
	}
	s, e := v.Seconds.Int64()
	if e != nil || s <= 0 || v.Nanos < 0 || v.Nanos >= 1e9 {
		return ErrDriver
	}
	t.Time = time.Unix(s, v.Nanos)
	return nil
}

type node struct {
	ID   json.Number `json:"id"`
	User user        `json:"user"`
	Key  string      `json:"node_key"`
	IPs  []string    `json:"ip_addresses"`
}

func numeric(n string) bool {
	v, e := strconv.ParseUint(n, 10, 64)
	return e == nil && v > 0 && strconv.FormatUint(v, 10) == n
}

func (d *Driver) Mint(ctx context.Context, r invite.ProvisionRecord) (invite.RegistrationCredential, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	name, err := ticket(r)
	if err != nil || r.Expires <= d.now().Unix() || r.Expires > d.now().Unix()+120 {
		return invite.RegistrationCredential{}, ErrDriver
	}
	var users []user
	if d.call(ctx, &users, "users", "list") != nil {
		return invite.RegistrationCredential{}, ErrDriver
	}
	for _, u := range users {
		if u.Name == name {
			return invite.RegistrationCredential{}, ErrDriver
		}
	} // never retry an unknown mint
	var u user
	if d.call(ctx, &u, "users", "create", name) != nil || !numeric(u.ID.String()) || u.Name != name {
		return invite.RegistrationCredential{}, ErrDriver
	}
	remaining := time.Until(time.Unix(r.Expires, 0))
	if d.Now != nil {
		remaining = time.Unix(r.Expires, 0).Sub(d.now())
	}
	// CLI accepts duration only. Round down with a safety second, then verify
	// the provider's absolute expiry before returning the secret. If CLI latency
	// or clock skew extends it, the durable intent must be cleaned, never issued.
	seconds := int64(remaining/time.Second) - 1
	if seconds <= 0 {
		return invite.RegistrationCredential{}, ErrDriver
	}
	var k authKey
	if d.call(ctx, &k, "preauthkeys", "create", "--user", u.ID.String(), "--expiration", strconv.FormatInt(seconds, 10)+"s") != nil ||
		!numeric(k.ID.String()) || k.User.ID != u.ID || k.User.Name != name || k.Reusable || k.Expiration.After(time.Unix(r.Expires, 0)) || !k.Expiration.After(d.now()) || k.Key == "" || len(k.Key) > 256 {
		return invite.RegistrationCredential{}, ErrDriver
	}
	return invite.RegistrationCredential{UserID: u.ID.String(), KeyID: k.ID.String(), Secret: k.Key}, nil
}
func (d *Driver) Observe(ctx context.Context, r invite.ProvisionRecord) (invite.ProvisionObservation, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	name, err := ticket(r)
	if err != nil || !numeric(r.UserID) {
		return invite.ProvisionObservation{}, ErrDriver
	}
	var nodes []node
	if d.call(ctx, &nodes, "nodes", "list") != nil {
		return invite.ProvisionObservation{}, ErrDriver
	}
	var result invite.ProvisionObservation
	count := 0
	for _, n := range nodes {
		if n.User.ID.String() != r.UserID {
			continue
		}
		if n.User.Name != name || n.Key != r.NodeKey || !numeric(n.ID.String()) {
			return invite.ProvisionObservation{}, ErrDriver
		}
		ip := ""
		for _, s := range n.IPs {
			a, e := netip.ParseAddr(s)
			if e == nil && a.Is4() && netip.MustParsePrefix("100.120.0.0/24").Contains(a) && a.String() == s && s != "100.120.0.0" && s != "100.120.0.255" {
				if ip != "" {
					return invite.ProvisionObservation{}, ErrDriver
				}
				ip = s
			}
		}
		if ip == "" {
			return invite.ProvisionObservation{}, ErrDriver
		}
		count++
		result = invite.ProvisionObservation{UserID: r.UserID, NodeID: n.ID.String(), NodeKey: n.Key, IP: ip}
	}
	if count != 1 {
		return invite.ProvisionObservation{}, ErrDriver
	}
	return result, nil
}
func (d *Driver) Verify(ctx context.Context, r invite.ProvisionRecord, o invite.ProvisionObservation) bool {
	if ctx.Err() != nil || d.Proof == nil || o.UserID != r.UserID || o.NodeKey != r.NodeKey {
		return false
	}
	return d.Proof(ctx, r, o) && ctx.Err() == nil
}
func (d *Driver) Cleanup(ctx context.Context, r invite.ProvisionRecord) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	name, err := ticket(r)
	if err != nil {
		return ErrDriver
	}
	var users []user
	if d.call(ctx, &users, "users", "list") != nil {
		return ErrDriver
	}
	for _, u := range users {
		if u.Name != name {
			continue
		}
		if !numeric(u.ID.String()) {
			return ErrDriver
		}
		var keys []authKey
		var nodes []node
		if d.call(ctx, &keys, "preauthkeys", "list") != nil || d.call(ctx, &nodes, "nodes", "list") != nil {
			return ErrDriver
		}
		for _, k := range keys {
			if k.User.ID == u.ID {
				if !numeric(k.ID.String()) || d.call(ctx, nil, "preauthkeys", "expire", "--id", k.ID.String()) != nil {
					return ErrDriver
				}
			}
		}
		for _, n := range nodes {
			if n.User.ID == u.ID {
				if !numeric(n.ID.String()) || d.call(ctx, nil, "nodes", "delete", "--identifier", n.ID.String(), "--force") != nil {
					return ErrDriver
				}
			}
		}
		if d.call(ctx, nil, "users", "destroy", "--identifier", u.ID.String(), "--force") != nil {
			return ErrDriver
		}
	}
	return nil
}
func (d *Driver) ReplaceRules(ctx context.Context, rules []invite.NetworkRule) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.PolicyMode != "db" || !filepath.IsAbs(d.PolicyDir) {
		return ErrDriver
	}
	st, e := os.Lstat(d.PolicyDir)
	if e != nil || !st.IsDir() || st.Mode().Perm() != 0700 || st.Mode()&os.ModeSymlink != 0 {
		return ErrDriver
	}
	type acl struct {
		Action string   `json:"action"`
		Src    []string `json:"src"`
		Dst    []string `json:"dst"`
	}
	policy := struct {
		ACLs []acl `json:"acls"`
	}{ACLs: []acl{}}
	if len(rules) > 256 {
		return ErrDriver
	}
	for _, r := range rules {
		if !strings.HasSuffix(r.Destination, ":47476") {
			return ErrDriver
		}
		dst := strings.TrimSuffix(r.Destination, ":47476")
		for _, s := range []string{r.Source, dst} {
			a, e := netip.ParseAddr(s)
			if e != nil || a.String() != s || !netip.MustParsePrefix("100.120.0.0/24").Contains(a) || s == "100.120.0.0" || s == "100.120.0.255" {
				return ErrDriver
			}
		}
		if r.Source == dst {
			return ErrDriver
		}
		policy.ACLs = append(policy.ACLs, acl{"accept", []string{r.Source}, []string{r.Destination}})
	}
	b, _ := json.Marshal(policy)
	f, e := os.CreateTemp(d.PolicyDir, "policy-*.json")
	if e != nil {
		return ErrDriver
	}
	path := f.Name()
	defer os.Remove(path)
	if _, e = f.Write(b); e != nil {
		f.Close()
		return ErrDriver
	}
	if e = f.Close(); e != nil {
		return ErrDriver
	}
	if d.call(ctx, nil, "policy", "check", "--file", path) != nil || d.call(ctx, nil, "policy", "set", "--file", path) != nil {
		return ErrDriver
	}
	var raw json.RawMessage
	if d.call(ctx, &raw, "policy", "get") != nil {
		return ErrDriver
	}
	var got struct {
		ACLs []acl `json:"acls"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&got) != nil {
		return ErrDriver
	}
	want, _ := json.Marshal(policy)
	actual, _ := json.Marshal(got)
	if !bytes.Equal(want, actual) {
		return fmt.Errorf("%w: policy readback mismatch", ErrDriver)
	}
	return nil
}
