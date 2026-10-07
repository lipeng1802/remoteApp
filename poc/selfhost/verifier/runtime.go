// Package verifier owns a persistent, isolated server-side overlay verifier.
// It never projects payload ACLs or approves devices. A live verifier is a
// prerequisite for a future privileged backend, not an authorization itself.
package verifier

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"remoteapp.local/selfhost-poc/invite"
	"remoteapp.local/selfhost-poc/nodeidentity"
	"remoteapp.local/selfhost-poc/privatepipe"
)

var ErrVerifier = errors.New("verifier_unavailable")

const Control = "https://mk.fengmap.com:8443"

type Driver interface {
	Mint(context.Context, invite.ProvisionRecord) (invite.RegistrationCredential, error)
	Observe(context.Context, invite.ProvisionRecord) (invite.ProvisionObservation, error)
	Cleanup(context.Context, invite.ProvisionRecord) error
}
type Info struct {
	Key, IP string
	Healthy bool
}
type Node interface {
	Up(context.Context, string) (Info, error)
	Status(context.Context) (Info, error)
	Close() error
}
type Factory func(string, string) Node
type snapshot struct {
	Version        int
	Control, Phase string
	Record         invite.ProvisionRecord
	Observation    invite.ProvisionObservation
}
type Runtime struct {
	cancel context.CancelFunc
	done   chan struct{}
	live   atomic.Bool
	mu     sync.Mutex
	err    error
	info   Info
}

