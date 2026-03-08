package k8s

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ResourceMetrics holds CPU/memory requests, limits, and actual usage for a pod.
type ResourceMetrics struct {
	// Requests and limits from the pod spec.
	CPURequest string `json:"cpu_request"`
	CPULimit   string `json:"cpu_limit"`
	MemRequest string `json:"mem_request"`
	MemLimit   string `json:"mem_limit"`
	// Actual usage from the metrics-server API.
	CPUUsage string `json:"cpu_usage"`
	MemUsage string `json:"mem_usage"`
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

// GetPodResourceMetrics fetches resource requests, limits, and live usage for a pod.
func (c *Client) GetPodResourceMetrics(ctx context.Context, namespace, podName string) (*ResourceMetrics, error) {
	pod, err := c.Core.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting pod %s/%s: %w", namespace, podName, err)
	}

	rm := &ResourceMetrics{}

	// Aggregate requests and limits across all containers.
	for _, container := range pod.Spec.Containers {
		if req, ok := container.Resources.Requests["cpu"]; ok {
			rm.CPURequest = req.String()
		}
		if lim, ok := container.Resources.Limits["cpu"]; ok {
			rm.CPULimit = lim.String()
		}
		if req, ok := container.Resources.Requests["memory"]; ok {
			rm.MemRequest = req.String()
		}
		if lim, ok := container.Resources.Limits["memory"]; ok {
			rm.MemLimit = lim.String()
		}
	}

	// Fetch live usage from the metrics-server API.
	if c.Metrics != nil {
		podMetrics, err := c.Metrics.MetricsV1beta1().PodMetricses(namespace).Get(ctx, podName, metav1.GetOptions{})
		if err == nil && len(podMetrics.Containers) > 0 {
			// Aggregate across containers.
			var totalCPU, totalMem int64
			for _, cm := range podMetrics.Containers {
				if cpu, ok := cm.Usage["cpu"]; ok {
					totalCPU += cpu.MilliValue()
				}
				if mem, ok := cm.Usage["memory"]; ok {
					totalMem += mem.Value()
				}
			}
			rm.CPUUsage = fmt.Sprintf("%dm", totalCPU)
			rm.MemUsage = fmt.Sprintf("%dMi", totalMem/(1024*1024))
		}
		// Metrics-server may not be available — that is not a hard error.
	}

	return rm, nil
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
				cpu := nm.Usage["cpu"]
				mem := nm.Usage["memory"]
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
			CPUCapacity:    node.Status.Capacity.Cpu().String(),
			CPUAllocatable: node.Status.Allocatable.Cpu().String(),
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
