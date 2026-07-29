package api

import "net/http"

// withCORS allows the configured browser origin. Local-only v1 (spec §6) means
// exactly one origin — the Vite dev server — so this echoes a single configured
// value rather than reflecting whatever Origin arrives. An empty AllowedOrigin
// disables the headers entirely.
func withCORS(origin string, next http.Handler) http.Handler {
	if origin == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Origin") == origin {
			h := w.Header()
			h.Set("Access-Control-Allow-Origin", origin)
			h.Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			h.Set("Access-Control-Allow-Headers", "Content-Type")
			// Origin-dependent response, so caches must not serve one origin's
			// response to another.
			h.Add("Vary", "Origin")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
