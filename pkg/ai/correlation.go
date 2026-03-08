package ai

import (
	"context"
	"fmt"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"
	"go.uber.org/zap"

	"github.com/kubepilot/kubepilot/pkg/k8s"
)

// ServiceNode represents a service in the dependency topology.
type ServiceNode struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Pods      []string          `json:"pods"`
	Healthy   bool              `json:"healthy"`
	Anomalies []string          `json:"anomalies,omitempty"` // Anomaly descriptions
}

// DependencyEdge represents a connection between two services.
type DependencyEdge struct {
	From string `json:"from"` // "namespace/service"
	To   string `json:"to"`
	Type string `json:"type"` // "http", "tcp", "dns"
	Port int32  `json:"port"`
}

// ServiceTopology maps the service dependency graph in a namespace.
type ServiceTopology struct {
	Services []ServiceNode  `json:"services"`
	Edges    []DependencyEdge `json:"edges"`
}

// TimelineEvent is a timestamped event in a correlation timeline.
type TimelineEvent struct {
	Timestamp   time.Time `json:"timestamp"`
	Service     string    `json:"service"`
	Event       string    `json:"event"`
	Severity    string    `json:"severity"`
}

// CorrelationReport links failures across dependent services.
type CorrelationReport struct {
	ID              string          `json:"id"`
	Timestamp       time.Time       `json:"timestamp"`
	RootService     string          `json:"root_service"`
	AffectedChain   []string        `json:"affected_chain"`
	Topology        *ServiceTopology `json:"topology"`
	TimelineEvents  []TimelineEvent `json:"timeline_events"`
	LLMAnalysis     string          `json:"llm_analysis"`
}

// CorrelationEngine builds service dependency graphs and correlates failures
// across the dependency chain.
type CorrelationEngine struct {
	engine *Engine
	k8s    *k8s.Client
	log    *zap.Logger
}

// NewCorrelationEngine creates a CorrelationEngine.
func NewCorrelationEngine(engine *Engine, k8sClient *k8s.Client, log *zap.Logger) *CorrelationEngine {
	return &CorrelationEngine{
		engine: engine,
		k8s:    k8sClient,
		log:    log,
	}
}

// BuildTopology discovers the service topology for a namespace by examining
// Services, Endpoints, and pod relationships.
func (ce *CorrelationEngine) BuildTopology(ctx context.Context, namespace string) (*ServiceTopology, error) {
	services, err := ce.k8s.ListServices(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("listing services for topology: %w", err)
	}

	pods, err := ce.k8s.ListPods(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("listing pods for topology: %w", err)
	}

	// Build a map of pod IP → pod name for endpoint resolution.
	podsByName := make(map[string]k8s.PodSummary)
	for _, p := range pods {
		podsByName[p.Name] = p
	}

	topology := &ServiceTopology{}

	for _, svc := range services {
		node := ServiceNode{
			Name:      svc.Name,
			Namespace: svc.Namespace,
			Healthy:   true,
		}

		// Find pods matching the service selector.
		for _, pod := range pods {
			// Check if pod matches service selector by label prefix matching.
			if matchesSelector(pod, svc.Selector) {
				node.Pods = append(node.Pods, pod.Name)
				if !pod.Ready || pod.Reason != "" {
					node.Healthy = false
					node.Anomalies = append(node.Anomalies,
						fmt.Sprintf("Pod %s: %s (restarts: %d)", pod.Name, pod.Reason, pod.Restarts))
				}
			}
		}

		topology.Services = append(topology.Services, node)

		// Infer edges from service port mappings.
		for _, port := range svc.Ports {
			edge := DependencyEdge{
				From: fmt.Sprintf("%s/%s", svc.Namespace, svc.Name),
				Type: strings.ToLower(port.Protocol),
				Port: port.Port,
			}
			// Edges to other services would require analyzing pod environment variables
			// or service mesh config — for now, we represent the service's own port exposure.
			topology.Edges = append(topology.Edges, edge)
		}
	}

	return topology, nil
}

