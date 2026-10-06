package invite

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"net/netip"
	"strings"
	"time"
)

// NetworkBinding is supplied only by trusted provisioning after querying the
// isolated Headscale administration API. It is never accepted from /v1/request.
type NetworkBinding struct {
	TargetNode     string `json:"target_node"`
	ControllerNode string `json:"controller_node"`
	TargetIP       string `json:"target_ip"`
	ControllerIP   string `json:"controller_ip"`
}
type NetworkClaims struct {
	Version   int            `json:"version"`
	Grant     Grant          `json:"grant"`
	Binding   NetworkBinding `json:"binding"`
	Active    bool           `json:"active"`
	IssuedAt  int64          `json:"issued_at"`
	ExpiresAt int64          `json:"expires_at"`
}
type NetworkState struct {
	Claims    NetworkClaims `json:"claims"`
	Signature string        `json:"signature"`
}

func networkBytes(c NetworkClaims) []byte {
	b, _ := json.Marshal(c)
	return append([]byte("remoteapp-network-state-v1\n"), b...)
}
func validBinding(b NetworkBinding) bool {
	for _, n := range []string{b.TargetNode, b.ControllerNode} {
		if !strings.HasPrefix(n, "nodekey:") {
			return false
		}
		raw, err := hex.DecodeString(strings.TrimPrefix(n, "nodekey:"))
		if err != nil || len(raw) != 32 || n != "nodekey:"+hex.EncodeToString(raw) {
			return false
		}
	}
	return b.TargetNode != b.ControllerNode && b.TargetIP == "100.120.0.1" && b.ControllerIP == "100.120.0.2"
}

// NetworkState signs a three-second snapshot, not an unlimited permission.
// Only the private in-process fixture calls this privileged method in this PoC.
func (s *Store) NetworkState(g Grant, b NetworkBinding) (NetworkState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.broken || s.clock().Unix() < s.state.LastTime || !validBinding(b) {
		return NetworkState{}, ErrDenied
	}
	key, _ := hex.DecodeString(s.state.PrivateKey)
	// An expired but genuine grant can still receive a signed inactive state;
	// otherwise its normal expiry would look like a broken authority process.
	if !VerifyGrant(ed25519.PrivateKey(key).Public().(ed25519.PublicKey), g, g.Claims.Target, g.Claims.Controller, time.Unix(g.Claims.IssuedAt, 0)) {
		return NetworkState{}, ErrDenied
	}
	active := s.authorizedLocked(g, g.Claims.Target, g.Claims.Controller)
	now := s.clock().UnixNano()
	c := NetworkClaims{1, g, b, active, now, now + int64(3*time.Second)}
	return NetworkState{c, hex.EncodeToString(ed25519.Sign(ed25519.PrivateKey(key), networkBytes(c)))}, nil
}
func VerifyNetworkState(pub ed25519.PublicKey, n NetworkState, now time.Time) bool {
	c := n.Claims
	sig, err := hex.DecodeString(n.Signature)
	_, e1 := netip.ParseAddr(c.Binding.TargetIP)
	_, e2 := netip.ParseAddr(c.Binding.ControllerIP)
	return len(pub) == 32 && err == nil && len(sig) == 64 && c.Version == 1 && validBinding(c.Binding) && e1 == nil && e2 == nil && c.IssuedAt <= now.UnixNano() && c.ExpiresAt > now.UnixNano() && c.ExpiresAt-c.IssuedAt == int64(3*time.Second) && ed25519.Verify(pub, networkBytes(c), sig)
}
