package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/kubepilot/kubepilot/pkg/ai"
)

// AnomalyNotifier sends alerts about high-severity findings to an external system.
// Implementations should be best-effort: failures must not block detection.
type AnomalyNotifier interface {
	// NotifyAnomaly is called when a new high-severity anomaly is detected,
	// before RCA has run. Implementations may choose to swallow lower-severity
	// findings; the watcher filters but doesn't enforce it.
	NotifyAnomaly(ctx context.Context, a *Anomaly)
	// NotifyReport is called after an RCA report completes (any severity).
	// Implementations typically filter to critical/high.
	NotifyReport(ctx context.Context, r *ai.RCAReport)
}

// SlackNotifier posts incoming-webhook messages to Slack for high-severity events.
//
// Minimum severity is configurable; "high" by default, so "medium"/"low"/"info"
// are ignored. Dashboard URL is optional — when set, each message includes a
// link back to the KubePilot UI for fast follow-up.
type SlackNotifier struct {
	webhookURL    string
	minSeverity   ai.Severity
	dashboardURL  string
	client        *http.Client
	log           *zap.Logger
}

// SlackConfig holds Slack notifier configuration.
type SlackConfig struct {
	WebhookURL    string
	MinSeverity   string // critical | high | medium | low | info; default "high"
	DashboardURL  string // optional, used to render dashboard links
	Timeout       time.Duration
}

// NewSlackNotifier returns a Slack notifier, or nil if no webhook URL is set
// (so callers can compose Notifier-or-nil checks cleanly).
func NewSlackNotifier(cfg SlackConfig, log *zap.Logger) *SlackNotifier {
	if strings.TrimSpace(cfg.WebhookURL) == "" {
		return nil
	}
	minSev := ai.Severity(strings.ToLower(strings.TrimSpace(cfg.MinSeverity)))
	if minSev == "" {
		minSev = ai.SeverityHigh
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &SlackNotifier{
		webhookURL:   cfg.WebhookURL,
		minSeverity:  minSev,
		dashboardURL: strings.TrimRight(cfg.DashboardURL, "/"),
		client:       &http.Client{Timeout: timeout},
		log:          log,
	}
}

// severityRank gives a numeric priority for filtering (critical=4 ... info=0).
func severityRank(s ai.Severity) int {
	switch s {
	case ai.SeverityCritical:
		return 4
	case ai.SeverityHigh:
		return 3
	case ai.SeverityMedium:
		return 2
	case ai.SeverityLow:
		return 1
	}
	return 0
}

func (n *SlackNotifier) passes(sev ai.Severity) bool {
	return severityRank(sev) >= severityRank(n.minSeverity)
}

// NotifyAnomaly posts a short Slack card when a new high-severity anomaly is detected.
func (n *SlackNotifier) NotifyAnomaly(ctx context.Context, a *Anomaly) {
	if n == nil || a == nil || !n.passes(a.Severity) {
		return
	}
	text := fmt.Sprintf("*Anomaly detected* — `%s` in `%s/%s`",
		a.Rule, a.Resource.Namespace, a.Resource.Name)
	detail := a.Description
	if detail == "" {
		detail = "(no description)"
	}
	n.post(ctx, slackPayload{
		Text: text,
		Blocks: []slackBlock{
			{Type: "section", Text: &slackText{Type: "mrkdwn", Text: text}},
			{Type: "section", Text: &slackText{Type: "mrkdwn", Text: fmt.Sprintf("*Severity:* %s\n*Detail:* %s", strings.ToUpper(string(a.Severity)), truncate(detail, 800))}},
		},
	})
}

// NotifyReport posts a richer card when an RCA report is finalised.
func (n *SlackNotifier) NotifyReport(ctx context.Context, r *ai.RCAReport) {
	if n == nil || r == nil || !n.passes(r.Severity) {
		return
	}
	header := fmt.Sprintf(":rotating_light: *RCA: %s* — `%s/%s` (%s)",
		r.RootCause.Category, r.TargetResource.Namespace, r.TargetResource.Name,
		strings.ToUpper(string(r.Severity)))
	body := fmt.Sprintf("*Summary:* %s\n*Confidence:* %.0f%%",
		truncate(r.RootCause.Summary, 800), r.Confidence*100)
	if n.dashboardURL != "" {
		body += fmt.Sprintf("\n*Dashboard:* %s/", n.dashboardURL)
	}
	n.post(ctx, slackPayload{
		Text: fmt.Sprintf("RCA %s/%s — %s", r.TargetResource.Namespace, r.TargetResource.Name, r.RootCause.Category),
		Blocks: []slackBlock{
			{Type: "section", Text: &slackText{Type: "mrkdwn", Text: header}},
			{Type: "section", Text: &slackText{Type: "mrkdwn", Text: body}},
		},
	})
}

func (n *SlackNotifier) post(ctx context.Context, payload slackPayload) {
	body, err := json.Marshal(payload)
	if err != nil {
		n.log.Warn("Slack marshal failed", zap.Error(err))
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.webhookURL, bytes.NewReader(body))
	if err != nil {
		n.log.Warn("Slack request build failed", zap.Error(err))
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := n.client.Do(req)
	if err != nil {
		n.log.Warn("Slack POST failed", zap.Error(err))
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		n.log.Warn("Slack POST returned error", zap.Int("status", resp.StatusCode))
	}
}

type slackPayload struct {
	Text   string       `json:"text"`
	Blocks []slackBlock `json:"blocks,omitempty"`
}

type slackBlock struct {
	Type string     `json:"type"`
	Text *slackText `json:"text,omitempty"`
}

type slackText struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}
