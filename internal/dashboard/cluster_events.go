package dashboard

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/kubepilot/kubepilot/pkg/k8s"
)

const defaultEventSince = 24 * time.Hour

type eventListResponse struct {
	Items []k8s.Event `json:"items"`
	Total int         `json:"total"`
}

type healthSummary struct {
	NotReadyNodes      int      `json:"not_ready_nodes"`
	CrashLoopPods      int      `json:"crashloop_pods"`
	FailedMountEvents  int      `json:"failed_mount_events"`
	PendingPods        int      `json:"pending_pods"`
	WarningEvents      int      `json:"warning_events"`
	RecommendedActions []string `json:"recommended_actions"`
}

type troubleshootingInsight struct {
	ID                string   `json:"id"`
	Category          string   `json:"category"`
	Severity          string   `json:"severity"`
	Title             string   `json:"title"`
	Summary           string   `json:"summary"`
	Suggestions       []string `json:"suggestions"`
	AffectedResources []string `json:"affected_resources,omitempty"`
}

type nodeHealthRow struct {
	Name               string `json:"name"`
	Ready              bool   `json:"ready"`
	CPUCapacity        string `json:"cpu_capacity"`
	MemoryCapacity     string `json:"memory_capacity"`
	CPUUsage           string `json:"cpu_usage,omitempty"`
	MemoryUsage        string `json:"memory_usage,omitempty"`
	CPUUsagePercent    int    `json:"cpu_usage_percent,omitempty"`
	MemoryUsagePercent int    `json:"memory_usage_percent,omitempty"`
	DiskPressure       bool   `json:"disk_pressure"`
	MemoryPressure     bool   `json:"memory_pressure"`
	PIDPressure        bool   `json:"pid_pressure"`
	Unschedulable      bool   `json:"unschedulable"`
	KubeletVersion     string `json:"kubelet_version"`
}

type resourcePressureSummary struct {
	MetricsAvailable     bool  `json:"metrics_available"`
	CPUUsagePercent      int   `json:"cpu_usage_percent,omitempty"`
	MemoryUsagePercent   int   `json:"memory_usage_percent,omitempty"`
	MemoryPressureNodes  int   `json:"memory_pressure_nodes"`
	DiskPressureNodes    int   `json:"disk_pressure_nodes"`
	PIDPressureNodes     int   `json:"pid_pressure_nodes"`
	CPUUsageMilli        int64 `json:"cpu_usage_milli,omitempty"`
	CPUCapacityMilli     int64 `json:"cpu_capacity_milli,omitempty"`
	MemoryUsageBytes     int64 `json:"memory_usage_bytes,omitempty"`
	MemoryCapacityBytes  int64 `json:"memory_capacity_bytes,omitempty"`

	// Storage: real physical disk usage sourced from Longhorn node CRDs
	// when available, otherwise from node.status.capacity ephemeral-storage
	// (which gives capacity but no usage).
	StorageUsagePercent  int    `json:"storage_usage_percent,omitempty"`
	StorageUsedBytes     int64  `json:"storage_used_bytes,omitempty"`
	StorageCapacityBytes int64  `json:"storage_capacity_bytes,omitempty"`
	StorageSource        string `json:"storage_source,omitempty"` // longhorn | ephemeral-storage

	// StorageClasses breaks down per-StorageClass provisioned capacity.
	StorageClasses []storageClassSummary `json:"storage_classes,omitempty"`
}

type storageClassSummary struct {
	Name               string `json:"name"`
	Provisioner        string `json:"provisioner"`
	ProvisionedBytes   int64  `json:"provisioned_bytes"`
	BoundBytes         int64  `json:"bound_bytes"`
	PVCount            int    `json:"pv_count"`
	PVBoundCount       int    `json:"pv_bound_count"`
}

type problemPod struct {
	Name       string `json:"name"`
	Namespace  string `json:"namespace"`
	Status     string `json:"status"`
	Restarts   int32  `json:"restarts"`
	Node       string `json:"node"`
	Reason     string `json:"reason"`
	AgeMinutes int64  `json:"age_minutes"`
	Message    string `json:"message,omitempty"`
}

