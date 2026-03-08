package ai

import "time"

// Severity indicates the severity level of an RCA finding.
type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityHigh     Severity = "high"
	SeverityMedium   Severity = "medium"
	SeverityLow      Severity = "low"
	SeverityInfo     Severity = "info"
)

// RCAStatus tracks the lifecycle of an RCA report.
type RCAStatus string

const (
	RCAStatusPending    RCAStatus = "pending"
	RCAStatusAnalyzing  RCAStatus = "analyzing"
	RCAStatusComplete   RCAStatus = "complete"
	RCAStatusActionTaken RCAStatus = "action_taken"
	RCAStatusFailed     RCAStatus = "failed"
)

// ResourceRef identifies a Kubernetes resource in a cluster.
type ResourceRef struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

// RCAReport is the structured output of the Root Cause Analysis engine.
// It contains the diagnosis, evidence chain, and remediation steps.
type RCAReport struct {
	ID             string            `json:"id"`
	Timestamp      time.Time         `json:"timestamp"`
	TargetResource ResourceRef       `json:"target_resource"`
	Severity       Severity          `json:"severity"`
	RootCause      RootCause         `json:"root_cause"`
	EvidenceChain  []Evidence        `json:"evidence_chain"`
	Remediation    []RemediationStep `json:"remediation"`
	RelatedIssues  []RelatedIssue    `json:"related_issues,omitempty"`
	Confidence     float64           `json:"confidence"` // 0.0-1.0
	Status         RCAStatus         `json:"status"`
}

// RootCause captures the identified cause of an incident.
type RootCause struct {
	Category           string   `json:"category"` // OOM, CrashLoop, ImagePull, Scheduling, Network, Config, Resource
	Summary            string   `json:"summary"`
	Detail             string   `json:"detail"`
	AffectedComponents []string `json:"affected_components,omitempty"`
}

// Evidence is a single piece of diagnostic data supporting the root cause analysis.
type Evidence struct {
	Source    string    `json:"source"` // "pod-logs", "events", "metrics", "config", "status"
	Data      string   `json:"data"`
	Relevance string   `json:"relevance"`
	Timestamp time.Time `json:"timestamp"`
}

// RemediationStep is an actionable fix suggested by the RCA engine.
type RemediationStep struct {
	Order       int    `json:"order"`
	Action      string `json:"action"` // "scale", "restart", "patch", "rollback", "manual", "delete_pod"
	Description string `json:"description"`
	Command     string `json:"command,omitempty"` // K8s operation to execute
	Risk        string `json:"risk"`              // "safe", "moderate", "high"
	AutoApply   bool   `json:"auto_apply"`
	RequiresCR  bool   `json:"requires_cr"`
}

// RelatedIssue links to another resource that may be impacted.
type RelatedIssue struct {
	Resource    ResourceRef `json:"resource"`
	Relationship string    `json:"relationship"` // "depends_on", "same_node", "same_namespace", "owner"
	Description  string    `json:"description"`
}

// LLMRCAResponse is the JSON schema the LLM is asked to produce.
// It is parsed into an RCAReport by the RCA engine.
type LLMRCAResponse struct {
	Severity   string `json:"severity"`
	RootCause  struct {
		Category           string   `json:"category"`
		Summary            string   `json:"summary"`
		Detail             string   `json:"detail"`
		AffectedComponents []string `json:"affected_components"`
	} `json:"root_cause"`
	Evidence []struct {
		Source    string `json:"source"`
		Data     string `json:"data"`
		Relevance string `json:"relevance"`
	} `json:"evidence"`
	Remediation []struct {
		Order       int    `json:"order"`
		Action      string `json:"action"`
		Description string `json:"description"`
		Command     string `json:"command"`
		Risk        string `json:"risk"`
		AutoApply   bool   `json:"auto_apply"`
		RequiresCR  bool   `json:"requires_cr"`
	} `json:"remediation"`
	Confidence float64 `json:"confidence"`
}
