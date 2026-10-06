// Package invite is an isolated authorization core, not a public HTTP service.
package invite

import (
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var ErrDenied = errors.New("authorization_denied")
var ErrStorage = errors.New("authorization_storage_unavailable")

const maxState = 1 << 20
const permission = "fixed-payload"

// Signed requests bind every field, action, timestamp and nonce to a device key.
type Body struct {
	Action     string `json:"action"`
	Nonce      string `json:"nonce"`
	IssuedAt   int64  `json:"issued_at"`
	TargetCode string `json:"target_code"`
	InviteID   string `json:"invite_id"`
	Code       string `json:"code"`
	RequestID  string `json:"request_id"`
	GrantID    string `json:"grant_id"`
	Allow      bool   `json:"allow"`
}
type Envelope struct {
	PublicKey string `json:"public_key"`
	Body      Body   `json:"body"`
	Signature string `json:"signature"`
}
type Claims struct {
	Version    int    `json:"version"`
	ID         string `json:"id"`
	Target     string `json:"target"`
	Controller string `json:"controller"`
	Permission string `json:"permission"`
	IssuedAt   int64  `json:"issued_at"`
	ExpiresAt  int64  `json:"expires_at"`
}
type Grant struct {
	Claims    Claims `json:"claims"`
	Signature string `json:"signature"`
}
type Result struct {
	DeviceID   string `json:"device_id,omitempty"`
	DeviceCode string `json:"device_code,omitempty"`
	InviteID   string `json:"invite_id,omitempty"`
	Code       string `json:"code,omitempty"` // returned only once to the owning device
	RequestID  string `json:"request_id,omitempty"`
	Status     string `json:"status"`
	Grant      *Grant `json:"grant,omitempty"`
}
type device struct{ Code, ActiveInvite string }
type invitation struct {
	Owner, CodeHash, RequestID string
	Expires                    int64
}
type request struct {
	Owner, Controller, Status string
	Expires                   int64
	Grant                     *Grant
}
type bucket struct {
	Count   int
	Expires int64
}
type snapshot struct {
	Version    int
	PrivateKey string
	LastTime   int64
	Devices    map[string]device
	Invites    map[string]invitation
	Requests   map[string]request
	Replay     map[string]int64
	Attempts   map[string]bucket
}
type Store struct {
	mu     sync.Mutex
	root   string
	clock  func() time.Time
	state  snapshot
	lease  *os.File
	broken bool
}

func randomHex(bytes int) (string, error) {
	b := make([]byte, bytes)
	_, err := rand.Read(b)
	return hex.EncodeToString(b), err
}
func deviceID(pub []byte) string { h := sha256.Sum256(pub); return hex.EncodeToString(h[:]) }
func requestBytes(body Body) []byte {
	b, _ := json.Marshal(body)
	return append([]byte("remoteapp-invite-request-v1\n"), b...)
}
func grantBytes(c Claims) []byte {
	b, _ := json.Marshal(c)
	return append([]byte("remoteapp-invite-grant-v1\n"), b...)
}

func Sign(key ed25519.PrivateKey, body Body) Envelope {
	return Envelope{hex.EncodeToString(key.Public().(ed25519.PublicKey)), body, hex.EncodeToString(ed25519.Sign(key, requestBytes(body)))}
}

// Caller must supply a dedicated absolute private directory. There is one writer.
// A crashed writer leaves a lease: recovery needs an explicit operator check;
// never silently delete a lock, reset identities, or open a second writer.
func Open(root string, clock func() time.Time) (*Store, error) {
	if !filepath.IsAbs(root) || clock == nil {
		return nil, ErrStorage
	}
	_, priorErr := os.Lstat(root)
	newRoot := os.IsNotExist(priorErr)
	if priorErr != nil && !newRoot {
		return nil, ErrStorage
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, ErrStorage
	}
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0077 != 0 {
		return nil, ErrStorage
	}
	lease, err := os.OpenFile(filepath.Join(root, "writer.lock"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return nil, ErrStorage
	}
	s := &Store{root: root, clock: clock, lease: lease}
	fail := func() (*Store, error) { s.Close(); return nil, ErrStorage }
	path := filepath.Join(root, "state.json")
	info, err = os.Lstat(path)
	if os.IsNotExist(err) {
		// Existing directories without a snapshot may have lost their issuer
		// identity. Never silently turn that into a fresh authorization service.
		if !newRoot {
			return fail()
		}
		_, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return fail()
		}
		s.state = snapshot{Version: 1, PrivateKey: hex.EncodeToString(priv), Devices: map[string]device{}, Invites: map[string]invitation{}, Requests: map[string]request{}, Replay: map[string]int64{}, Attempts: map[string]bucket{}}
		if s.save(s.state) != nil {
			return fail()
		}
		return s, nil
	}
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Size() > maxState {
		return fail()
	}
	f, err := os.Open(path)
	if err != nil {
		return fail()
	}
	defer f.Close()
	dec := json.NewDecoder(io.LimitReader(f, maxState+1))
	dec.DisallowUnknownFields()
	if dec.Decode(&s.state) != nil {
		return fail()
	}
	var extra any
	if dec.Decode(&extra) != io.EOF {
		return fail()
	}
	key, err := hex.DecodeString(s.state.PrivateKey)
	if err != nil || len(key) != ed25519.PrivateKeySize || s.state.Version != 1 || s.state.Devices == nil || s.state.Invites == nil || s.state.Requests == nil || s.state.Replay == nil || s.state.Attempts == nil || len(s.state.Devices) > 128 || len(s.state.Invites) > 512 || len(s.state.Requests) > 512 || len(s.state.Replay) > 4096 || len(s.state.Attempts) > 1024 {
		return fail()
	}
	if subtle.ConstantTimeCompare(key, ed25519.NewKeyFromSeed(key[:32])) != 1 {
		return fail()
	}
	return s, nil
}
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.broken = true
	if s.lease == nil {
		return nil
	}
	err := s.lease.Close()
	s.lease = nil
	if removeErr := os.Remove(filepath.Join(s.root, "writer.lock")); err == nil {
		err = removeErr
	}
	return err
}
func (s *Store) PublicKey() ed25519.PublicKey {
	s.mu.Lock()
	defer s.mu.Unlock()
	key, _ := hex.DecodeString(s.state.PrivateKey)
	return append(ed25519.PublicKey(nil), ed25519.PrivateKey(key).Public().(ed25519.PublicKey)...)
}
func (s *Store) save(next snapshot) error {
	raw, err := json.Marshal(next)
	if err != nil || len(raw) > maxState {
		return ErrStorage
	}
	f, err := os.CreateTemp(s.root, "state-*.tmp")
	if err != nil {
		return ErrStorage
	}
	path := f.Name()
	defer os.Remove(path)
	if _, err = f.Write(raw); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return ErrStorage
	}
	if os.Rename(path, filepath.Join(s.root, "state.json")) != nil {
		return ErrStorage
	}
	dir, err := os.Open(s.root)
	if err != nil {
		return ErrStorage
	}
	err = dir.Sync()
	dir.Close()
	if err != nil {
		return ErrStorage
	}
	return nil
}
func (s *Store) codeHash(id, code string) string {
	key, _ := hex.DecodeString(s.state.PrivateKey)
	derived := sha256.Sum256(append([]byte("remoteapp-invite-code-hmac-v1\n"), key...))
	mac := hmac.New(sha256.New, derived[:])
	mac.Write([]byte(id + ":" + code))
	return hex.EncodeToString(mac.Sum(nil))
}