type clusterTroubleshootingResponse struct {
	Namespace        string                   `json:"namespace"`
	GeneratedAt      time.Time                `json:"generated_at"`
	HealthSummary    healthSummary            `json:"health_summary"`
	Insights         []troubleshootingInsight `json:"insights"`
	Nodes            []nodeHealthRow          `json:"nodes"`
	ResourcePressure resourcePressureSummary  `json:"resource_pressure"`
	ProblemPods      []problemPod             `json:"problem_pods"`
}

type podDiagnosticsResponse struct {
	Diagnostics *k8s.PodDiagnostics `json:"diagnostics"`
	Logs        string              `json:"logs"`
}

func (s *Server) handleListEvents(w http.ResponseWriter, r *http.Request) {
	k8sClient := s.currentK8sClient()
	ns := normalizeDashboardNamespace(r.URL.Query().Get("namespace"))
	kind := strings.TrimSpace(r.URL.Query().Get("kind"))
	eventType := strings.TrimSpace(r.URL.Query().Get("type"))
	search := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("search")))
	sortDir := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("sort")))
	limit := parseIntWithDefault(r.URL.Query().Get("limit"), 200)
	if limit <= 0 {
		limit = 200
	}
	if limit > 1000 {
		limit = 1000
	}

	since := defaultEventSince
	if rawSince := strings.TrimSpace(r.URL.Query().Get("since")); rawSince != "" {
		if d, err := time.ParseDuration(rawSince); err == nil {
			since = d
		}
	}

	events, err := k8sClient.ListEvents(r.Context(), k8s.EventFilter{
		Namespace:    ns,
		InvolvedKind: normalizeFilterValue(kind),
		EventType:    normalizeFilterValue(eventType),
		Since:        since,
	})
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}

	if search != "" {
		filtered := make([]k8s.Event, 0, len(events))
		for _, event := range events {
			if strings.Contains(strings.ToLower(event.Reason), search) ||
				strings.Contains(strings.ToLower(event.Message), search) ||
				strings.Contains(strings.ToLower(event.InvolvedObject.Name), search) ||
				strings.Contains(strings.ToLower(event.InvolvedObject.Kind), search) ||
				strings.Contains(strings.ToLower(event.InvolvedObject.Namespace), search) {
				filtered = append(filtered, event)
			}
		}
		events = filtered
	}

	sort.Slice(events, func(i, j int) bool {
		if sortDir == "asc" {
			return events[i].LastSeen.Before(events[j].LastSeen)
		}
		return events[i].LastSeen.After(events[j].LastSeen)
	})

	total := len(events)
	if len(events) > limit {
		events = events[:limit]
	}

	writeJSON(w, eventListResponse{Items: events, Total: total})
}

func (s *Server) handleClusterTroubleshooting(w http.ResponseWriter, r *http.Request) {
	resp, err := s.buildClusterTroubleshooting(r.Context(), normalizeDashboardNamespace(r.URL.Query().Get("namespace")))
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, resp)
}

func (s *Server) handlePodDiagnostics(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	namespace := vars["namespace"]
	pod := vars["pod"]
	tailLines := int64(parseIntWithDefault(r.URL.Query().Get("tail_lines"), 200))
	if tailLines <= 0 {
		tailLines = 200
	}

	k8sClient := s.currentK8sClient()
	diagnostics, err := k8sClient.GetPodDiagnostics(r.Context(), namespace, pod)
	if err != nil {
		httpError(w, err, http.StatusInternalServerError)
		return
	}
	logs, err := k8sClient.GetPodLogs(r.Context(), namespace, pod, "", tailLines)
	if err != nil {
		logs = fmt.Sprintf("(logs unavailable: %v)", err)
	}

	writeJSON(w, podDiagnosticsResponse{
		Diagnostics: diagnostics,
		Logs:        logs,
	})
}

