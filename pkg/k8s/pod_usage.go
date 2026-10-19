package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PodUsage is one pod's live resource consumption next to what it asked for.
// All CPU values are millicores and all memory/storage values are bytes, so
// the dashboard can sort and sum them without parsing Quantity strings.
type PodUsage struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Node      string `json:"node"`
	Phase     string `json:"phase"`
	// OwnerKind/OwnerName name the workload the pod belongs to. ReplicaSets are
	// resolved to their Deployment so a rollout's pods group together.
	OwnerKind string `json:"owner_kind,omitempty"`
	OwnerName string `json:"owner_name,omitempty"`
	Restarts  int32  `json:"restarts"`

	CPUUsageMilli   int64 `json:"cpu_usage_milli"`
	CPURequestMilli int64 `json:"cpu_request_milli"`
	CPULimitMilli   int64 `json:"cpu_limit_milli"`

	MemUsageBytes   int64 `json:"mem_usage_bytes"`
	MemRequestBytes int64 `json:"mem_request_bytes"`
	MemLimitBytes   int64 `json:"mem_limit_bytes"`

	// StorageUsedBytes is ephemeral (container rootfs + logs + emptyDir) plus
	// the used bytes of every PVC the pod mounts, as reported by the kubelet.
	// When kubelet stats are unreachable it falls back to the claimed PVC
	// capacity, and StorageEstimated is set.
	StorageUsedBytes    int64 `json:"storage_used_bytes"`
	EphemeralUsedBytes  int64 `json:"ephemeral_used_bytes"`
	PVCUsedBytes        int64 `json:"pvc_used_bytes"`
	PVCCapacityBytes    int64 `json:"pvc_capacity_bytes"`
	EphemeralLimitBytes int64 `json:"ephemeral_limit_bytes,omitempty"`
	StorageEstimated    bool  `json:"storage_estimated,omitempty"`
	PVCCount            int   `json:"pvc_count,omitempty"`
}

// PodUsageReport is the full per-pod usage list plus which data sources
// answered, so the UI can say "metrics-server unavailable" instead of
// silently showing zeros.
type PodUsageReport struct {
	Pods                  []PodUsage `json:"pods"`
	MetricsAvailable      bool       `json:"metrics_available"`
	StorageStatsAvailable bool       `json:"storage_stats_available"`
	GeneratedAt           time.Time  `json:"generated_at"`
}

// kubeletSummary is the subset of the kubelet /stats/summary response we read.
type kubeletSummary struct {
	Pods []struct {
		PodRef struct {
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
		} `json:"podRef"`
		EphemeralStorage *struct {
			UsedBytes *int64 `json:"usedBytes"`
		} `json:"ephemeral-storage"`
		Volume []struct {
			Name          string `json:"name"`
			UsedBytes     *int64 `json:"usedBytes"`
			CapacityBytes *int64 `json:"capacityBytes"`
			PVCRef        *struct {
				Name string `json:"name"`
			} `json:"pvcRef"`
		} `json:"volume"`
	} `json:"pods"`
}

type podStorageStats struct {
	ephemeral int64
	volumes   []pvcVolumeStats
}

type pvcVolumeStats struct {
	claim       string
	used        int64
	capacity    int64
	hasUsed     bool
	hasCapacity bool
}

