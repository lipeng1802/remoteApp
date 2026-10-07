package invite

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"
)

// Signed protocol for unprivileged device clients. Deployment remains private
// until Internet admission/abuse controls and issuer distribution are complete.
type NetworkRequestBody struct {
	Action           string            `json:"action"`
	Nonce            string            `json:"nonce"`
	IssuedAt         int64             `json:"issued_at"`
	Grant            Grant             `json:"grant"`
	Ticket           string            `json:"ticket"`
	Challenge        string            `json:"challenge"`
	Intent           ProvisionEnvelope `json:"intent"`
	Certificate      string            `json:"certificate,omitempty"`
	CertificateProof string            `json:"certificate_proof,omitempty"`
}
type NetworkRequest struct {
	PublicKey string             `json:"public_key"`
	Body      NetworkRequestBody `json:"body"`
	Signature string             `json:"signature"`
}
type NetworkReply struct {
	Ticket     string                  `json:"ticket,omitempty"`
	Credential *RegistrationCredential `json:"credential,omitempty"`
	State      *NetworkState           `json:"network_state,omitempty"`
	Proof      *ClockProof             `json:"proof,omitempty"`
	TLS        *TLSIdentity            `json:"tls_identity,omitempty"`
}

func networkRequestBytes(b NetworkRequestBody) []byte {
	raw, _ := json.Marshal(b)
	return append([]byte("remoteapp-network-api-v1\nPOST /v1/network\n"), raw...)
}
func SignNetworkRequest(k ed25519.PrivateKey, b NetworkRequestBody) NetworkRequest {
	return NetworkRequest{hex.EncodeToString(k.Public().(ed25519.PublicKey)), b, hex.EncodeToString(ed25519.Sign(k, networkRequestBytes(b)))}
}