func (s *Server) buildClusterTroubleshooting(ctx context.Context, namespace string) (*clusterTroubleshootingResponse, error) {
	k8sClient := s.currentK8sClient()

	events, err := k8sClient.ListEvents(ctx, k8s.EventFilter{Namespace: namespace, Since: defaultEventSince})
	if err != nil {
		return nil, fmt.Errorf("listing events: %w", err)
	}
	nodes, err := k8sClient.ListNodes(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing nodes: %w", err)
	}
	nodeMetrics, err := k8sClient.GetNodeResourceMetrics(ctx)
	if err != nil {
		nodeMetrics = []k8s.NodeResourceMetrics{}
	}
	podList, err := k8sClient.Core.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing pods: %w", err)
	}

	nodeRows, resourcePressure := buildNodeHealthRows(nodes, nodeMetrics)
	enrichStorageSummary(ctx, k8sClient, nodes, &resourcePressure)
	problemPods := buildProblemPods(podList.Items, events)
	insights := buildTroubleshootingInsights(problemPods, events, nodes)
	health := buildHealthSummary(problemPods, events, nodes, insights)

	return &clusterTroubleshootingResponse{
		Namespace:        namespace,
		GeneratedAt:      time.Now().UTC(),
		HealthSummary:    health,
		Insights:         insights,
		Nodes:            nodeRows,
		ResourcePressure: resourcePressure,
		ProblemPods:      problemPods,
	}, nil
}

func normalizeDashboardNamespace(raw string) string {
	ns := strings.TrimSpace(raw)
	if ns == "" || strings.EqualFold(ns, "all") || ns == "*" {
		return ""
	}
	return ns
}

func normalizeFilterValue(raw string) string {
	value := strings.TrimSpace(raw)
	if strings.EqualFold(value, "all") || value == "*" {
		return ""
	}
	return value
}

func parseIntWithDefault(raw string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return fallback
	}
	return value
}

func buildNodeHealthRows(nodes []k8s.NodeSummary, metrics []k8s.NodeResourceMetrics) ([]nodeHealthRow, resourcePressureSummary) {
	metricsByName := make(map[string]k8s.NodeResourceMetrics, len(metrics))
	for _, metric := range metrics {
		metricsByName[metric.Name] = metric
	}

	rows := make([]nodeHealthRow, 0, len(nodes))
	pressure := resourcePressureSummary{}
	var totalCPUMilli int64
	var totalCPUUsageMilli int64
	var totalMemBytes int64
	var totalMemUsageBytes int64

	for _, node := range nodes {
		row := nodeHealthRow{
			Name:           node.Name,
			Ready:          node.Ready,
			CPUCapacity:    node.CPUCapacity,
			MemoryCapacity: node.MemoryCapacity,
			DiskPressure:   node.DiskPressure,
			MemoryPressure: node.MemoryPressure,
			PIDPressure:    node.PIDPressure,
			Unschedulable:  node.Unschedulable,
			KubeletVersion: node.KubeletVersion,
		}
		if node.MemoryPressure {
			pressure.MemoryPressureNodes++
		}
		if node.DiskPressure {
			pressure.DiskPressureNodes++
		}
		if node.PIDPressure {
			pressure.PIDPressureNodes++
		}

		if metric, ok := metricsByName[node.Name]; ok {
			row.CPUUsage = metric.CPUUsage
			row.MemoryUsage = metric.MemUsage
			allocCPU := quantityMilliValue(metric.CPUAllocatable)
			usageCPU := quantityMilliValue(metric.CPUUsage)
			allocMem := quantityByteValue(metric.MemAllocatable)
			usageMem := quantityByteValue(metric.MemUsage)
			if allocCPU > 0 && usageCPU > 0 {
				row.CPUUsagePercent = int((usageCPU * 100) / allocCPU)
				pressure.MetricsAvailable = true
				totalCPUMilli += allocCPU
				totalCPUUsageMilli += usageCPU
			}
			if allocMem > 0 && usageMem > 0 {
				row.MemoryUsagePercent = int((usageMem * 100) / allocMem)
				pressure.MetricsAvailable = true
				totalMemBytes += allocMem
				totalMemUsageBytes += usageMem
			}
		}
		rows = append(rows, row)
	}

	if totalCPUMilli > 0 {
		pressure.CPUUsagePercent = int((totalCPUUsageMilli * 100) / totalCPUMilli)
		pressure.CPUUsageMilli = totalCPUUsageMilli
		pressure.CPUCapacityMilli = totalCPUMilli
	}
	if totalMemBytes > 0 {
		pressure.MemoryUsagePercent = int((totalMemUsageBytes * 100) / totalMemBytes)
		pressure.MemoryUsageBytes = totalMemUsageBytes
		pressure.MemoryCapacityBytes = totalMemBytes
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Ready != rows[j].Ready {
			return !rows[i].Ready
		}
		leftPressure := rows[i].MemoryPressure || rows[i].DiskPressure || rows[i].PIDPressure
		rightPressure := rows[j].MemoryPressure || rows[j].DiskPressure || rows[j].PIDPressure
		if leftPressure != rightPressure {
			return leftPressure
		}
		return rows[i].Name < rows[j].Name
	})

	return rows, pressure
}

