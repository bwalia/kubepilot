package k8s

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ResourceMetrics holds CPU/memory requests, limits, and actual usage for a pod.
//
// CPU fields are always millicores with an "m" suffix (e.g. "50m", "1000m") so
// consumers — including LLMs — never confuse a bare core count ("1") with one
// millicore ("1m"). Memory request/limit keep Kubernetes Quantity.String()
// form; usage is rounded down to whole MiB for readability.
type ResourceMetrics struct {
	// Requests and limits from the pod spec (summed across containers).
	CPURequest string `json:"cpu_request"`
	CPULimit   string `json:"cpu_limit"`
	MemRequest string `json:"mem_request"`
	MemLimit   string `json:"mem_limit"`
	// Actual usage from the metrics-server API.
	CPUUsage string `json:"cpu_usage"`
	MemUsage string `json:"mem_usage"`
	// Optional utilization ratios (0–100+). Omitted when limit or usage is missing.
	CPUUsageOfLimitPct *float64 `json:"cpu_usage_of_limit_pct,omitempty"`
	MemUsageOfLimitPct *float64 `json:"mem_usage_of_limit_pct,omitempty"`
}

// NodeResourceMetrics holds node capacity, allocatable, and usage information.
type NodeResourceMetrics struct {
	Name           string `json:"name"`
	CPUCapacity    string `json:"cpu_capacity"`
	CPUAllocatable string `json:"cpu_allocatable"`
	CPUUsage       string `json:"cpu_usage"`
	MemCapacity    string `json:"mem_capacity"`
	MemAllocatable string `json:"mem_allocatable"`
	MemUsage       string `json:"mem_usage"`
}

// FormatCPUMilli renders a CPU Quantity as millicores with an "m" suffix.
// Example: 1 core → "1000m", 50m → "50m".
func FormatCPUMilli(q resource.Quantity) string {
	return fmt.Sprintf("%dm", q.MilliValue())
}

// GetPodResourceMetrics fetches resource requests, limits, and live usage for a pod.
func (c *Client) GetPodResourceMetrics(ctx context.Context, namespace, podName string) (*ResourceMetrics, error) {
	pod, err := c.Core.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting pod %s/%s: %w", namespace, podName, err)
	}

	rm := sumContainerResources(pod.Spec.Containers)

	// Fetch live usage from the metrics-server API.
	if c.Metrics != nil {
		podMetrics, err := c.Metrics.MetricsV1beta1().PodMetricses(namespace).Get(ctx, podName, metav1.GetOptions{})
		if err == nil && len(podMetrics.Containers) > 0 {
			var totalCPU, totalMem int64
			for _, cm := range podMetrics.Containers {
				if cpu, ok := cm.Usage[corev1.ResourceCPU]; ok {
					totalCPU += cpu.MilliValue()
				}
				if mem, ok := cm.Usage[corev1.ResourceMemory]; ok {
					totalMem += mem.Value()
				}
			}
			rm.CPUUsage = fmt.Sprintf("%dm", totalCPU)
			rm.MemUsage = fmt.Sprintf("%dMi", totalMem/(1024*1024))
		}
		// Metrics-server may not be available — that is not a hard error.
	}

	rm.CPUUsageOfLimitPct = usagePercent(rm.CPUUsage, rm.CPULimit)
	rm.MemUsageOfLimitPct = usagePercent(rm.MemUsage, rm.MemLimit)

	return rm, nil
}

// sumContainerResources aggregates requests/limits across containers into
// millicore CPU strings and Quantity.String() memory strings.
func sumContainerResources(containers []corev1.Container) *ResourceMetrics {
	rm := &ResourceMetrics{}
	var cpuReq, cpuLim, memReq, memLim resource.Quantity
	var haveCPUReq, haveCPULim, haveMemReq, haveMemLim bool

	for _, container := range containers {
		if req, ok := container.Resources.Requests[corev1.ResourceCPU]; ok {
			cpuReq.Add(req)
			haveCPUReq = true
		}
		if lim, ok := container.Resources.Limits[corev1.ResourceCPU]; ok {
			cpuLim.Add(lim)
			haveCPULim = true
		}
		if req, ok := container.Resources.Requests[corev1.ResourceMemory]; ok {
			memReq.Add(req)
			haveMemReq = true
		}
		if lim, ok := container.Resources.Limits[corev1.ResourceMemory]; ok {
			memLim.Add(lim)
			haveMemLim = true
		}
	}

	if haveCPUReq {
		rm.CPURequest = FormatCPUMilli(cpuReq)
	}
	if haveCPULim {
		rm.CPULimit = FormatCPUMilli(cpuLim)
	}
	if haveMemReq {
		rm.MemRequest = memReq.String()
	}
	if haveMemLim {
		rm.MemLimit = memLim.String()
	}
	return rm
}

func usagePercent(usage, limit string) *float64 {
	if usage == "" || limit == "" {
		return nil
	}
	u, err1 := resource.ParseQuantity(usage)
	l, err2 := resource.ParseQuantity(limit)
	if err1 != nil || err2 != nil || l.IsZero() {
		return nil
	}
	// Both quantities share a unit (cores or bytes), so their float ratio is
	// the utilisation regardless of suffix.
	pct := (u.AsApproximateFloat64() / l.AsApproximateFloat64()) * 100
	return &pct
}

// GetNodeResourceMetrics fetches resource capacity, allocatable, and live usage for all nodes.
func (c *Client) GetNodeResourceMetrics(ctx context.Context) ([]NodeResourceMetrics, error) {
	nodeList, err := c.Core.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing nodes: %w", err)
	}

	results := make([]NodeResourceMetrics, 0, len(nodeList.Items))

	// Fetch node metrics if metrics-server is available.
	var nodeMetricsMap map[string]*struct{ CPU, Mem int64 }
	if c.Metrics != nil {
		nmList, err := c.Metrics.MetricsV1beta1().NodeMetricses().List(ctx, metav1.ListOptions{})
		if err == nil {
			nodeMetricsMap = make(map[string]*struct{ CPU, Mem int64 }, len(nmList.Items))
			for _, nm := range nmList.Items {
				cpu := nm.Usage[corev1.ResourceCPU]
				mem := nm.Usage[corev1.ResourceMemory]
				nodeMetricsMap[nm.Name] = &struct{ CPU, Mem int64 }{
					CPU: cpu.MilliValue(),
					Mem: mem.Value(),
				}
			}
		}
	}

	for _, node := range nodeList.Items {
		nrm := NodeResourceMetrics{
			Name:           node.Name,
			CPUCapacity:    FormatCPUMilli(node.Status.Capacity[corev1.ResourceCPU]),
			CPUAllocatable: FormatCPUMilli(node.Status.Allocatable[corev1.ResourceCPU]),
			MemCapacity:    node.Status.Capacity.Memory().String(),
			MemAllocatable: node.Status.Allocatable.Memory().String(),
		}
		if nodeMetricsMap != nil {
			if usage, ok := nodeMetricsMap[node.Name]; ok {
				nrm.CPUUsage = fmt.Sprintf("%dm", usage.CPU)
				nrm.MemUsage = fmt.Sprintf("%dMi", usage.Mem/(1024*1024))
			}
		}
		results = append(results, nrm)
	}

	return results, nil
}
