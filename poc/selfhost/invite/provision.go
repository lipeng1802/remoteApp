package invite

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"net/netip"
	"sort"
	"strings"
)

// Private provisioning protocol. These actions are NOT added to public Handler.
type ProvisionBody struct {
	GrantID  string `json:"grant_id"`
	NodeKey  string `json:"node_key"`
	Nonce    string `json:"nonce"`
	IssuedAt int64  `json:"issued_at"`
}
type ProvisionEnvelope struct {
	PublicKey string        `json:"public_key"`
	Body      ProvisionBody `json:"body"`
	Signature string        `json:"signature"`
}
type ProvisionRecord struct {
	ID, Device, NodeKey, Status string
	Grant                       Grant
	Expires                     int64
	UserID, KeyID, NodeID, IP   string // provider handles, NEVER registration secrets
}
type ProvisionObservation struct{ UserID, NodeID, NodeKey, IP string }
type NetworkRule struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
}

func provisionBytes(b ProvisionBody) []byte {
	v, _ := json.Marshal(b)
	return append([]byte("remoteapp-provision-intent-v1\n"), v...)
}
func SignProvision(k ed25519.PrivateKey, b ProvisionBody) ProvisionEnvelope {
	return ProvisionEnvelope{hex.EncodeToString(k.Public().(ed25519.PublicKey)), b, hex.EncodeToString(ed25519.Sign(k, provisionBytes(b)))}
}
func validNodeKey(n string) bool {
	if len(n) != 72 || !strings.HasPrefix(n, "nodekey:") {
		return false
	}
	b, e := hex.DecodeString(n[8:])
	return e == nil && len(b) == 32 && n == "nodekey:"+hex.EncodeToString(b) && n != "nodekey:"+strings.Repeat("0", 64)
}
func safeHandle(s string) bool {
	if len(s) == 0 || len(s) > 64 {
		return false
	}
	for _, c := range s {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-') {
			return false
		}
	}
	return true
}
func validOverlay(s string) bool {
	ip, e := netip.ParseAddr(s)
	return e == nil && ip.String() == s && netip.MustParsePrefix("100.120.0.0/24").Contains(ip) && s != "100.120.0.0" && s != "100.120.0.255"
}
func validProvisions(m map[string]ProvisionRecord) bool {
	if len(m) > 512 {
		return false
	}
	for id, r := range m {
		if len(id) != 32 || r.ID != id || len(r.Device) != 64 || !validNodeKey(r.NodeKey) || r.Expires <= 0 || r.Expires > r.Grant.Claims.ExpiresAt || r.Device != r.Grant.Claims.Target && r.Device != r.Grant.Claims.Controller {
			return false
		}
		switch r.Status {
		case "planned", "cleanup_pending", "cleaned":
		case "issued":
			if !safeHandle(r.UserID) || !safeHandle(r.KeyID) {
				return false
			}
		case "bound":
			if !safeHandle(r.UserID) || !safeHandle(r.KeyID) || !safeHandle(r.NodeID) || !validOverlay(r.IP) {
				return false
			}
		default:
			return false
		}
	}
	return true
}
func (s *Store) provisionClone() snapshot {
	b, _ := json.Marshal(s.state)
	var n snapshot
	json.Unmarshal(b, &n)
	if n.Provisions == nil {
		n.Provisions = map[string]ProvisionRecord{}
	}
	return n
}
func (s *Store) provisionCommit(n snapshot) error {
	if s.save(n) != nil {
		s.broken = true
		return ErrStorage
	}
	s.state = n
	return nil
}

// PlanProvision commits intent BEFORE any provider side effect. The device must
// sign the expected node key and own one subject of a currently approved grant.
func (s *Store) PlanProvision(g Grant, e ProvisionEnvelope) (ProvisionRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.broken {
		return ProvisionRecord{}, ErrStorage
	}
	b := e.Body
	now := s.clock().Unix()
	if len(e.PublicKey) != 64 || len(e.Signature) != 128 || len(b.Nonce) != 32 || len(b.GrantID) != 32 || len(g.Signature) != 128 || len(g.Claims.Target) != 64 || len(g.Claims.Controller) != 64 || b.GrantID != g.Claims.ID || !validNodeKey(b.NodeKey) || b.IssuedAt < now-30 || b.IssuedAt > now+30 || now < s.state.LastTime {
		return ProvisionRecord{}, ErrDenied
	}
	pub, pe := hex.DecodeString(e.PublicKey)
	sig, se := hex.DecodeString(e.Signature)
	nonce, ne := hex.DecodeString(b.Nonce)
	if pe != nil || se != nil || ne != nil || len(nonce) != 16 || e.PublicKey != hex.EncodeToString(pub) || b.Nonce != hex.EncodeToString(nonce) || !ed25519.Verify(pub, provisionBytes(b), sig) {
		return ProvisionRecord{}, ErrDenied
	}
	device := deviceID(pub)
	if device != g.Claims.Target && device != g.Claims.Controller || !s.authorizedLocked(g, g.Claims.Target, g.Claims.Controller) {
		return ProvisionRecord{}, ErrDenied
	}
	replay := "provision:" + device + ":" + b.Nonce
	if _, ok := s.state.Replay[replay]; ok {
		return ProvisionRecord{}, ErrDenied
	}
	n := s.provisionClone()
	for k, v := range n.Replay {
		if v < now {
			delete(n.Replay, k)
		}
	}
	if len(n.Replay) >= 4096 || len(n.Provisions) >= 512 {
		return ProvisionRecord{}, ErrDenied
	}
	n.Replay[replay] = now + 61
	n.LastTime = now
	for _, r := range n.Provisions {
		if r.Status != "cleaned" && (r.Device == device || r.NodeKey == b.NodeKey) {
			if err := s.provisionCommit(n); err != nil {
				return ProvisionRecord{}, err
			}
			return ProvisionRecord{}, ErrDenied
		}
	}
	id, err := randomHex(16)
	if err != nil {
		return ProvisionRecord{}, ErrStorage
	}
	expires := now + 120
	if expires > g.Claims.ExpiresAt {
		expires = g.Claims.ExpiresAt
	}
	r := ProvisionRecord{ID: id, Device: device, NodeKey: b.NodeKey, Status: "planned", Grant: g, Expires: expires}
	n.Provisions[id] = r
	if err := s.provisionCommit(n); err != nil {
		return ProvisionRecord{}, err
	}
	return r, nil
}