// enrichStorageSummary populates cluster-wide storage fields:
//   - Primary metric (gauge): real physical disk usage, sourced from Longhorn
//     Node CRDs when available, else from node.status.capacity ephemeral-storage.
//     Longhorn is preferred because it reports actual used vs total; bare
//     PV capacity metrics would be meaningless (dynamic provisioners report
//     100% bound). Ephemeral-storage fallback gives capacity but no usage.
//   - Per-StorageClass breakdown: how much each class has provisioned,
//     independent of physical disk usage.
// All failures are non-fatal — missing fields cause the UI to render a
// best-effort view.
func enrichStorageSummary(ctx context.Context, k8sClient *k8s.Client, nodes []k8s.NodeSummary, pressure *resourcePressureSummary) {
	// 1. Physical disk stats via Longhorn (preferred).
	if used, total, ok := longhornDiskTotals(ctx, k8sClient); ok {
		pressure.StorageUsedBytes = used
		pressure.StorageCapacityBytes = total
		pressure.StorageSource = "longhorn"
		if total > 0 {
			pressure.StorageUsagePercent = int((used * 100) / total)
		}
	} else {
		// Fall back to node ephemeral-storage capacity — capacity only, no usage.
		var total int64
		for _, n := range nodes {
			total += quantityByteValue(n.EphemeralStorageCapacity)
		}
		if total > 0 {
			pressure.StorageCapacityBytes = total
			pressure.StorageSource = "ephemeral-storage"
		}
	}

	// 2. Per-StorageClass provisioned breakdown.
	pvs, err := k8sClient.Core.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return
	}
	scs, err := k8sClient.Core.StorageV1().StorageClasses().List(ctx, metav1.ListOptions{})
	provisionerByClass := map[string]string{}
	if err == nil {
		for _, sc := range scs.Items {
			provisionerByClass[sc.Name] = sc.Provisioner
		}
	}
	byClass := map[string]*storageClassSummary{}
	for _, pv := range pvs.Items {
		class := pv.Spec.StorageClassName
		if class == "" {
			class = "(none)"
		}
		entry, ok := byClass[class]
		if !ok {
			entry = &storageClassSummary{
				Name:        class,
				Provisioner: provisionerByClass[class],
			}
			byClass[class] = entry
		}
		entry.PVCount++
		if qty, has := pv.Spec.Capacity[corev1.ResourceStorage]; has {
			bytes := quantityByteValue(qty.String())
			entry.ProvisionedBytes += bytes
			if pv.Status.Phase == corev1.VolumeBound {
				entry.BoundBytes += bytes
				entry.PVBoundCount++
			}
		}
	}
	out := make([]storageClassSummary, 0, len(byClass))
	for _, v := range byClass {
		out = append(out, *v)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ProvisionedBytes > out[j].ProvisionedBytes
	})
	pressure.StorageClasses = out
}