// Execute verifies possession, timestamp, one-use nonce and permission under a
// mutex. Even denied authenticated requests persist nonce/rate-limit changes.
func (s *Store) Execute(e Envelope) (Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.broken {
		return Result{}, ErrStorage
	}
	// Bound all caller-controlled strings before serialization or hex decoding.
	if len(e.PublicKey) != 64 || len(e.Signature) != 128 || len(e.Body.Nonce) != 32 || len(e.Body.Action) > 32 || len(e.Body.TargetCode) > 12 || len(e.Body.InviteID) > 32 || len(e.Body.RequestID) > 32 || len(e.Body.GrantID) > 32 || len(e.Body.Code) > 8 {
		return Result{}, ErrDenied
	}
	raw, _ := json.Marshal(e)
	pub, err := hex.DecodeString(e.PublicKey)
	sig, sigErr := hex.DecodeString(e.Signature)
	nonce, nErr := hex.DecodeString(e.Body.Nonce)
	now := s.clock().Unix()
	if len(raw) > 4096 || err != nil || sigErr != nil || nErr != nil || len(pub) != ed25519.PublicKeySize || len(sig) != ed25519.SignatureSize || len(nonce) != 16 || e.PublicKey != hex.EncodeToString(pub) || e.Body.Nonce != hex.EncodeToString(nonce) || e.Body.IssuedAt < now-30 || e.Body.IssuedAt > now+30 || !ed25519.Verify(pub, requestBytes(e.Body), sig) {
		return Result{}, ErrDenied
	}
	if now < s.state.LastTime {
		return Result{}, ErrDenied
	}
	id := deviceID(pub)
	replayID := id + ":" + e.Body.Nonce
	if _, seen := s.state.Replay[replayID]; seen {
		return Result{}, ErrDenied
	}
	// Clone before mutation: a failed atomic commit cannot serve a live grant.
	old, _ := json.Marshal(s.state)
	var next snapshot
	json.Unmarshal(old, &next)
	for k, expires := range next.Replay {
		if expires < now {
			delete(next.Replay, k)
		}
	}
	for k, b := range next.Attempts {
		if b.Expires <= now {
			delete(next.Attempts, k)
		}
	}
	if len(next.Replay) >= 4096 {
		return Result{}, ErrDenied
	}
	next.Replay[replayID] = now + 61
	next.LastTime = now
	result, opErr := s.apply(&next, id, e.Body, now)
	if s.save(next) != nil {
		s.broken = true
		return Result{}, ErrStorage
	}
	s.state = next
	if result.Grant != nil {
		copy := *result.Grant
		result.Grant = &copy
	}
	return result, opErr
}
func (s *Store) apply(st *snapshot, id string, b Body, now int64) (Result, error) {
	denied := func() (Result, error) { return Result{}, ErrDenied }
	if b.Action == "register" {
		d, exists := st.Devices[id]
		if !exists {
			if len(st.Devices) >= 128 {
				return denied()
			}
			for {
				code, err := randomHex(6)
				if err != nil {
					return Result{}, ErrStorage
				}
				collision := false
				for _, other := range st.Devices {
					if other.Code == code {
						collision = true
					}
				}
				if !collision {
					d = device{Code: code}
					break
				}
			}
			st.Devices[id] = d
		}
		return Result{DeviceID: id, DeviceCode: d.Code, Status: "registered"}, nil
	}
	if _, exists := st.Devices[id]; !exists {
		return denied()
	}
	switch b.Action {
	case "invite":
		if len(st.Invites) >= 512 {
			return denied()
		}
		inviteID, err := randomHex(16)
		if err != nil {
			return Result{}, ErrStorage
		}
		number, err := rand.Int(rand.Reader, big.NewInt(100000000))
		if err != nil {
			return Result{}, ErrStorage
		}
		code := fmt.Sprintf("%08d", number.Int64())
		d := st.Devices[id]
		if old, exists := st.Invites[d.ActiveInvite]; exists {
			old.Expires = now
			st.Invites[d.ActiveInvite] = old
			if req, exists := st.Requests[old.RequestID]; exists && req.Status == "pending" {
				req.Status = "denied"
				st.Requests[old.RequestID] = req
			}
		}
		d.ActiveInvite = inviteID
		st.Devices[id] = d
		st.Invites[inviteID] = invitation{Owner: id, CodeHash: s.codeHash(inviteID, code), Expires: now + 120}
		return Result{InviteID: inviteID, Code: code, Status: "inviting"}, nil
	case "claim":
		// Users supply just device code + verification code. Invite IDs remain
		// internal; an optional ID must match the owner's current invitation.
		var target device
		for _, d := range st.Devices {
			if d.Code == b.TargetCode {
				target = d
				break
			}
		}
		inviteID := target.ActiveInvite
		// Both per-device and per-invitation budgets; unknown IDs do not enumerate.
		keys := []string{"device:" + id}
		if _, exists := st.Invites[inviteID]; exists {
			keys = append(keys, "invite:"+inviteID)
		}
		for _, k := range keys {
			bucket := st.Attempts[k]
			if bucket.Expires > now && bucket.Count >= 5 {
				return denied()
			}
		}
		for _, k := range keys {
			v := st.Attempts[k]
			if v.Expires <= now {
				v = bucket{Expires: now + 60}
			}
			v.Count++
			st.Attempts[k] = v
		}
		inv, exists := st.Invites[inviteID]
		expected := s.codeHash(inviteID, b.Code)
		if !exists || (b.InviteID != "" && b.InviteID != inviteID) || inv.Owner == id || inv.Expires <= now || inv.RequestID != "" || target.Code != b.TargetCode || len(b.Code) != 8 || strings.Trim(b.Code, "0123456789") != "" || subtle.ConstantTimeCompare([]byte(expected), []byte(inv.CodeHash)) != 1 || len(st.Requests) >= 512 {
			return denied()
		}
		requestID, err := randomHex(16)
		if err != nil {
			return Result{}, ErrStorage
		}
		inv.RequestID = requestID
		st.Invites[inviteID] = inv
		st.Requests[requestID] = request{Owner: inv.Owner, Controller: id, Status: "pending", Expires: inv.Expires}
		return Result{RequestID: requestID, Status: "pending"}, nil
	case "approve":
		req, exists := st.Requests[b.RequestID]
		if !exists || req.Owner != id || req.Status != "pending" || req.Expires <= now {
			return denied()
		}
		if !b.Allow {
			req.Status = "denied"
		} else {
			for _, active := range st.Requests {
				if active.Owner == id && active.Status == "approved" && active.Grant != nil && active.Grant.Claims.ExpiresAt > now {
					return denied()
				}
			}
			grantID, err := randomHex(16)
			if err != nil {
				return Result{}, ErrStorage
			}
			claims := Claims{1, grantID, req.Owner, req.Controller, permission, now, now + 300}
			key, _ := hex.DecodeString(st.PrivateKey)
			grant := Grant{claims, hex.EncodeToString(ed25519.Sign(key, grantBytes(claims)))}
			req.Grant = &grant
			req.Status = "approved"
		}
		st.Requests[b.RequestID] = req
		return Result{RequestID: b.RequestID, Status: req.Status, Grant: req.Grant}, nil
	case "status":
		req, exists := st.Requests[b.RequestID]
		if !exists || (req.Owner != id && req.Controller != id) {
			return denied()
		}
		status := req.Status
		grant := req.Grant
		if req.Status == "pending" && req.Expires <= now || req.Status == "approved" && grant != nil && grant.Claims.ExpiresAt <= now {
			status = "expired"
		}
		if status != "approved" {
			grant = nil
		}
		return Result{RequestID: b.RequestID, Status: status, Grant: grant}, nil
	case "revoke":
		for requestID, req := range st.Requests {
			if req.Grant != nil && req.Grant.Claims.ID == b.GrantID {
				if req.Owner != id && req.Controller != id {
					return denied()
				}
				req.Status = "revoked"
				st.Requests[requestID] = req
				return Result{Status: "revoked"}, nil
			}
		}
	}
	return denied()
}

// VerifyGrant is offline cryptographic verification, not a revocation check.
// The issuer key is supplied from trusted provisioning, never from the grant.
func VerifyGrant(pub ed25519.PublicKey, g Grant, target, controller string, now time.Time) bool {
	sig, err := hex.DecodeString(g.Signature)
	c := g.Claims
	return err == nil && len(pub) == ed25519.PublicKeySize && c.Version == 1 && len(c.ID) == 32 && c.Target == target && c.Controller == controller && c.Permission == permission && c.IssuedAt <= now.Unix() && c.ExpiresAt > now.Unix() && c.ExpiresAt-c.IssuedAt == 300 && ed25519.Verify(pub, grantBytes(c), sig)
}
func (s *Store) Authorized(g Grant, target, controller string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.broken || s.clock().Unix() < s.state.LastTime {
		return false
	}
	key, _ := hex.DecodeString(s.state.PrivateKey)
	if !VerifyGrant(ed25519.PrivateKey(key).Public().(ed25519.PublicKey), g, target, controller, s.clock()) {
		return false
	}
	for _, req := range s.state.Requests {
		if req.Status == "approved" && req.Grant != nil && *req.Grant == g {
			return true
		}
	}
	return false
}
