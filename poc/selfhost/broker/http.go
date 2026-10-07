package broker

import (
	"context"
	"encoding/json"
	"net/http"
	"remoteapp.local/selfhost-poc/invite"
	"time"
)

// DeviceHandler proxies only device requests; it has no Store/issuer private
// key or privileged driver. TLS/admission/server deadlines remain mandatory.
func DeviceHandler(c Client, issuer string) http.Handler {
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
		if r.URL.Path != "/v1/request" {
			fail(404, "not_found")
			return
		}
		if r.Method != http.MethodPost {
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
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		defer r.Body.Close()
		env, e := invite.DecodeDeviceRequest(r.Body)
		if e != nil {
			fail(400, "invalid_request")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
		defer cancel()
		reply, e := c.Call(ctx, Request{Operation: "device", Device: &env})
		if e != nil || reply.Status == 503 {
			fail(503, "unavailable")
			return
		}
		if reply.Status != 200 {
			fail(403, "denied")
			return
		}
		if reply.Issuer != issuer || reply.Device == nil {
			fail(503, "unavailable")
			return
		}
		json.NewEncoder(w).Encode(reply.Device)
	})
}
