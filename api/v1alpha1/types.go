// Package v1alpha1 defines the KubePilot CRD API types.
// CRDs: KubePilotCluster, KubePilotJob, KubePilotCRCode.
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ─────────────────────────────────────────
// KubePilotCluster CRD
// ─────────────────────────────────────────

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=kpc

// KubePilotCluster registers a remote Kubernetes cluster with KubePilot.
// The operator installs a MCP agent in the registered cluster and tracks its health.
type KubePilotCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KubePilotClusterSpec   `json:"spec"`
	Status KubePilotClusterStatus `json:"status,omitempty"`
}

// KubePilotClusterSpec describes the desired state of a registered cluster.
type KubePilotClusterSpec struct {
	// MCPServerAddress is the address of the KubePilot MCP server this cluster should connect to.
	MCPServerAddress string `json:"mcpServerAddress"`
	// KubeconfigSecretRef references a Kubernetes secret containing the cluster's kubeconfig.
	// +optional
	KubeconfigSecretRef *SecretRef `json:"kubeconfigSecretRef,omitempty"`
	// Environment labels the cluster: production | staging | development.
	// +kubebuilder:validation:Enum=production;staging;development
	// +kubebuilder:default=development
	Environment string `json:"environment"`
}

// KubePilotClusterStatus describes the observed state of a registered cluster.
type KubePilotClusterStatus struct {
	// Connected indicates the MCP agent in this cluster is live.
	Connected bool `json:"connected"`
	// LastSeenAt is the timestamp of the last agent heartbeat.
	// +optional
	LastSeenAt *metav1.Time `json:"lastSeenAt,omitempty"`
	// AgentVersion is the KubePilot agent version running in the cluster.
	// +optional
	AgentVersion string `json:"agentVersion,omitempty"`
	// Conditions tracks the lifecycle state of the cluster registration.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true

// KubePilotClusterList is a list of KubePilotCluster resources.
type KubePilotClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KubePilotCluster `json:"items"`
}

// ─────────────────────────────────────────
// KubePilotJob CRD
// ─────────────────────────────────────────

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=kpj

// KubePilotJob defines a multi-agent AI job or workflow.
// Jobs can be one-shot or recurring (cron), and may target one or many clusters.
type KubePilotJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KubePilotJobSpec   `json:"spec"`
	Status KubePilotJobStatus `json:"status,omitempty"`
}

// KubePilotJobSpec describes the desired state of a KubePilotJob.
type KubePilotJobSpec struct {
	// Command is the natural language or structured command for the AI agent to execute.
	Command string `json:"command"`
	// Schedule is an optional cron expression for repeating jobs (e.g. "0 */6 * * *").
	// +optional
	Schedule string `json:"schedule,omitempty"`
	// TargetClusters lists the cluster names to run this job on. Empty means all clusters.
	// +optional
	TargetClusters []string `json:"targetClusters,omitempty"`
	// TargetEnvironment restricts execution: production | staging | development.
	// +kubebuilder:validation:Enum=production;staging;development
	// +kubebuilder:default=development
	TargetEnvironment string `json:"targetEnvironment"`
	// ChangeIDRef references the KubePilotCRCode change ID required for production jobs.
	// +optional
	ChangeIDRef string `json:"changeIDRef,omitempty"`
}

// KubePilotJobStatus describes the observed state of a job.
type KubePilotJobStatus struct {
	// Phase is the current lifecycle phase: Pending | Running | Succeeded | Failed | Blocked.
	// +kubebuilder:validation:Enum=Pending;Running;Succeeded;Failed;Blocked
	Phase string `json:"phase,omitempty"`
	// LastRunAt is the time the job last executed.
	// +optional
	LastRunAt *metav1.Time `json:"lastRunAt,omitempty"`
	// Result is a human-readable summary of the last execution.
	// +optional
	Result string `json:"result,omitempty"`
	// Conditions tracks detailed lifecycle conditions.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true

// KubePilotJobList is a list of KubePilotJob resources.
type KubePilotJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KubePilotJob `json:"items"`
}

// ─────────────────────────────────────────
// KubePilotCRCode CRD
// ─────────────────────────────────────────

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=kpcr

// KubePilotCRCode manages production CR (Change Request) codes for production-safe operations.
// The actual code value is stored in a referenced Kubernetes Secret, never in the CRD spec.
type KubePilotCRCode struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KubePilotCRCodeSpec   `json:"spec"`
	Status KubePilotCRCodeStatus `json:"status,omitempty"`
}

// KubePilotCRCodeSpec describes a CR code registration.
type KubePilotCRCodeSpec struct {
	// ChangeID is the unique identifier for this change (e.g. Jira issue key "INFRA-1234").
	ChangeID string `json:"changeID"`
	// SecretRef references the Kubernetes Secret containing the CR code value.
	SecretRef SecretRef `json:"secretRef"`
	// ExpiresAt is the optional time after which this CR code is no longer valid.
	// +optional
	ExpiresAt *metav1.Time `json:"expiresAt,omitempty"`
	// AllowedJobs lists the KubePilotJob names this CR code authorizes.
	// Empty means the code authorizes all jobs bearing this changeID.
	// +optional
	AllowedJobs []string `json:"allowedJobs,omitempty"`
}

// KubePilotCRCodeStatus describes the observed state of a CR code.
type KubePilotCRCodeStatus struct {
	// Active indicates the CR code has not expired and has not been revoked.
	Active bool `json:"active"`
	// UsedAt is the last time this code was successfully used to authorize a production change.
	// +optional
	UsedAt *metav1.Time `json:"usedAt,omitempty"`
}

// +kubebuilder:object:root=true

// KubePilotCRCodeList is a list of KubePilotCRCode resources.
type KubePilotCRCodeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KubePilotCRCode `json:"items"`
}

// ─────────────────────────────────────────
// Shared types
// ─────────────────────────────────────────

// SecretRef is a reference to a named Kubernetes secret in a given namespace.
type SecretRef struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	// Key is the data key within the secret.
	// +kubebuilder:default=cr-code
	Key string `json:"key,omitempty"`
}