// These completion calls are private trusted-adapter operations, not endpoints
// accepting user claims. Tokens never enter the durable ledger.
func (s *Store) ProvisionIssued(id, user, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.broken {
		return ErrStorage
	}
	r, ok := s.state.Provisions[id]
	if !ok || r.Status != "planned" || !safeHandle(user) || !safeHandle(key) {
		return ErrDenied
	}
	n := s.provisionClone()
	r.UserID = user
	r.KeyID = key
	r.Status = "issued"
	if s.clock().Unix() >= r.Expires || !s.authorizedLocked(r.Grant, r.Grant.Claims.Target, r.Grant.Claims.Controller) {
		r.Status = "cleanup_pending"
	}
	n.Provisions[id] = r
	if err := s.provisionCommit(n); err != nil {
		return err
	}
	if r.Status != "issued" {
		return ErrDenied
	}
	return nil
}
func (s *Store) ProvisionBound(id string, o ProvisionObservation, verified bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.broken {
		return ErrStorage
	}
	r, ok := s.state.Provisions[id]
	if !ok || r.Status != "issued" || !verified || o.NodeKey != r.NodeKey || o.UserID != r.UserID || !safeHandle(o.NodeID) || !validOverlay(o.IP) || s.clock().Unix() >= r.Expires || !s.authorizedLocked(r.Grant, r.Grant.Claims.Target, r.Grant.Claims.Controller) {
		return ErrDenied
	}
	for k, v := range s.state.Provisions {
		if k != id && v.Status != "cleaned" && (v.NodeID == o.NodeID || v.IP == o.IP) {
			return ErrDenied
		}
	}
	n := s.provisionClone()
	r.Status = "bound"
	r.NodeID = o.NodeID
	r.IP = o.IP
	n.Provisions[id] = r
	return s.provisionCommit(n)
}

// Cleanup is journaled first; only confirmed adapter cleanup can mark cleaned.
func (s *Store) ProvisionCleanup(id string, confirmed bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.broken {
		return ErrStorage
	}
	r, ok := s.state.Provisions[id]
	if !ok || confirmed && r.Status != "cleanup_pending" {
		return ErrDenied
	}
	if r.Status == "cleaned" {
		return ErrDenied
	}
	n := s.provisionClone()
	r.Status = "cleanup_pending"
	if confirmed {
		r.Status = "cleaned"
	}
	n.Provisions[id] = r
	return s.provisionCommit(n)
}
func (s *Store) ProvisionRecords() []ProvisionRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	v := make([]ProvisionRecord, 0, len(s.state.Provisions))
	for _, r := range s.state.Provisions {
		v = append(v, r)
	}
	return v
}

// Deny by default. A rule exists only for BOTH verified subjects of the exact
// current grant, with controller->target fixed-port direction and no wildcard.
func (s *Store) ProvisionRules() ([]NetworkRule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.broken {
		return nil, ErrStorage
	}
	if s.clock().Unix() < s.state.LastTime {
		return nil, ErrDenied
	}
	var rules []NetworkRule
	for _, target := range s.state.Provisions {
		if target.Status != "bound" || target.Device != target.Grant.Claims.Target || !s.authorizedLocked(target.Grant, target.Grant.Claims.Target, target.Grant.Claims.Controller) {
			continue
		}
		for _, client := range s.state.Provisions {
			if client.Status == "bound" && client.Grant == target.Grant && client.Device == target.Grant.Claims.Controller {
				rules = append(rules, NetworkRule{client.IP, target.IP + ":47476"})
			}
		}
	}
	sort.Slice(rules, func(i, j int) bool {
		if rules[i].Source == rules[j].Source {
			return rules[i].Destination < rules[j].Destination
		}
		return rules[i].Source < rules[j].Source
	})
	return rules, nil
}