// CorrelateFailures analyzes service topology and identifies cascading failures.
func (ce *CorrelationEngine) CorrelateFailures(ctx context.Context, namespace string) (*CorrelationReport, error) {
	topology, err := ce.BuildTopology(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("building topology: %w", err)
	}

	// Collect events for the namespace to build a timeline.
	events, err := ce.k8s.GetWarningEvents(ctx, namespace, 30*time.Minute)
	if err != nil {
		ce.log.Warn("Could not get events for correlation", zap.Error(err))
	}

	// Build a timeline from events.
	var timeline []TimelineEvent
	for _, e := range events {
		timeline = append(timeline, TimelineEvent{
			Timestamp: e.LastSeen,
			Service:   e.InvolvedObject.Name,
			Event:     fmt.Sprintf("%s: %s", e.Reason, e.Message),
			Severity:  e.Type,
		})
	}

	// Identify unhealthy services.
	var unhealthy []string
	for _, svc := range topology.Services {
		if !svc.Healthy {
			unhealthy = append(unhealthy, svc.Name)
		}
	}

	report := &CorrelationReport{
		ID:             fmt.Sprintf("corr-%s-%d", namespace, time.Now().Unix()),
		Timestamp:      time.Now().UTC(),
		Topology:       topology,
		TimelineEvents: timeline,
		AffectedChain:  unhealthy,
	}

	if len(unhealthy) > 0 {
		report.RootService = unhealthy[0]
	}

	// Use LLM to analyze the correlation if there are failures.
	if len(unhealthy) > 0 {
		analysis, err := ce.llmCorrelate(ctx, topology, timeline, unhealthy)
		if err != nil {
			ce.log.Warn("LLM correlation analysis failed", zap.Error(err))
		} else {
			report.LLMAnalysis = analysis
		}
	}

	return report, nil
}

// llmCorrelate asks the LLM to analyze cascading failures.
func (ce *CorrelationEngine) llmCorrelate(ctx context.Context, topology *ServiceTopology, timeline []TimelineEvent, unhealthy []string) (string, error) {
	var sb strings.Builder
	sb.WriteString("Analyze the following Kubernetes service topology for cascading failure correlation:\n\n")

	sb.WriteString("## Services\n")
	for _, svc := range topology.Services {
		status := "healthy"
		if !svc.Healthy {
			status = "UNHEALTHY"
		}
		sb.WriteString(fmt.Sprintf("- %s/%s [%s] pods=%d\n", svc.Namespace, svc.Name, status, len(svc.Pods)))
		for _, a := range svc.Anomalies {
			sb.WriteString(fmt.Sprintf("  Anomaly: %s\n", a))
		}
	}

	if len(timeline) > 0 {
		sb.WriteString("\n## Event Timeline (most recent 30min)\n")
		maxEvents := 20
		if len(timeline) < maxEvents {
			maxEvents = len(timeline)
		}
		for _, e := range timeline[:maxEvents] {
			sb.WriteString(fmt.Sprintf("- [%s] %s: %s (%s)\n",
				e.Timestamp.Format(time.RFC3339), e.Service, e.Event, e.Severity))
		}
	}

	sb.WriteString(fmt.Sprintf("\n## Unhealthy Services: %s\n", strings.Join(unhealthy, ", ")))
	sb.WriteString("\nIdentify the root cause service, the failure cascade chain, and recommended fixes.")

	systemPrompt := `You are a Kubernetes service topology expert analyzing cascading failures.
Identify which service is the root cause, which services are affected downstream, and suggest remediation.
Be concise and specific.`

	resp, err := ce.engine.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: ce.engine.cfg.Model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: sb.String()},
		},
		Temperature: 0.2,
	})
	if err != nil {
		return "", err
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("empty LLM response for correlation")
	}

	return resp.Choices[0].Message.Content, nil
}

// matchesSelector checks if a pod matches a service's label selector.
func matchesSelector(_ k8s.PodSummary, _ map[string]string) bool {
	// PodSummary doesn't include labels — we match all pods in the namespace.
	// This will be refined when PodSummary includes labels or we use the full Pod object.
	// For now, return true to show topology of all services with all pods.
	return true
}
