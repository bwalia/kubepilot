package k8s

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ServiceGraphNodeKind identifies the resource type of a graph node.
type ServiceGraphNodeKind string

const (
	SGKindIngress     ServiceGraphNodeKind = "Ingress"
	SGKindService     ServiceGraphNodeKind = "Service"
	SGKindDeployment  ServiceGraphNodeKind = "Deployment"
	SGKindStatefulSet ServiceGraphNodeKind = "StatefulSet"
	SGKindDaemonSet   ServiceGraphNodeKind = "DaemonSet"
	SGKindPod         ServiceGraphNodeKind = "Pod"
)

// maxPodsInGraph caps total pod nodes to keep the canvas readable.
const maxPodsInGraph = 60

// SGPortInfo describes a single port exposed by a Service.
type SGPortInfo struct {
	Name       string `json:"name"`
	Port       int32  `json:"port"`
	TargetPort string `json:"target_port"`
	Protocol   string `json:"protocol"`
	NodePort   int32  `json:"node_port,omitempty"`
}

// SGNode is a single resource node in the service topology graph.
type SGNode struct {
	ID        string               `json:"id"` // "Kind/namespace/name"
	Kind      ServiceGraphNodeKind `json:"kind"`
	Name      string               `json:"name"`
	Namespace string               `json:"namespace"`
	// Status is one of: "healthy" | "degraded" | "pending" | "unknown"
	Status string            `json:"status"`
	Labels map[string]string `json:"labels,omitempty"`

	// Service-specific
	ServiceType string       `json:"service_type,omitempty"`
	Ports       []SGPortInfo `json:"ports,omitempty"`
	ClusterIP   string       `json:"cluster_ip,omitempty"`
	ExternalIPs []string     `json:"external_ips,omitempty"`

	// Workload-specific (Deployment / StatefulSet / DaemonSet)
	Replicas      int32  `json:"replicas,omitempty"`
	ReadyReplicas int32  `json:"ready_replicas,omitempty"`
	Image         string `json:"image,omitempty"`

	// Pod-specific
	Phase    string `json:"phase,omitempty"`
	Ready    bool   `json:"ready,omitempty"`
	Restarts int32  `json:"restarts,omitempty"`
	NodeName string `json:"node_name,omitempty"`

	// Ingress-specific
	Host       string `json:"host,omitempty"`
	IngressURL string `json:"ingress_url,omitempty"`
	TLS        bool   `json:"tls,omitempty"`
}

// SGEdge connects two graph nodes by their IDs.
type SGEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// ServiceGraph is the full topology response for the canvas UI.
type ServiceGraph struct {
	Namespace string   `json:"namespace"`
	Nodes     []SGNode `json:"nodes"`
	Edges     []SGEdge `json:"edges"`
}

func sgID(kind ServiceGraphNodeKind, namespace, name string) string {
	return fmt.Sprintf("%s/%s/%s", kind, namespace, name)
}