// longhornDiskTotals queries Longhorn Node CRDs to sum real disk usage/capacity
// across all Longhorn-managed disks. Returns (used, total, true) on success.
func longhornDiskTotals(ctx context.Context, k8sClient *k8s.Client) (int64, int64, bool) {
	gvr := schema.GroupVersionResource{Group: "longhorn.io", Version: "v1beta2", Resource: "nodes"}
	list, err := k8sClient.Dynamic.Resource(gvr).Namespace("longhorn-system").List(ctx, metav1.ListOptions{})
	if err != nil || list == nil || len(list.Items) == 0 {
		return 0, 0, false
	}
	var totalMax, totalAvail int64
	for _, node := range list.Items {
		disks, found, err := unstructured.NestedMap(node.Object, "status", "diskStatus")
		if err != nil || !found {
			continue
		}
		for _, raw := range disks {
			disk, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			totalMax += asInt64(disk["storageMaximum"])
			totalAvail += asInt64(disk["storageAvailable"])
		}
	}
	if totalMax == 0 {
		return 0, 0, false
	}
	used := totalMax - totalAvail
	if used < 0 {
		used = 0
	}
	return used, totalMax, true
}

func asInt64(v interface{}) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case float64:
		return int64(n)
	case int:
		return int64(n)
	}
	return 0
}

func buildProblemPods(pods []corev1.Pod, events []k8s.Event) []problemPod {
	eventMap := make(map[string][]k8s.Event)
	for _, event := range events {
		if event.InvolvedObject.Kind != "Pod" {
			continue
		}
		key := event.InvolvedObject.Namespace + "/" + event.InvolvedObject.Name
		eventMap[key] = append(eventMap[key], event)
	}

	problematic := make([]problemPod, 0)
	for _, pod := range pods {
		reason, restarts, ready, message := inspectPodStatus(pod)
		ageMinutes := int64(time.Since(pod.CreationTimestamp.Time).Minutes())
		if pod.Status.Reason == "Evicted" && reason == "" {
			reason = "Evicted"
		}
		if reason == "" && pod.Status.Phase == corev1.PodPending && ageMinutes > 5 {
			reason = "Pending"
		}
		if reason == "" && !ready && restarts > 5 {
			reason = "Restarting"
		}

		key := pod.Namespace + "/" + pod.Name
		if reason == "Pending" {
			for _, event := range eventMap[key] {
				if event.Reason == "FailedScheduling" {
					reason = "FailedScheduling"
					if message == "" {
						message = event.Message
					}
					break
				}
			}
		}

		if reason == "" {
			continue
		}

		problematic = append(problematic, problemPod{
			Name:       pod.Name,
			Namespace:  pod.Namespace,
			Status:     string(pod.Status.Phase),
			Restarts:   restarts,
			Node:       pod.Spec.NodeName,
			Reason:     reason,
			AgeMinutes: ageMinutes,
			Message:    message,
		})
	}

	sort.Slice(problematic, func(i, j int) bool {
		if problematic[i].Reason != problematic[j].Reason {
			return problematic[i].Reason < problematic[j].Reason
		}
		if problematic[i].Restarts != problematic[j].Restarts {
			return problematic[i].Restarts > problematic[j].Restarts
		}
		return problematic[i].Name < problematic[j].Name
	})

	return problematic
}

func inspectPodStatus(pod corev1.Pod) (reason string, restarts int32, ready bool, message string) {
	statuses := append([]corev1.ContainerStatus{}, pod.Status.InitContainerStatuses...)
	statuses = append(statuses, pod.Status.ContainerStatuses...)
	for _, status := range statuses {
		restarts += status.RestartCount
		if status.Ready {
			ready = true
		}
		if status.State.Waiting != nil {
			if reason == "" {
				reason = status.State.Waiting.Reason
				message = status.State.Waiting.Message
			}
		}
		if status.State.Terminated != nil && reason == "" {
			reason = status.State.Terminated.Reason
			message = status.State.Terminated.Message
		}
		if status.LastTerminationState.Terminated != nil && reason == "" {
			reason = status.LastTerminationState.Terminated.Reason
			message = status.LastTerminationState.Terminated.Message
		}
	}
	return reason, restarts, ready, message
}

