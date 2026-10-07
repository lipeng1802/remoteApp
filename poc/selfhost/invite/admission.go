package invite

import (
	"encoding/json"
	"net"
	"net/http"
	"net/netip"
	"sync"
	"time"
)

// Admission is process-local and deliberately shared across Store/handler
// reopen. Socket peers only: Forwarded/X-Forwarded-For are NEVER trusted.
// Behind a proxy all clients share its quota until a separately authenticated
// proxy identity design exists. This is not distributed DDoS protection.
type Admission struct {
	mu     sync.Mutex
	peers  map[netip.Prefix]bucket
	global bucket
	last   int64
	now    func() time.Time
}

func NewAdmission() *Admission {
	return &Admission{peers: make(map[netip.Prefix]bucket), now: time.Now}
}

func (a *Admission) allow(remote string) bool {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		return false
	}
	ip, err := netip.ParseAddr(host)
	if err != nil || ip.Zone() != "" {
		return false
	}
	ip = ip.Unmap()
	bits := 64
	if ip.Is4() {
		bits = 32
	}
	prefix := netip.PrefixFrom(ip, bits).Masked()
	a.mu.Lock()
	defer a.mu.Unlock()
	now := a.now().Unix()
	if now < a.last {
		return false
	}
	a.last = now
	if a.global.Expires <= now {
		a.global = bucket{Expires: now + 60}
	}
	if a.global.Count >= 6000 {
		return false
	}
	// Even requests rejected by a peer quota consume the global budget.
	a.global.Count++
	for p, v := range a.peers {
		if v.Expires <= now {
			delete(a.peers, p)
		}
	}
	v, known := a.peers[prefix]
	if !known && len(a.peers) >= 256 {
		return false
	}
	if v.Expires <= now {
		v = bucket{Expires: now + 60}
	}
	if v.Count >= 600 {
		return false
	}
	v.Count++
	a.peers[prefix] = v
	return true
}

// Wrap before request parsing/signature verification. Bounded bodies, work
// slots and server read/header/write deadlines remain mandatory downstream.
func (a *Admission) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a == nil || !a.allow(r.RemoteAddr) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]string{"error": "denied"})
			return
		}
		next.ServeHTTP(w, r)
	})
}