// GetServiceGraph returns all Ingresses, Services, Deployments, StatefulSets,
// DaemonSets and Pods in namespace, with edges describing routing and
// selection relationships. Pod count is capped at maxPodsInGraph.
func (c *Client) GetServiceGraph(ctx context.Context, namespace string) (*ServiceGraph, error) {
	nodes := make([]SGNode, 0)
	edges := make([]SGEdge, 0)

	added := make(map[string]bool)
	addEdgeSeen := make(map[string]bool)

	addNode := func(n SGNode) {
		if !added[n.ID] {
			added[n.ID] = true
			nodes = append(nodes, n)
		}
	}

	addEdge := func(from, to string) {
		k := from + "->" + to
		if !addEdgeSeen[k] && added[from] && added[to] {
			addEdgeSeen[k] = true
			edges = append(edges, SGEdge{From: from, To: to})
		}
	}

	// ── Pods ──────────────────────────────────────────────────────────────────
	podList, err := c.Core.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing pods: %w", err)
	}

	podCount := 0
	for _, pod := range podList.Items {
		if podCount >= maxPodsInGraph {
			break
		}
		ready := false
		restarts := int32(0)
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.Ready {
				ready = true
			}
			restarts += cs.RestartCount
		}
		phase := string(pod.Status.Phase)
		status := "healthy"
		switch {
		case pod.Status.Phase == corev1.PodPending:
			status = "pending"
		case !ready:
			status = "degraded"
		}
		addNode(SGNode{
			ID:        sgID(SGKindPod, pod.Namespace, pod.Name),
			Kind:      SGKindPod,
			Name:      pod.Name,
			Namespace: pod.Namespace,
			Status:    status,
			Labels:    pod.Labels,
			Phase:     phase,
			Ready:     ready,
			Restarts:  restarts,
			NodeName:  pod.Spec.NodeName,
		})
		podCount++
	}

	// ── Services ──────────────────────────────────────────────────────────────
	svcList, err := c.Core.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing services: %w", err)
	}

	for _, svc := range svcList.Items {
		ports := make([]SGPortInfo, 0, len(svc.Spec.Ports))
		for _, p := range svc.Spec.Ports {
			ports = append(ports, SGPortInfo{
				Name:       p.Name,
				Port:       p.Port,
				TargetPort: p.TargetPort.String(),
				Protocol:   string(p.Protocol),
				NodePort:   p.NodePort,
			})
		}
		extIPs := append([]string{}, svc.Spec.ExternalIPs...)
		if svc.Spec.Type == corev1.ServiceTypeLoadBalancer {
			for _, lbing := range svc.Status.LoadBalancer.Ingress {
				if lbing.IP != "" {
					extIPs = append(extIPs, lbing.IP)
				} else if lbing.Hostname != "" {
					extIPs = append(extIPs, lbing.Hostname)
				}
			}
		}

		svcID := sgID(SGKindService, svc.Namespace, svc.Name)
		addNode(SGNode{
			ID:          svcID,
			Kind:        SGKindService,
			Name:        svc.Name,
			Namespace:   svc.Namespace,
			Status:      "healthy",
			Labels:      svc.Labels,
			ServiceType: string(svc.Spec.Type),
			Ports:       ports,
			ClusterIP:   svc.Spec.ClusterIP,
			ExternalIPs: extIPs,
		})

		// service → matching pods (edges added after workloads are processed)
		for _, pod := range podList.Items {
			if len(svc.Spec.Selector) > 0 && matchLabels(pod.Labels, svc.Spec.Selector) {
				podID := sgID(SGKindPod, pod.Namespace, pod.Name)
				if added[podID] {
					addEdge(svcID, podID)
				}
			}
		}
	}

	// ── Deployments ───────────────────────────────────────────────────────────
	depList, err := c.Core.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing deployments: %w", err)
	}

	for _, dep := range depList.Items {
		image := ""
		if len(dep.Spec.Template.Spec.Containers) > 0 {
			image = dep.Spec.Template.Spec.Containers[0].Image
		}
		status := "healthy"
		if dep.Status.ReadyReplicas < dep.Status.Replicas {
			status = "degraded"
		}
		depID := sgID(SGKindDeployment, dep.Namespace, dep.Name)
		addNode(SGNode{
			ID:            depID,
			Kind:          SGKindDeployment,
			Name:          dep.Name,
			Namespace:     dep.Namespace,
			Status:        status,
			Labels:        dep.Labels,
			Replicas:      dep.Status.Replicas,
			ReadyReplicas: dep.Status.ReadyReplicas,
			Image:         image,
		})
		// service → deployment when template labels satisfy service selector
		for _, svc := range svcList.Items {
			if len(svc.Spec.Selector) > 0 && matchLabels(dep.Spec.Template.Labels, svc.Spec.Selector) {
				addEdge(sgID(SGKindService, svc.Namespace, svc.Name), depID)
			}
		}
		// deployment → owned pods
		for _, pod := range podList.Items {
			if dep.Spec.Selector != nil && matchLabels(pod.Labels, dep.Spec.Selector.MatchLabels) {
				podID := sgID(SGKindPod, pod.Namespace, pod.Name)
				if added[podID] {
					addEdge(depID, podID)
				}
			}
		}
	}

	// ── StatefulSets ──────────────────────────────────────────────────────────
	ssList, err := c.Core.AppsV1().StatefulSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing statefulsets: %w", err)
	}

	for _, ss := range ssList.Items {
		image := ""
		if len(ss.Spec.Template.Spec.Containers) > 0 {
			image = ss.Spec.Template.Spec.Containers[0].Image
		}
		desired := int32(1)
		if ss.Spec.Replicas != nil {
			desired = *ss.Spec.Replicas
		}
		status := "healthy"
		if ss.Status.ReadyReplicas < desired {
			status = "degraded"
		}
		ssID := sgID(SGKindStatefulSet, ss.Namespace, ss.Name)
		addNode(SGNode{
			ID:            ssID,
			Kind:          SGKindStatefulSet,
			Name:          ss.Name,
			Namespace:     ss.Namespace,
			Status:        status,
			Labels:        ss.Labels,
			Replicas:      desired,
			ReadyReplicas: ss.Status.ReadyReplicas,
			Image:         image,
		})
		for _, svc := range svcList.Items {
			if len(svc.Spec.Selector) > 0 && matchLabels(ss.Spec.Template.Labels, svc.Spec.Selector) {
				addEdge(sgID(SGKindService, svc.Namespace, svc.Name), ssID)
			}
		}
		if ss.Spec.Selector != nil {
			for _, pod := range podList.Items {
				if matchLabels(pod.Labels, ss.Spec.Selector.MatchLabels) {
					podID := sgID(SGKindPod, pod.Namespace, pod.Name)
					if added[podID] {
						addEdge(ssID, podID)
					}
				}
			}
		}
	}

	// ── DaemonSets ────────────────────────────────────────────────────────────
	dsList, err := c.Core.AppsV1().DaemonSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing daemonsets: %w", err)
	}

	for _, ds := range dsList.Items {
		image := ""
		if len(ds.Spec.Template.Spec.Containers) > 0 {
			image = ds.Spec.Template.Spec.Containers[0].Image
		}
		status := "healthy"
		if ds.Status.NumberReady < ds.Status.DesiredNumberScheduled {
			status = "degraded"
		}
		dsID := sgID(SGKindDaemonSet, ds.Namespace, ds.Name)
		addNode(SGNode{
			ID:            dsID,
			Kind:          SGKindDaemonSet,
			Name:          ds.Name,
			Namespace:     ds.Namespace,
			Status:        status,
			Labels:        ds.Labels,
			Replicas:      ds.Status.DesiredNumberScheduled,
			ReadyReplicas: ds.Status.NumberReady,
			Image:         image,
		})
		for _, svc := range svcList.Items {
			if len(svc.Spec.Selector) > 0 && matchLabels(ds.Spec.Template.Labels, svc.Spec.Selector) {
				addEdge(sgID(SGKindService, svc.Namespace, svc.Name), dsID)
			}
		}
		if ds.Spec.Selector != nil {
			for _, pod := range podList.Items {
				if matchLabels(pod.Labels, ds.Spec.Selector.MatchLabels) {
					podID := sgID(SGKindPod, pod.Namespace, pod.Name)
					if added[podID] {
						addEdge(dsID, podID)
					}
				}
			}
		}
	}

	// ── Ingresses ─────────────────────────────────────────────────────────────
	ingList, err := c.Core.NetworkingV1().Ingresses(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing ingresses: %w", err)
	}

	for _, ing := range ingList.Items {
		host := ""
		if len(ing.Spec.Rules) > 0 {
			host = ing.Spec.Rules[0].Host
		}
		scheme := "http"
		if len(ing.Spec.TLS) > 0 {
			scheme = "https"
		}
		ingressURL := ""
		if host != "" {
			ingressURL = scheme + "://" + host
		}
		ingID := sgID(SGKindIngress, ing.Namespace, ing.Name)
		addNode(SGNode{
			ID:         ingID,
			Kind:       SGKindIngress,
			Name:       ing.Name,
			Namespace:  ing.Namespace,
			Status:     "healthy",
			Labels:     ing.Labels,
			Host:       host,
			IngressURL: ingressURL,
			TLS:        len(ing.Spec.TLS) > 0,
		})
		for _, rule := range ing.Spec.Rules {
			if rule.HTTP == nil {
				continue
			}
			for _, path := range rule.HTTP.Paths {
				if path.Backend.Service != nil {
					targetSvcID := sgID(SGKindService, ing.Namespace, path.Backend.Service.Name)
					addEdge(ingID, targetSvcID)
				}
			}
		}
	}

	return &ServiceGraph{
		Namespace: namespace,
		Nodes:     nodes,
		Edges:     edges,
	}, nil
}
