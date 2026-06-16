// Package middleware provides HTTP middleware for the ElaMachan backend.
package middleware

import (
	"net/http"
	"os"
	"strings"
)

// CORS wraps h with a CORS middleware driven by CORS_ALLOWED_ORIGINS (comma-separated).
// Default allow-list: https://ai-elamachan.vercel.app,http://localhost:3000.
// Preflight OPTIONS requests receive 204; disallowed origins get no CORS headers.
func CORS(h http.Handler) http.Handler {
	rawOrigins := os.Getenv("CORS_ALLOWED_ORIGINS")
	if rawOrigins == "" {
		rawOrigins = "https://ai-elamachan.vercel.app,http://localhost:3000"
	}

	allowed := make(map[string]struct{})
	for _, o := range strings.Split(rawOrigins, ",") {
		if trimmed := strings.TrimSpace(o); trimmed != "" {
			allowed[trimmed] = struct{}{}
		}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			if _, ok := allowed[origin]; ok {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Add("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
				w.Header().Set("Access-Control-Max-Age", "86400")
			}
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		h.ServeHTTP(w, r)
	})
}