func buildHealthSummary(problemPods []problemPod, events []k8s.Event, nodes []k8s.NodeSummary, insights []troubleshootingInsight) healthSummary {
	summary := healthSummary{}
	for _, node := range nodes {
		if !node.Ready {
			summary.NotReadyNodes++
		}
	}
	for _, pod := range problemPods {
		switch pod.Reason {
		case "CrashLoopBackOff", "OOMKilled", "ImagePullBackOff", "ErrImagePull", "ContainerCannotRun", "BackOff":
			summary.CrashLoopPods++
		}
		if pod.Status == string(corev1.PodPending) || pod.Reason == "FailedScheduling" || pod.Reason == "Pending" {
			summary.PendingPods++
		}
	}
	for _, event := range events {
		if event.Type == "Warning" {
			summary.WarningEvents++
		}
		if event.Reason == "FailedMount" || event.Reason == "VolumeAttachTimeout" {
			summary.FailedMountEvents++
		}
	}
	for _, insight := range insights {
		for _, suggestion := range insight.Suggestions {
			if !containsString(summary.RecommendedActions, suggestion) {
				summary.RecommendedActions = append(summary.RecommendedActions, suggestion)
			}
			if len(summary.RecommendedActions) >= 4 {
				return summary
			}
		}
	}
	return summary
}

func buildTroubleshootingInsights(problemPods []problemPod, events []k8s.Event, nodes []k8s.NodeSummary) []troubleshootingInsight {
	insights := make([]troubleshootingInsight, 0)

	podFailures := filterProblemPods(problemPods, func(p problemPod) bool {
		switch p.Reason {
		case "CrashLoopBackOff", "ImagePullBackOff", "ErrImagePull", "OOMKilled", "ContainerCannotRun", "BackOff":
			return true
		default:
			return p.Restarts > 5
		}
	})
	if len(podFailures) > 0 {
		insights = append(insights, troubleshootingInsight{
			ID:       "pod-failures",
			Category: "pod",
			Severity: "high",
			Title:    fmt.Sprintf("%d pods are crashing or restarting", len(podFailures)),
			Summary:  "Detected repeated pod startup failures such as CrashLoopBackOff, image pull errors, OOMKilled, or unusually high restart counts.",
			Suggestions: []string{
				"Investigate failing pods",
				"Check container logs and environment variables",
				"Review readiness/liveness probes",
			},
			AffectedResources: problemPodRefs(podFailures),
		})
	}

	schedulingEvents := filterEvents(events, func(event k8s.Event) bool {
		message := strings.ToLower(event.Message)
		return event.Reason == "FailedScheduling" || strings.Contains(message, "insufficient cpu") || strings.Contains(message, "insufficient memory")
	})
	if len(schedulingEvents) > 0 {
		insights = append(insights, troubleshootingInsight{
			ID:       "scheduling-issues",
			Category: "scheduling",
			Severity: "high",
			Title:    fmt.Sprintf("%d scheduling failures detected", len(schedulingEvents)),
			Summary:  "Pods are failing to schedule because the cluster lacks available resources or placement constraints cannot be satisfied.",
			Suggestions: []string{
				"Check node CPU allocation",
				"Review pending pods and scheduler constraints",
				"Verify cluster autoscaler status",
			},
			AffectedResources: eventRefs(schedulingEvents),
		})
	}

	storageEvents := filterEvents(events, func(event k8s.Event) bool {
		message := strings.ToLower(event.Message)
		return event.Reason == "FailedMount" || event.Reason == "VolumeAttachTimeout" || strings.Contains(message, "persistentvolumeclaim") || strings.Contains(message, "pvc")
	})
	if len(storageEvents) > 0 {
		insights = append(insights, troubleshootingInsight{
			ID:       "storage-issues",
			Category: "storage",
			Severity: "medium",
			Title:    fmt.Sprintf("%d storage mount issues detected", len(storageEvents)),
			Summary:  "Persistent volumes or mounts are failing, which often points to StorageClass, CSI driver, or volume attachment problems.",
			Suggestions: []string{
				"Verify storage classes",
				"Check CSI driver health",
				"Inspect PVC and PV binding state",
			},
			AffectedResources: eventRefs(storageEvents),
		})
	}

	networkEvents := filterEvents(events, func(event k8s.Event) bool {
		message := strings.ToLower(event.Message)
		return strings.EqualFold(event.Reason, "FailedCreatePodSandBox") || strings.Contains(message, "cni") || strings.Contains(message, "networkpluginnotready")
	})
	if len(networkEvents) > 0 {
		insights = append(insights, troubleshootingInsight{
			ID:       "network-issues",
			Category: "networking",
			Severity: "medium",
			Title:    fmt.Sprintf("%d pod networking issues detected", len(networkEvents)),
			Summary:  "Pod sandbox creation or CNI initialization is failing, which typically indicates cluster networking or CNI plugin issues.",
			Suggestions: []string{
				"Inspect CNI plugin pods and logs",
				"Check node network readiness",
				"Review kubelet and container runtime events",
			},
			AffectedResources: eventRefs(networkEvents),
		})
	}

	unhealthyNodes := make([]string, 0)
	for _, node := range nodes {
		if !node.Ready || node.MemoryPressure || node.DiskPressure || node.PIDPressure {
			unhealthyNodes = append(unhealthyNodes, node.Name)
		}
	}
	if len(unhealthyNodes) > 0 {
		insights = append(insights, troubleshootingInsight{
			ID:       "node-health",
			Category: "nodes",
			Severity: "high",
			Title:    fmt.Sprintf("%d nodes are degraded or under pressure", len(unhealthyNodes)),
			Summary:  "One or more nodes are NotReady or experiencing Memory, Disk, or PID pressure, which can cascade into scheduling and runtime failures.",
			Suggestions: []string{
				"Check node resource pressure",
				"Inspect kubelet and node conditions",
				"Drain or replace unhealthy nodes if needed",
			},
			AffectedResources: unhealthyNodes,
		})
	}

	return insights
}

