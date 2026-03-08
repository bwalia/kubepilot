package observability

import (
	"context"
	"fmt"
	"time"

	"github.com/kubepilot/kubepilot/pkg/ai"
	"github.com/kubepilot/kubepilot/pkg/k8s"
)

// Anomaly represents a detected cluster issue.
type Anomaly struct {
	ID          string          `json:"id"`
	DetectedAt  time.Time       `json:"detected_at"`
	Rule        string          `json:"rule"`
	Resource    ai.ResourceRef  `json:"resource"`
	Severity    ai.Severity     `json:"severity"`
	Description string          `json:"description"`
	RCAReportID string          `json:"rca_report_id,omitempty"`
}

// AnomalyRule is the interface for pluggable anomaly detection rules.
type AnomalyRule interface {
	Name() string
	Evaluate(ctx context.Context, snapshot *k8s.ClusterSnapshot) ([]Anomaly, error)
}

// CrashLoopDetector detects pods in CrashLoopBackOff state.
type CrashLoopDetector struct {
	MinRestarts int32
}

func (d *CrashLoopDetector) Name() string { return "CrashLoopDetector" }

func (d *CrashLoopDetector) Evaluate(_ context.Context, snapshot *k8s.ClusterSnapshot) ([]Anomaly, error) {
	minRestarts := d.MinRestarts
	if minRestarts <= 0 {
		minRestarts = 3
	}

	var anomalies []Anomaly
	for _, pod := range snapshot.Pods {
		if pod.Reason == "CrashLoopBackOff" || (pod.Restarts >= minRestarts && !pod.Ready) {
			anomalies = append(anomalies, Anomaly{
				ID:         fmt.Sprintf("crashloop-%s-%s-%d", pod.Namespace, pod.Name, time.Now().Unix()),
				DetectedAt: time.Now().UTC(),
				Rule:       d.Name(),
				Resource: ai.ResourceRef{
					Kind:      "Pod",
					Name:      pod.Name,
					Namespace: pod.Namespace,
				},
				Severity:    ai.SeverityHigh,
				Description: fmt.Sprintf("Pod %s/%s is in CrashLoopBackOff with %d restarts", pod.Namespace, pod.Name, pod.Restarts),
			})
		}
	}
	return anomalies, nil
}

// OOMDetector detects pods killed by OOM.
type OOMDetector struct{}

func (d *OOMDetector) Name() string { return "OOMDetector" }

func (d *OOMDetector) Evaluate(_ context.Context, snapshot *k8s.ClusterSnapshot) ([]Anomaly, error) {
	var anomalies []Anomaly
	for _, pod := range snapshot.Pods {
		if pod.Reason == "OOMKilled" {
			anomalies = append(anomalies, Anomaly{
				ID:         fmt.Sprintf("oom-%s-%s-%d", pod.Namespace, pod.Name, time.Now().Unix()),
				DetectedAt: time.Now().UTC(),
				Rule:       d.Name(),
				Resource: ai.ResourceRef{
					Kind:      "Pod",
					Name:      pod.Name,
					Namespace: pod.Namespace,
				},
				Severity:    ai.SeverityCritical,
				Description: fmt.Sprintf("Pod %s/%s was OOMKilled", pod.Namespace, pod.Name),
			})
		}
	}
	return anomalies, nil
}

// PendingPodDetector detects pods stuck in Pending state.
type PendingPodDetector struct {
	// ThresholdMinutes is how long a pod must be Pending before flagging.
	ThresholdMinutes int
}

func (d *PendingPodDetector) Name() string { return "PendingPodDetector" }

func (d *PendingPodDetector) Evaluate(_ context.Context, snapshot *k8s.ClusterSnapshot) ([]Anomaly, error) {
	var anomalies []Anomaly
	for _, pod := range snapshot.Pods {
		if string(pod.Phase) == "Pending" {
			anomalies = append(anomalies, Anomaly{
				ID:         fmt.Sprintf("pending-%s-%s-%d", pod.Namespace, pod.Name, time.Now().Unix()),
				DetectedAt: time.Now().UTC(),
				Rule:       d.Name(),
				Resource: ai.ResourceRef{
					Kind:      "Pod",
					Name:      pod.Name,
					Namespace: pod.Namespace,
				},
				Severity:    ai.SeverityMedium,
				Description: fmt.Sprintf("Pod %s/%s stuck in Pending state", pod.Namespace, pod.Name),
			})
		}
	}
	return anomalies, nil
}

