package broker

import (
	"context"
	"encoding/json"
	"net/http"
	"remoteapp.local/selfhost-poc/invite"
	"time"
)

// PrivateNetworkHandler is a test/integration adapter, not enabled by the
// installed device-only frontend. Deploy only behind trusted TLS/admission.
// No Store or private issuer is available to this adapter.
func PrivateNetworkHandler(c Client, issuer string) http.Handler {
	device := DeviceHandler(c, issuer)
	slots := make(chan struct{}, 4)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/request" {
			device.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		fail := func(status int) {
			w.WriteHeader(status)
			json.NewEncoder(w).Encode(map[string]string{"error": "request_unavailable"})
		}
		if r.URL.Path != "/v1/network" {
			fail(404)
			return
		}
		if r.Method != "POST" {
			fail(405)
			return
		}
		if r.TLS == nil || r.Header.Get("Content-Type") != "application/json" || r.Header.Get("Origin") != "" || r.URL.RawQuery != "" || r.URL.ForceQuery {
			fail(400)
			return
		}
		select {
		case slots <- struct{}{}:
			defer func() { <-slots }()
		default:
			fail(503)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 8192)
		defer r.Body.Close()
		env, e := invite.DecodeNetworkRequest(r.Body)
		if e != nil {
			fail(400)
			return
		}
		req := Request{Operation: "network", Network: &env}
		// Full wrapper must fit the strict IPC frame, including its newline.
		raw, e := json.Marshal(req)
		if e != nil || len(raw) > 8191 {
			fail(400)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 14*time.Second)
		defer cancel()
		reply, e := c.Call(ctx, req)
		if e != nil || reply.Status == 503 {
			fail(503)
			return
		}
		if reply.Status != 200 {
			fail(403)
			return
		}
		if reply.Issuer != issuer || reply.Network == nil {
			fail(503)
			return
		}
		json.NewEncoder(w).Encode(reply.Network)
	})
}