// ListPodUsage returns live CPU, memory and storage consumption for every pod
// in namespace ("" for all namespaces). Missing metrics sources degrade to
// zeros and are flagged on the report rather than failing the call.
func (c *Client) ListPodUsage(ctx context.Context, namespace string) (*PodUsageReport, error) {
	pods, err := c.Core.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing pods: %w", err)
	}

	report := &PodUsageReport{GeneratedAt: time.Now().UTC()}

	type usage struct{ cpu, mem int64 }
	live := map[string]usage{}
	if c.Metrics != nil {
		pml, err := c.Metrics.MetricsV1beta1().PodMetricses(namespace).List(ctx, metav1.ListOptions{})
		if err == nil {
			report.MetricsAvailable = true
			for _, pm := range pml.Items {
				var u usage
				for _, cm := range pm.Containers {
					if q, ok := cm.Usage[corev1.ResourceCPU]; ok {
						u.cpu += q.MilliValue()
					}
					if q, ok := cm.Usage[corev1.ResourceMemory]; ok {
						u.mem += q.Value()
					}
				}
				live[pm.Namespace+"/"+pm.Name] = u
			}
		}
	}

	pvcCapacity := map[string]int64{}
	if pvcs, err := c.Core.CoreV1().PersistentVolumeClaims(namespace).List(ctx, metav1.ListOptions{}); err == nil {
		for _, pvc := range pvcs.Items {
			q, ok := pvc.Status.Capacity[corev1.ResourceStorage]
			if !ok {
				q, ok = pvc.Spec.Resources.Requests[corev1.ResourceStorage]
			}
			if ok {
				pvcCapacity[pvc.Namespace+"/"+pvc.Name] = q.Value()
			}
		}
	}

	nodes := map[string]struct{}{}
	for _, p := range pods.Items {
		if p.Spec.NodeName != "" {
			nodes[p.Spec.NodeName] = struct{}{}
		}
	}
	storage, storageOK := c.kubeletStorageStats(ctx, nodes)
	report.StorageStatsAvailable = storageOK

	report.Pods = make([]PodUsage, 0, len(pods.Items))
	for i := range pods.Items {
		p := &pods.Items[i]
		key := p.Namespace + "/" + p.Name
		pu := PodUsage{
			Name:      p.Name,
			Namespace: p.Namespace,
			Node:      p.Spec.NodeName,
			Phase:     string(p.Status.Phase),
		}
		pu.OwnerKind, pu.OwnerName = podOwner(p)
		for _, cs := range p.Status.ContainerStatuses {
			pu.Restarts += cs.RestartCount
		}
		for _, ctr := range p.Spec.Containers {
			pu.CPURequestMilli += ctr.Resources.Requests.Cpu().MilliValue()
			pu.CPULimitMilli += ctr.Resources.Limits.Cpu().MilliValue()
			pu.MemRequestBytes += ctr.Resources.Requests.Memory().Value()
			pu.MemLimitBytes += ctr.Resources.Limits.Memory().Value()
			pu.EphemeralLimitBytes += ctr.Resources.Limits.StorageEphemeral().Value()
		}
		if u, ok := live[key]; ok {
			pu.CPUUsageMilli, pu.MemUsageBytes = u.cpu, u.mem
		}
		for _, v := range p.Spec.Volumes {
			if v.PersistentVolumeClaim != nil {
				pu.PVCCount++
				pu.PVCCapacityBytes += pvcCapacity[p.Namespace+"/"+v.PersistentVolumeClaim.ClaimName]
			}
		}
		if s, ok := storage[key]; ok {
			pu.EphemeralUsedBytes = s.ephemeral
			for _, v := range s.volumes {
				claimed := pvcCapacity[p.Namespace+"/"+v.claim]
				if v.hasUsed && !sharedFilesystem(v, claimed) {
					pu.PVCUsedBytes += v.used
				} else {
					pu.PVCUsedBytes += claimed
					pu.StorageEstimated = true
				}
			}
			pu.StorageUsedBytes = pu.EphemeralUsedBytes + pu.PVCUsedBytes
		} else if pu.PVCCapacityBytes > 0 {
			pu.StorageUsedBytes = pu.PVCCapacityBytes
			pu.StorageEstimated = true
		}
		report.Pods = append(report.Pods, pu)
	}
	return report, nil
}

// kubeletStorageStats reads /stats/summary from each node through the API
// server proxy. It needs nodes/proxy RBAC; any node that fails is skipped.
// The bool reports whether at least one node answered.
func (c *Client) kubeletStorageStats(ctx context.Context, nodes map[string]struct{}) (map[string]podStorageStats, bool) {
	out := map[string]podStorageStats{}
	if len(nodes) == 0 {
		return out, false
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	var (
		mu       sync.Mutex
		wg       sync.WaitGroup
		answered bool
	)
	for node := range nodes {
		wg.Add(1)
		go func(node string) {
			defer wg.Done()
			raw, err := c.Core.CoreV1().RESTClient().Get().
				AbsPath("/api/v1/nodes", node, "proxy", "stats", "summary").
				DoRaw(ctx)
			if err != nil {
				return
			}
			var sum kubeletSummary
			if json.Unmarshal(raw, &sum) != nil {
				return
			}
			mu.Lock()
			defer mu.Unlock()
			answered = true
			for _, ps := range sum.Pods {
				var s podStorageStats
				if ps.EphemeralStorage != nil && ps.EphemeralStorage.UsedBytes != nil {
					s.ephemeral = *ps.EphemeralStorage.UsedBytes
				}
				for _, v := range ps.Volume {
					if v.PVCRef == nil {
						continue
					}
					vs := pvcVolumeStats{claim: v.PVCRef.Name}
					if v.UsedBytes != nil {
						vs.used, vs.hasUsed = *v.UsedBytes, true
					}
					if v.CapacityBytes != nil {
						vs.capacity, vs.hasCapacity = *v.CapacityBytes, true
					}
					s.volumes = append(s.volumes, vs)
				}
				out[ps.PodRef.Namespace+"/"+ps.PodRef.Name] = s
			}
		}(node)
	}
	wg.Wait()
	return out, answered
}

// sharedFilesystem reports whether the kubelet's numbers for a PVC describe
// the whole host filesystem rather than the volume. Directory-backed
// provisioners such as k3s local-path and hostPath PVs have no filesystem of
// their own, so the kubelet reports the node disk (often terabytes) for every
// such claim. A volume whose filesystem is more than twice its claim is
// treated that way and its usage is not trusted.
func sharedFilesystem(v pvcVolumeStats, claimed int64) bool {
	return claimed > 0 && v.hasCapacity && v.capacity > 2*claimed
}

// podOwner names the workload that controls p, collapsing the ReplicaSet a
// Deployment creates into the Deployment itself.
func podOwner(p *corev1.Pod) (kind, name string) {
	ref := metav1.GetControllerOf(p)
	if ref == nil {
		return "", ""
	}
	if ref.Kind == "ReplicaSet" {
		if hash := p.Labels["pod-template-hash"]; hash != "" && strings.HasSuffix(ref.Name, "-"+hash) {
			return "Deployment", strings.TrimSuffix(ref.Name, "-"+hash)
		}
	}
	return ref.Kind, ref.Name
}
