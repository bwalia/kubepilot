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
// This middleware intentionally allows /healthz without auth for liveness
// checks, and /metrics without auth for Prometheus scraping.
//
// Exempting /metrics is the standard convention — most scrapers are configured
// without credentials — and the endpoint is built to carry no secrets: its
// labels are route templates and verbs, never namespace, pod, or object names.
// Operators who still want it closed can set MetricsRequireAuth.
type AuthConfig struct {
	Enabled  bool
	Token    string
	Username string
	Password string
	// MetricsRequireAuth closes /metrics behind the same credentials as the
	// rest of the API. Scrapers then need matching credentials configured.
	MetricsRequireAuth bool
	// IngestToken is a Bearer token accepted only for POSTs to the OTLP
	// ingest routes. A collector then never holds dashboard credentials.
	IngestToken string
}

// isOTLPIngest matches the routes a collector pushes telemetry to.
func isOTLPIngest(r *http.Request) bool {
	if r.Method != http.MethodPost {
		return false
	}
	switch r.URL.Path {
	case "/api/v1/otel/ingest", "/api/v1/otel/v1/traces", "/api/v1/otel/v1/metrics", "/api/v1/otel/v1/logs":
		return true
	}
	return false
}

func withAuth(next http.Handler, cfg AuthConfig) http.Handler {
	token := strings.TrimSpace(cfg.Token)
	username := strings.TrimSpace(cfg.Username)
	password := strings.TrimSpace(cfg.Password)
	ingestToken := strings.TrimSpace(cfg.IngestToken)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			next.ServeHTTP(w, r)
			return
		}
		if r.URL.Path == "/metrics" && !cfg.MetricsRequireAuth {
			next.ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		authz := strings.TrimSpace(r.Header.Get("Authorization"))
		if authz != "" {
			if hasAuthPrefix(authz, "Bearer ") {
				provided := strings.TrimSpace(authz[len("Bearer "):])
				if token != "" && secureEqual(provided, token) {
					next.ServeHTTP(w, r)
					return
				}
				if ingestToken != "" && isOTLPIngest(r) && secureEqual(provided, ingestToken) {
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
