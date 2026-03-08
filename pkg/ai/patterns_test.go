package ai

import (
	"testing"
)

func TestCommonPatternsCompile(t *testing.T) {
	patterns := CommonPatterns()
	if len(patterns) == 0 {
		t.Fatal("CommonPatterns returned 0 patterns")
	}

	for _, p := range patterns {
		if p.Name == "" {
			t.Error("pattern has empty name")
		}
		if p.Regex == nil {
			t.Errorf("pattern %q has nil regex", p.Name)
		}
		if p.Severity == "" {
			t.Errorf("pattern %q has empty severity", p.Name)
		}
	}
}

func TestMatchPatterns_GoPanic(t *testing.T) {
	lines := []string{
		"goroutine 1 [running]:",
		"main.main()",
		"  /app/main.go:42 +0x1a8",
	}
	matches := MatchPatterns(lines)
	if _, ok := matches["go-panic"]; !ok {
		t.Error("expected go-panic pattern to match goroutine stack trace")
	}
}

func TestMatchPatterns_PythonTraceback(t *testing.T) {
	lines := []string{
		"Traceback (most recent call last):",
		"  File \"/app/main.py\", line 10, in <module>",
		"ValueError: invalid literal",
	}
	matches := MatchPatterns(lines)
	if _, ok := matches["python-traceback"]; !ok {
		t.Error("expected python-traceback pattern to match")
	}
}

func TestMatchPatterns_OOMKilled(t *testing.T) {
	lines := []string{
		"fatal: out of memory allocating 1048576 bytes",
		"normal log line",
		"container killed signal 9",
	}
	matches := MatchPatterns(lines)
	if _, ok := matches["oom-killed"]; !ok {
		t.Error("expected oom-killed pattern to match")
	}
	if len(matches["oom-killed"]) != 2 {
		t.Errorf("expected 2 oom matches, got %d", len(matches["oom-killed"]))
	}
}

func TestMatchPatterns_ConnectionRefused(t *testing.T) {
	lines := []string{
		"dial tcp 10.0.0.1:5432: connect: connection refused",
	}
	matches := MatchPatterns(lines)
	if _, ok := matches["connection-refused"]; !ok {
		t.Error("expected connection-refused pattern to match")
	}
}

func TestMatchPatterns_DNSFailure(t *testing.T) {
	lines := []string{
		"lookup db-service.default.svc.cluster.local: no such host",
	}
	matches := MatchPatterns(lines)
	if _, ok := matches["dns-failure"]; !ok {
		t.Error("expected dns-failure pattern to match")
	}
}

func TestMatchPatterns_Timeout(t *testing.T) {
	lines := []string{
		"context deadline exceeded",
		"i/o timeout after 30s",
	}
	matches := MatchPatterns(lines)
	if _, ok := matches["timeout"]; !ok {
		t.Error("expected timeout pattern to match")
	}
	if len(matches["timeout"]) != 2 {
		t.Errorf("expected 2 timeout matches, got %d", len(matches["timeout"]))
	}
}

func TestMatchPatterns_AuthFailure(t *testing.T) {
	lines := []string{
		"HTTP 403 forbidden: user lacks permission",
		"authentication failed: invalid token",
	}
	matches := MatchPatterns(lines)
	if _, ok := matches["auth-failure"]; !ok {
		t.Error("expected auth-failure pattern to match")
	}
}

func TestMatchPatterns_TLSError(t *testing.T) {
	lines := []string{
		"x509: certificate has expired or is not yet valid",
	}
	matches := MatchPatterns(lines)
	if _, ok := matches["tls-error"]; !ok {
		t.Error("expected tls-error pattern to match")
	}
}

func TestMatchPatterns_DiskFull(t *testing.T) {
	lines := []string{
		"write /var/log/app.log: no space left on device",
	}
	matches := MatchPatterns(lines)
	if _, ok := matches["disk-full"]; !ok {
		t.Error("expected disk-full pattern to match")
	}
}

func TestMatchPatterns_RateLimit(t *testing.T) {
	lines := []string{
		"HTTP 429 too many requests",
		"rate limit exceeded, retrying in 30s",
	}
	matches := MatchPatterns(lines)
	if _, ok := matches["rate-limit"]; !ok {
		t.Error("expected rate-limit pattern to match")
	}
}

func TestMatchPatterns_DBConnection(t *testing.T) {
	lines := []string{
		"FATAL: database connection pool exhausted",
	}
	matches := MatchPatterns(lines)
	if _, ok := matches["db-connection"]; !ok {
		t.Error("expected db-connection pattern to match")
	}
}

func TestMatchPatterns_NoMatch(t *testing.T) {
	lines := []string{
		"INFO: server started on port 8080",
		"DEBUG: processing request",
		"request completed in 45ms",
	}
	matches := MatchPatterns(lines)
	if len(matches) != 0 {
		t.Errorf("expected 0 matches for normal logs, got %d pattern groups", len(matches))
	}
}
