package k8s

import (
	"context"
	"fmt"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NetworkPolicySummary is a concise view of a Kubernetes NetworkPolicy.
type NetworkPolicySummary struct {
	Name      string   `json:"name"`
	Namespace string   `json:"namespace"`
	PodSelector string `json:"pod_selector"`
	PolicyTypes []string `json:"policy_types"`
	IngressRules int   `json:"ingress_rules"`
	EgressRules  int   `json:"egress_rules"`
}

// ServiceEndpoint describes a Service and its backing endpoints for topology mapping.
type ServiceEndpoint struct {
	Name       string   `json:"name"`
	Namespace  string   `json:"namespace"`
	Type       string   `json:"type"` // ClusterIP, NodePort, LoadBalancer
	ClusterIP  string   `json:"cluster_ip"`
	Ports      []PortInfo `json:"ports"`
	Selector   map[string]string `json:"selector"`
	EndpointIPs []string `json:"endpoint_ips,omitempty"`
}

// PortInfo holds service port metadata.
type PortInfo struct {
	Name       string `json:"name"`
	Port       int32  `json:"port"`
	TargetPort string `json:"target_port"`
	Protocol   string `json:"protocol"`
}

// ListNetworkPolicies returns all NetworkPolicies in a namespace (or all namespaces if empty).
func (c *Client) ListNetworkPolicies(ctx context.Context, namespace string) ([]NetworkPolicySummary, error) {
	list, err := c.Core.NetworkingV1().NetworkPolicies(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing network policies in namespace %q: %w", namespace, err)
	}

	summaries := make([]NetworkPolicySummary, 0, len(list.Items))
	for _, np := range list.Items {
		summaries = append(summaries, toNetworkPolicySummary(np))
	}
	return summaries, nil
}

// ListServices returns services with their endpoint information for topology analysis.
func (c *Client) ListServices(ctx context.Context, namespace string) ([]ServiceEndpoint, error) {
	svcList, err := c.Core.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing services in namespace %q: %w", namespace, err)
	}

	// Pre-fetch endpoints for resolving backing pod IPs.
	epList, err := c.Core.CoreV1().Endpoints(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing endpoints in namespace %q: %w", namespace, err)
	}
	epMap := make(map[string][]string, len(epList.Items))
	for _, ep := range epList.Items {
		var ips []string
		for _, subset := range ep.Subsets {
			for _, addr := range subset.Addresses {
				ips = append(ips, addr.IP)
			}
		}
		epMap[ep.Namespace+"/"+ep.Name] = ips
	}

	results := make([]ServiceEndpoint, 0, len(svcList.Items))
	for _, svc := range svcList.Items {
		se := ServiceEndpoint{
			Name:      svc.Name,
			Namespace: svc.Namespace,
			Type:      string(svc.Spec.Type),
			ClusterIP: svc.Spec.ClusterIP,
			Selector:  svc.Spec.Selector,
		}

		for _, p := range svc.Spec.Ports {
			se.Ports = append(se.Ports, PortInfo{
				Name:       p.Name,
				Port:       p.Port,
				TargetPort: p.TargetPort.String(),
				Protocol:   string(p.Protocol),
			})
		}

		key := svc.Namespace + "/" + svc.Name
		if ips, ok := epMap[key]; ok {
			se.EndpointIPs = ips
		}

		results = append(results, se)
	}
	return results, nil
}

func toNetworkPolicySummary(np networkingv1.NetworkPolicy) NetworkPolicySummary {
	policyTypes := make([]string, 0, len(np.Spec.PolicyTypes))
	for _, pt := range np.Spec.PolicyTypes {
		policyTypes = append(policyTypes, string(pt))
	}

	podSelector := ""
	if len(np.Spec.PodSelector.MatchLabels) > 0 {
		podSelector = fmt.Sprintf("%v", np.Spec.PodSelector.MatchLabels)
	}

	return NetworkPolicySummary{
		Name:        np.Name,
		Namespace:   np.Namespace,
		PodSelector: podSelector,
		PolicyTypes: policyTypes,
		IngressRules: len(np.Spec.Ingress),
		EgressRules:  len(np.Spec.Egress),
	}
}