func (r *Runtime) Live() bool { return r.live.Load() }
func (r *Runtime) Info() Info {
	i := r.info
	i.Healthy = r.live.Load()
	return i
}
func (r *Runtime) Done() <-chan struct{} { return r.done }
func (r *Runtime) Stop(ctx context.Context) error {
	r.live.Store(false)
	r.cancel()
	select {
	case <-r.done:
		r.mu.Lock()
		defer r.mu.Unlock()
		return r.err
	case <-ctx.Done():
		return ErrVerifier
	}
}
func save(dir string, s snapshot) error {
	b, e := json.Marshal(s)
	if e != nil {
		return ErrVerifier
	}
	f, e := os.CreateTemp(dir, "journal-*.tmp")
	if e != nil {
		return ErrVerifier
	}
	name := f.Name()
	defer os.Remove(name)
	_, e = f.Write(b)
	if e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e != nil || ce != nil {
		return ErrVerifier
	}
	if os.Rename(name, filepath.Join(dir, "journal.json")) != nil {
		return ErrVerifier
	}
	d, e := os.Open(dir)
	if e != nil {
		return ErrVerifier
	}
	defer d.Close()
	if d.Sync() != nil {
		return ErrVerifier
	}
	return nil
}
func load(dir string) (snapshot, error) {
	var s snapshot
	path := filepath.Join(dir, "journal.json")
	i, e := os.Lstat(path)
	if e != nil || !i.Mode().IsRegular() || i.Mode().Perm()&0077 != 0 || i.Size() > 8191 {
		return s, ErrVerifier
	}
	b, e := os.ReadFile(path)
	if e != nil {
		return s, ErrVerifier
	}
	reader := privatepipe.NewReader(strings.NewReader(string(b) + "\n"))
	var extra struct{}
	if reader.Read(&s) != nil || reader.Read(&extra) != io.EOF {
		return s, ErrVerifier
	}
	id, e := hex.DecodeString(s.Record.ID)
	if e != nil || len(id) != 16 || hex.EncodeToString(id) != s.Record.ID || s.Version != 1 || s.Control != Control || !strings.HasPrefix(s.Record.NodeKey, "nodekey:") || len(s.Record.NodeKey) != 72 || s.Record.Expires <= 0 {
		return s, ErrVerifier
	}
	key, e := hex.DecodeString(s.Record.NodeKey[8:])
	if e != nil || len(key) != 32 || hex.EncodeToString(key) != s.Record.NodeKey[8:] || s.Record.NodeKey == "nodekey:"+strings.Repeat("0", 64) {
		return s, ErrVerifier
	}
	switch s.Phase {
	case "planned":
		if s.Record.UserID != "" || s.Record.KeyID != "" {
			return s, ErrVerifier
		}
	case "issued", "ready":
		if !handle(s.Record.UserID) || !handle(s.Record.KeyID) {
			return s, ErrVerifier
		}
	case "retired":
		if (s.Record.UserID != "" || s.Record.KeyID != "") && (!handle(s.Record.UserID) || !handle(s.Record.KeyID)) {
			return s, ErrVerifier
		}
	default:
		return s, ErrVerifier
	}
	if s.Record.Grant != (invite.Grant{}) || s.Record.Device != "" || s.Record.Status != "" || s.Record.NodeID != "" || s.Record.IP != "" {
		return s, ErrVerifier
	}
	if s.Phase == "ready" && (!validInfo(Info{s.Observation.NodeKey, s.Observation.IP, true}, s.Record.NodeKey) || s.Observation.UserID != s.Record.UserID || !handle(s.Observation.NodeID)) {
		return s, ErrVerifier
	}
	return s, nil
}
func handle(s string) bool {
	if s == "" || len(s) > 64 {
		return false
	}
	for _, c := range s {
		if !(c >= '0' && c <= '9') {
			return false
		}
	}
	return s != "0" && s[0] != '0'
}
func lease(dir string, create bool) (func() error, error) {
	if !filepath.IsAbs(dir) || filepath.Clean(dir) != dir {
		return nil, ErrVerifier
	}
	p, e := filepath.EvalSymlinks(filepath.Dir(dir))
	if e != nil || p != filepath.Dir(dir) {
		return nil, ErrVerifier
	}
	if create && os.Mkdir(dir, 0700) != nil {
		return nil, ErrVerifier
	}
	i, e := os.Lstat(dir)
	if e != nil || !i.IsDir() || i.Mode()&os.ModeSymlink != 0 || i.Mode().Perm()&0077 != 0 {
		return nil, ErrVerifier
	}
	if create {
		entries, e := os.ReadDir(dir)
		if e != nil || len(entries) != 0 {
			return nil, ErrVerifier
		}
	}
	f, e := os.OpenFile(filepath.Join(dir, "owner.lock"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return nil, ErrVerifier
	}
	return func() error {
		e := f.Close()
		re := os.Remove(filepath.Join(dir, "owner.lock"))
		if e != nil || re != nil {
			return ErrVerifier
		}
		return nil
	}, nil
}
func validInfo(i Info, key string) bool {
	ip, e := netip.ParseAddr(i.IP)
	return i.Healthy && i.Key == key && e == nil && ip.String() == i.IP && netip.MustParsePrefix("100.120.0.0/24").Contains(ip) && i.IP != "100.120.0.0" && i.IP != "100.120.0.255"
}
func observation(d Driver, ctx context.Context, s snapshot, i Info) (invite.ProvisionObservation, error) {
	if !validInfo(i, s.Record.NodeKey) {
		return invite.ProvisionObservation{}, ErrVerifier
	}
	o, e := d.Observe(ctx, s.Record)
	if e != nil || !validInfo(i, s.Record.NodeKey) || o.NodeKey != i.Key || o.IP != i.IP || o.UserID != s.Record.UserID || !handle(o.NodeID) {
		return o, ErrVerifier
	}
	if s.Phase == "ready" && o != s.Observation {
		return o, ErrVerifier
	}
	return o, nil
}

// Open requires explicit create vs resume. Mint intent is journaled first;
// unknown outcomes/failed registration NEVER trigger an automatic second Mint.
// Registration credentials exist only in this call's memory, not the journal.
func Open(ctx context.Context, dir string, create bool, d Driver, f Factory) (*Runtime, error) {
	if d == nil || f == nil || ctx.Err() != nil {
		return nil, ErrVerifier
	}
	release, e := lease(dir, create)
	if e != nil {
		return nil, e
	}
	owned := true
	defer func() {
		if owned {
			release()
		}
	}()
	var s snapshot
	if !create {
		s, e = load(dir)
		if e != nil || s.Phase != "issued" && s.Phase != "ready" {
			return nil, ErrVerifier
		}
	}
	nodeDir := filepath.Join(dir, "node")
	if !create {
		if _, e = os.Lstat(filepath.Join(nodeDir, "tailscaled.state")); e != nil {
			return nil, ErrVerifier
		}
	}
	key, e := nodeidentity.Prepare(nodeDir, Control)
	if e != nil {
		return nil, ErrVerifier
	}
	secret := ""
	if create {
		id := make([]byte, 16)
		if _, e = rand.Read(id); e != nil {
			return nil, ErrVerifier
		}
		s = snapshot{Version: 1, Control: Control, Phase: "planned", Record: invite.ProvisionRecord{ID: hex.EncodeToString(id), NodeKey: key, Expires: time.Now().Unix() + 120}}
		if save(dir, s) != nil {
			return nil, ErrVerifier
		}
		mint, cancel := context.WithTimeout(ctx, 10*time.Second)
		credential, e := d.Mint(mint, s.Record)
		cancel()
		if e != nil || !handle(credential.UserID) || !handle(credential.KeyID) || credential.Secret == "" || len(credential.Secret) > 256 {
			return nil, ErrVerifier
		}
		s.Record.UserID = credential.UserID
		s.Record.KeyID = credential.KeyID
		s.Phase = "issued"
		if save(dir, s) != nil {
			return nil, ErrVerifier
		}
		secret = credential.Secret
	} else if key != s.Record.NodeKey {
		return nil, ErrVerifier
	}
	n := f(nodeDir, key)
	if n == nil {
		return nil, ErrVerifier
	}
	closeNode := true
	defer func() {
		if closeNode {
			n.Close()
		}
	}()
	up, cancel := context.WithTimeout(ctx, 30*time.Second)
	info, e := n.Up(up, secret)
	secret = ""
	if e != nil || !validInfo(info, s.Record.NodeKey) {
		cancel()
		return nil, ErrVerifier
	}
	o, e := observation(d, up, s, info)
	cancel()
	if e != nil || ctx.Err() != nil {
		return nil, ErrVerifier
	}
	s.Observation = o
	s.Phase = "ready"
	if save(dir, s) != nil {
		return nil, ErrVerifier
	}
	run, stop := context.WithCancel(ctx)
	r := &Runtime{cancel: stop, done: make(chan struct{}), info: info}
	r.live.Store(true)
	owned = false
	closeNode = false
	go func() {
		defer close(r.done)
		defer func() {
			r.live.Store(false)
			ce := n.Close()
			le := release()
			if ce != nil || le != nil {
				r.mu.Lock()
				r.err = ErrVerifier
				r.mu.Unlock()
			}
		}()
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-run.Done():
				return
			case <-ticker.C:
				check, cancel := context.WithTimeout(run, 3*time.Second)
				i, e := n.Status(check)
				if e == nil {
					_, e = observation(d, check, s, i)
				}
				cancel()
				if run.Err() != nil {
					return
				}
				if e != nil {
					r.live.Store(false)
					r.mu.Lock()
					r.err = ErrVerifier
					r.mu.Unlock()
					stop()
					return
				}
			}
		}
	}()
	return r, nil
}

// Retire requires exclusive ownership and reclaims only the durable ticket.
// Existing network backends must be fenced/stopped BEFORE retiring a verifier.
// Missing/corrupt journals are not authority to delete any provider resource.
func Retire(ctx context.Context, dir string, d Driver) (result error) {
	if d == nil || ctx.Err() != nil {
		return ErrVerifier
	}
	release, e := lease(dir, false)
	if e != nil {
		return e
	}
	defer func() {
		if release() != nil {
			result = ErrVerifier
		}
	}()
	s, e := load(dir)
	if e != nil {
		return e
	}
	if s.Phase == "retired" {
		return nil
	}
	cleanup, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if d.Cleanup(cleanup, s.Record) != nil {
		return ErrVerifier
	}
	// Planned/unknown Mint may have no reliable handles. Cleanup discovers its
	// own ticket by name; the terminal journal preserves that uncertainty.
	s.Phase = "retired"
	return save(dir, s)
}
