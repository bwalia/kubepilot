package ai

import (
	"testing"
	"time"
)

func TestSeverityConstants(t *testing.T) {
	tests := []struct {
		severity Severity
		expected string
	}{
		{SeverityCritical, "critical"},
		{SeverityHigh, "high"},
		{SeverityMedium, "medium"},
		{SeverityLow, "low"},
		{SeverityInfo, "info"},
	}

	for _, tc := range tests {
		if string(tc.severity) != tc.expected {
			t.Errorf("expected severity %q, got %q", tc.expected, tc.severity)
		}
	}
}

func TestRCAStatusConstants(t *testing.T) {
	tests := []struct {
		status   RCAStatus
		expected string
	}{
		{RCAStatusPending, "pending"},
		{RCAStatusAnalyzing, "analyzing"},
		{RCAStatusComplete, "complete"},
		{RCAStatusActionTaken, "action_taken"},
		{RCAStatusFailed, "failed"},
	}

	for _, tc := range tests {
		if string(tc.status) != tc.expected {
			t.Errorf("expected status %q, got %q", tc.expected, tc.status)
		}
	}
}

func TestRCAReportFields(t *testing.T) {
	report := RCAReport{
		ID:        "rca-test-001",
		Timestamp: time.Now(),
		TargetResource: ResourceRef{
			Kind:      "Pod",
			Name:      "nginx-abc123",
			Namespace: "default",
		},
		Severity: SeverityHigh,
		RootCause: RootCause{
			Category: "CrashLoop",
			Summary:  "Application crash due to misconfiguration",
			Detail:   "The pod is crashing because the config file is missing.",
		},
		EvidenceChain: []Evidence{
			{
				Source:    "pod-logs",
				Data:      "Error: config.yaml not found",
				Relevance: "Direct cause of crash",
				Timestamp: time.Now(),
			},
		},
		Remediation: []RemediationStep{
			{
				Order:       1,
				Action:      "manual",
				Description: "Create the missing config.yaml ConfigMap",
				Risk:        "safe",
				AutoApply:   false,
				RequiresCR:  false,
			},
		},
		Confidence: 0.85,
		Status:     RCAStatusComplete,
	}

	if report.ID != "rca-test-001" {
		t.Errorf("expected ID 'rca-test-001', got %q", report.ID)
	}
	if report.TargetResource.Kind != "Pod" {
		t.Errorf("expected kind 'Pod', got %q", report.TargetResource.Kind)
	}
	if report.Severity != SeverityHigh {
		t.Errorf("expected severity 'high', got %q", report.Severity)
	}
	if report.Confidence < 0 || report.Confidence > 1 {
		t.Errorf("confidence should be between 0 and 1, got %f", report.Confidence)
	}
	if len(report.EvidenceChain) != 1 {
		t.Errorf("expected 1 evidence item, got %d", len(report.EvidenceChain))
	}
	if len(report.Remediation) != 1 {
		t.Errorf("expected 1 remediation step, got %d", len(report.Remediation))
	}
}