// NodePressureDetector detects nodes under resource pressure.
type NodePressureDetector struct{}

func (d *NodePressureDetector) Name() string { return "NodePressureDetector" }

func (d *NodePressureDetector) Evaluate(_ context.Context, snapshot *k8s.ClusterSnapshot) ([]Anomaly, error) {
	var anomalies []Anomaly
	for _, node := range snapshot.Nodes {
		if !node.Ready {
			anomalies = append(anomalies, Anomaly{
				ID:         fmt.Sprintf("node-notready-%s-%d", node.Name, time.Now().Unix()),
				DetectedAt: time.Now().UTC(),
				Rule:       d.Name(),
				Resource: ai.ResourceRef{
					Kind: "Node",
					Name: node.Name,
				},
				Severity:    ai.SeverityCritical,
				Description: fmt.Sprintf("Node %s is not Ready", node.Name),
			})
		}
		if node.MemoryPressure || node.DiskPressure || node.PIDPressure {
			pressures := ""
			if node.MemoryPressure {
				pressures += "Memory "
			}
			if node.DiskPressure {
				pressures += "Disk "
			}
			if node.PIDPressure {
				pressures += "PID "
			}
			anomalies = append(anomalies, Anomaly{
				ID:         fmt.Sprintf("node-pressure-%s-%d", node.Name, time.Now().Unix()),
				DetectedAt: time.Now().UTC(),
				Rule:       d.Name(),
				Resource: ai.ResourceRef{
					Kind: "Node",
					Name: node.Name,
				},
				Severity:    ai.SeverityHigh,
				Description: fmt.Sprintf("Node %s has resource pressure: %s", node.Name, pressures),
			})
		}
	}
	return anomalies, nil
}

// FailedEventDetector detects high-frequency Warning events.
type FailedEventDetector struct {
	MinCount int32
}

func (d *FailedEventDetector) Name() string { return "FailedEventDetector" }

func (d *FailedEventDetector) Evaluate(_ context.Context, snapshot *k8s.ClusterSnapshot) ([]Anomaly, error) {
	minCount := d.MinCount
	if minCount <= 0 {
		minCount = 5
	}

	var anomalies []Anomaly
	for _, event := range snapshot.Events {
		if event.Type == "Warning" && event.Count >= minCount {
			anomalies = append(anomalies, Anomaly{
				ID:         fmt.Sprintf("event-%s-%s-%d", event.InvolvedObject.Name, event.Reason, time.Now().Unix()),
				DetectedAt: time.Now().UTC(),
				Rule:       d.Name(),
				Resource: ai.ResourceRef{
					Kind:      event.InvolvedObject.Kind,
					Name:      event.InvolvedObject.Name,
					Namespace: event.InvolvedObject.Namespace,
				},
				Severity:    ai.SeverityMedium,
				Description: fmt.Sprintf("Repeated warning event: %s - %s (count: %d)", event.Reason, event.Message, event.Count),
			})
		}
	}
	return anomalies, nil
}

// ImagePullDetector detects pods with image pull errors.
type ImagePullDetector struct{}

func (d *ImagePullDetector) Name() string { return "ImagePullDetector" }

func (d *ImagePullDetector) Evaluate(_ context.Context, snapshot *k8s.ClusterSnapshot) ([]Anomaly, error) {
	var anomalies []Anomaly
	for _, pod := range snapshot.Pods {
		if pod.Reason == "ImagePullBackOff" || pod.Reason == "ErrImagePull" {
			anomalies = append(anomalies, Anomaly{
				ID:         fmt.Sprintf("imagepull-%s-%s-%d", pod.Namespace, pod.Name, time.Now().Unix()),
				DetectedAt: time.Now().UTC(),
				Rule:       d.Name(),
				Resource: ai.ResourceRef{
					Kind:      "Pod",
					Name:      pod.Name,
					Namespace: pod.Namespace,
				},
				Severity:    ai.SeverityHigh,
				Description: fmt.Sprintf("Pod %s/%s has image pull error: %s", pod.Namespace, pod.Name, pod.Reason),
			})
		}
	}
	return anomalies, nil
}

// DefaultRules returns the standard set of anomaly detection rules.
func DefaultRules() []AnomalyRule {
	return []AnomalyRule{
		&CrashLoopDetector{MinRestarts: 3},
		&OOMDetector{},
		&PendingPodDetector{},
		&NodePressureDetector{},
		&FailedEventDetector{MinCount: 5},
		&ImagePullDetector{},
	}
}