// Consume a valid device signature durably before privileged effects, including
// denied requests. Grant possession alone is not a caller credential.
func (s *Store) authorizeNetworkRequest(e NetworkRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.broken {
		return ErrStorage
	}
	raw, _ := json.Marshal(e)
	if len(raw) > 8192 || len(e.PublicKey) != 64 || len(e.Signature) != 128 || len(e.Body.Nonce) != 32 || len(e.Body.Action) > 32 || len(e.Body.Ticket) > 32 || len(e.Body.Challenge) > 64 {
		return ErrDenied
	}
	pub, pe := hex.DecodeString(e.PublicKey)
	sig, se := hex.DecodeString(e.Signature)
	nonce, ne := hex.DecodeString(e.Body.Nonce)
	now := s.clock().Unix()
	if pe != nil || se != nil || ne != nil || len(pub) != 32 || len(sig) != 64 || len(nonce) != 16 || e.PublicKey != hex.EncodeToString(pub) || e.Body.Nonce != hex.EncodeToString(nonce) || e.Signature != hex.EncodeToString(sig) || e.Body.IssuedAt < now-30 || e.Body.IssuedAt > now+30 || now < s.state.LastTime || !ed25519.Verify(pub, networkRequestBytes(e.Body), sig) {
		return ErrDenied
	}
	id := deviceID(pub)
	if _, registered := s.state.Devices[id]; !registered {
		return ErrDenied
	}
	replay := "network:" + id + ":" + e.Body.Nonce
	if _, ok := s.state.Replay[replay]; ok {
		return ErrDenied
	}
	next := s.provisionClone()
	for k, v := range next.Replay {
		if v < now {
			delete(next.Replay, k)
		}
	}
	for k, v := range next.Attempts {
		if v.Expires <= now {
			delete(next.Attempts, k)
		}
	}
	if len(next.Replay) >= 4096 {
		return ErrDenied
	}
	next.Replay[replay] = now + 61
	next.LastTime = now
	budget := "network:" + id
	limit := 240
	if e.Body.Action == "enroll" || e.Body.Action == "bind" {
		budget = "network-provision:" + id
		limit = 6
	}
	denied := false
	v := next.Attempts[budget]
	if v.Expires <= now {
		v = bucket{Expires: now + 60}
	}
	if v.Count >= limit || len(next.Attempts) >= 1024 {
		denied = true
	} else {
		v.Count++
		next.Attempts[budget] = v
	}
	if err := s.provisionCommit(next); err != nil {
		return err
	}
	g := e.Body.Grant
	if len(e.Body.Certificate) > 5500 || len(e.Body.CertificateProof) > 1024 || (e.Body.Action != "publish_tls" && (e.Body.Certificate != "" || e.Body.CertificateProof != "")) {
		return ErrDenied
	}
	if denied || id != g.Claims.Target && id != g.Claims.Controller || !VerifyGrant(s.publicKeyLocked(), g, g.Claims.Target, g.Claims.Controller, time.Unix(now, 0)) {
		return ErrDenied
	}
	known := false
	for _, r := range s.state.Requests {
		if r.Grant != nil && *r.Grant == g {
			known = true
		}
	}
	if !known {
		return ErrDenied
	}
	switch e.Body.Action {
	case "publish_tls":
		nonce, err := hex.DecodeString(e.Body.Challenge)
		if id != g.Claims.Target || err != nil || len(nonce) != 32 || hex.EncodeToString(nonce) != e.Body.Challenge || e.Body.Ticket != "" || e.Body.Intent != (ProvisionEnvelope{}) {
			return ErrDenied
		}
	case "tls_identity":
		nonce, err := hex.DecodeString(e.Body.Challenge)
		if err != nil || len(nonce) != 32 || hex.EncodeToString(nonce) != e.Body.Challenge || e.Body.Ticket != "" || e.Body.Intent != (ProvisionEnvelope{}) {
			return ErrDenied
		}
	case "enroll":
		if e.Body.Ticket != "" || e.Body.Challenge != "" || e.Body.Intent.PublicKey != e.PublicKey {
			return ErrDenied
		}
	case "bind":
		r, ok := s.state.Provisions[e.Body.Ticket]
		if !ok || r.Device != id || r.Grant != g || e.Body.Challenge != "" || e.Body.Intent != (ProvisionEnvelope{}) {
			return ErrDenied
		}
	case "state":
		if e.Body.Ticket != "" || e.Body.Challenge != "" || e.Body.Intent != (ProvisionEnvelope{}) {
			return ErrDenied
		}
	case "clock", "grant_clock":
		if e.Body.Ticket != "" || e.Body.Intent != (ProvisionEnvelope{}) {
			return ErrDenied
		}
	default:
		return ErrDenied
	}
	return nil
}
func (s *Store) publicKeyLocked() ed25519.PublicKey {
	k, _ := hex.DecodeString(s.state.PrivateKey)
	return ed25519.PrivateKey(k).Public().(ed25519.PublicKey)
}
func (b *ProvisionBackend) ExecuteNetworkRequest(ctx context.Context, e NetworkRequest) (NetworkReply, error) {
	if b.Store == nil || b.Driver == nil || ctx.Err() != nil {
		return NetworkReply{}, ErrDenied
	}
	if err := b.Store.authorizeNetworkRequest(e); err != nil {
		return NetworkReply{}, err
	}
	switch e.Body.Action {
	case "publish_tls":
		return NetworkReply{}, b.publishTLS(e)
	case "tls_identity":
		d, err := b.TLSIdentity(e.Body.Grant, e.Body.Challenge)
		if err != nil {
			return NetworkReply{}, err
		}
		return NetworkReply{TLS: &d}, nil
	case "enroll":
		id, c, err := b.Enroll(ctx, e.Body.Grant, e.Body.Intent)
		if err != nil {
			return NetworkReply{}, err
		}
		return NetworkReply{Ticket: id, Credential: &c}, nil
	case "bind":
		return NetworkReply{}, b.Bind(ctx, e.Body.Ticket)
	case "state":
		n, err := b.NetworkState(e.Body.Grant)
		if err != nil {
			return NetworkReply{}, err
		}
		return NetworkReply{State: &n}, nil
	case "clock":
		p, err := b.ClockProof(e.Body.Grant, e.Body.Challenge)
		if err != nil {
			return NetworkReply{}, err
		}
		return NetworkReply{Proof: &p}, nil
	case "grant_clock":
		p, err := b.GrantClockProof(e.Body.Grant, e.Body.Challenge)
		if err != nil {
			return NetworkReply{}, err
		}
		return NetworkReply{Proof: &p}, nil
	}
	return NetworkReply{}, ErrDenied
}

// Bounded private HTTPS protocol handler, not an Internet deployment decision.
// Global admission bounds privileged work; invalid signatures never call Driver.
func NetworkHandler(b *ProvisionBackend) http.Handler {
	slots := make(chan struct{}, 4)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		fail := func(status int, code string) {
			w.WriteHeader(status)
			json.NewEncoder(w).Encode(map[string]string{"error": code})
		}
		if r.URL.RawQuery != "" || r.URL.ForceQuery || r.Header.Get("Origin") != "" {
			fail(400, "invalid_request")
			return
		}
		if r.URL.Path != "/v1/network" && r.URL.Path != "/v1/request" {
			fail(404, "not_found")
			return
		}
		if r.Method != "POST" {
			fail(405, "method_not_allowed")
			return
		}
		if r.TLS == nil || r.Header.Get("Content-Type") != "application/json" {
			fail(400, "invalid_request")
			return
		}
		select {
		case slots <- struct{}{}:
			defer func() { <-slots }()
		default:
			fail(503, "unavailable")
			return
		}
		if r.URL.Path == "/v1/request" {
			Handler(b.Store).ServeHTTP(w, r)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 8192)
		defer r.Body.Close()
		var e NetworkRequest
		if decodeHTTP(r.Body, &e) != nil {
			fail(400, "invalid_request")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 13*time.Second)
		defer cancel()
		result, err := b.ExecuteNetworkRequest(ctx, e)
		if err == ErrStorage {
			fail(503, "unavailable")
			return
		}
		if err != nil {
			fail(403, "denied")
			return
		}
		json.NewEncoder(w).Encode(result)
	})
}
