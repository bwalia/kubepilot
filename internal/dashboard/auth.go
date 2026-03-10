package dashboard

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// AuthConfig controls optional HTTP auth for dashboard and API routes.
//
// When Enabled is true, requests must include one of:
//  1. Authorization: Bearer <Token> (if Token configured)
//  2. Basic auth username/password (if Username and Password configured)
//
// This middleware intentionally allows /healthz without auth for liveness checks.
type AuthConfig struct {
	Enabled  bool
	Token    string
	Username string
	Password string
}

func withAuth(next http.Handler, cfg AuthConfig) http.Handler {
	token := strings.TrimSpace(cfg.Token)
	username := strings.TrimSpace(cfg.Username)
	password := strings.TrimSpace(cfg.Password)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			next.ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		authz := strings.TrimSpace(r.Header.Get("Authorization"))
		if authz != "" {
			if token != "" && hasAuthPrefix(authz, "Bearer ") {
				provided := strings.TrimSpace(authz[len("Bearer "):])
				if secureEqual(provided, token) {
					next.ServeHTTP(w, r)
					return
				}
			}
		}

		if username != "" && password != "" {
			if u, p, ok := r.BasicAuth(); ok && secureEqual(u, username) && secureEqual(p, password) {
				next.ServeHTTP(w, r)
				return
			}
		}

		w.Header().Set("WWW-Authenticate", `Basic realm="KubePilot"`)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	})
}

func hasAuthPrefix(value, prefix string) bool {
	if len(value) < len(prefix) {
		return false
	}
	return strings.EqualFold(value[:len(prefix)], prefix)
}

func secureEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