func filterProblemPods(pods []problemPod, predicate func(problemPod) bool) []problemPod {
	filtered := make([]problemPod, 0)
	for _, pod := range pods {
		if predicate(pod) {
			filtered = append(filtered, pod)
		}
	}
	return filtered
}

func filterEvents(events []k8s.Event, predicate func(k8s.Event) bool) []k8s.Event {
	filtered := make([]k8s.Event, 0)
	for _, event := range events {
		if predicate(event) {
			filtered = append(filtered, event)
		}
	}
	return filtered
}

func problemPodRefs(pods []problemPod) []string {
	refs := make([]string, 0, len(pods))
	for _, pod := range pods {
		refs = append(refs, pod.Namespace+"/"+pod.Name)
	}
	return refs
}

func eventRefs(events []k8s.Event) []string {
	refs := make([]string, 0, len(events))
	for _, event := range events {
		ref := event.InvolvedObject.Namespace + "/" + event.InvolvedObject.Name
		if event.InvolvedObject.Namespace == "" {
			ref = event.InvolvedObject.Name
		}
		if !containsString(refs, ref) {
			refs = append(refs, ref)
		}
	}
	return refs
}

func containsString(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func quantityMilliValue(raw string) int64 {
	if strings.TrimSpace(raw) == "" {
		return 0
	}
	q, err := resource.ParseQuantity(raw)
	if err != nil {
		return 0
	}
	return q.MilliValue()
}

func quantityByteValue(raw string) int64 {
	if strings.TrimSpace(raw) == "" {
		return 0
	}
	q, err := resource.ParseQuantity(raw)
	if err != nil {
		return 0
	}
	return q.Value()
}
