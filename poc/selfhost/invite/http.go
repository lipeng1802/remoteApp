package invite

import (
	"encoding/json"
	"io"
	"net/http"
)

// Handler has no public enrollment protections and MUST stay on loopback.
// No request bodies, codes, signed grants or private keys are logged.
func Handler(s *Store) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		reply := func(status int, value any) { w.WriteHeader(status); json.NewEncoder(w).Encode(value) }
		fail := func(status int, code string) {
			reply(status, struct {
				Error string `json:"error"`
			}{code})
		}
		if r.URL.RawQuery != "" || r.URL.ForceQuery || r.Header.Get("Origin") != "" {
			fail(400, "invalid_request")
			return
		}
		if r.URL.Path == "/health" && r.Method == http.MethodGet {
			s.mu.Lock()
			available := !s.broken
			s.mu.Unlock()
			if !available {
				fail(503, "unavailable")
				return
			}
			reply(200, struct {
				Status string `json:"status"`
			}{"pass"})
			return
		}
		if r.URL.Path != "/v1/request" {
			fail(404, "not_found")
			return
		}
		if r.Method != http.MethodPost {
			fail(405, "method_not_allowed")
			return
		}
		if r.Header.Get("Content-Type") != "application/json" {
			fail(400, "invalid_request")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		defer r.Body.Close()
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		var env Envelope
		if dec.Decode(&env) != nil {
			fail(400, "invalid_request")
			return
		}
		var extra any
		if dec.Decode(&extra) != io.EOF {
			fail(400, "invalid_request")
			return
		}
		result, err := s.Execute(env)
		if err == ErrStorage {
			fail(503, "unavailable")
			return
		}
		if err != nil {
			fail(403, "denied")
			return
		}
		reply(200, result)
	})
}
