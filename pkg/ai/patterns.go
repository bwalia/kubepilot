package ai

import (
	"regexp"
)

// ErrorPattern defines a compiled regex pattern for identifying specific error types in logs.
type ErrorPattern struct {
	Name        string
	Description string
	Regex       *regexp.Regexp
	Severity    Severity
}

// CommonPatterns returns a set of compiled regex patterns for identifying
// common Kubernetes-related errors in pod logs.
func CommonPatterns() []ErrorPattern {
	return []ErrorPattern{
		{
			Name:        "go-panic",
			Description: "Go runtime panic with goroutine stack trace",
			Regex:       regexp.MustCompile(`(?m)^goroutine \d+ \[.*\]:`),
			Severity:    SeverityCritical,
		},
		{
			Name:        "python-traceback",
			Description: "Python exception traceback",
			Regex:       regexp.MustCompile(`(?m)^Traceback \(most recent call last\):`),
			Severity:    SeverityHigh,
		},
		{
			Name:        "java-exception",
			Description: "Java exception stack trace",
			Regex:       regexp.MustCompile(`(?m)^(Exception|Caused by|java\.\w+Exception)`),
			Severity:    SeverityHigh,
		},
		{
			Name:        "oom-killed",
			Description: "Out of memory termination signal",
			Regex:       regexp.MustCompile(`(?i)(oom|out.?of.?memory|cannot allocate memory|killed.*signal 9)`),
			Severity:    SeverityCritical,
		},
		{
			Name:        "connection-refused",
			Description: "TCP connection refused — target service may be down",
			Regex:       regexp.MustCompile(`(?i)(connection refused|ECONNREFUSED|connect: connection refused)`),
			Severity:    SeverityHigh,
		},
		{
			Name:        "dns-failure",
			Description: "DNS resolution failure — service discovery issue",
			Regex:       regexp.MustCompile(`(?i)(no such host|NXDOMAIN|name resolution failed|could not resolve)`),
			Severity:    SeverityHigh,
		},
		{
			Name:        "timeout",
			Description: "Operation timeout — possible network or performance issue",
			Regex:       regexp.MustCompile(`(?i)(context deadline exceeded|i/o timeout|read timeout|connect timeout)`),
			Severity:    SeverityMedium,
		},
		{
			Name:        "auth-failure",
			Description: "Authentication or authorization failure",
			Regex:       regexp.MustCompile(`(?i)(unauthorized|403 forbidden|authentication failed|invalid token|permission denied)`),
			Severity:    SeverityHigh,
		},
		{
			Name:        "tls-error",
			Description: "TLS/SSL certificate or handshake error",
			Regex:       regexp.MustCompile(`(?i)(certificate.*expired|x509|tls handshake|ssl.*error)`),
			Severity:    SeverityHigh,
		},
		{
			Name:        "disk-full",
			Description: "Disk space exhaustion",
			Regex:       regexp.MustCompile(`(?i)(no space left on device|disk full|ENOSPC)`),
			Severity:    SeverityCritical,
		},
		{
			Name:        "rate-limit",
			Description: "API rate limiting hit",
			Regex:       regexp.MustCompile(`(?i)(rate limit|429|too many requests|throttl)`),
			Severity:    SeverityMedium,
		},
		{
			Name:        "db-connection",
			Description: "Database connection failure",
			Regex:       regexp.MustCompile(`(?i)(database.*connection|connection.*pool|max.*connections|SQLSTATE)`),
			Severity:    SeverityHigh,
		},
	}
}

// MatchPatterns runs all common patterns against a set of log lines and returns matches.
func MatchPatterns(lines []string) map[string][]string {
	patterns := CommonPatterns()
	matches := make(map[string][]string)

	for _, line := range lines {
		for _, p := range patterns {
			if p.Regex.MatchString(line) {
				matches[p.Name] = append(matches[p.Name], line)
			}
		}
	}
	return matches
}
