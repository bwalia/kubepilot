package runbooks

// Builtin returns the set of runbooks shipped with KubePilot.
// These are opinionated workflows for common Kubernetes operations.
func Builtin() []Runbook {
	return []Runbook{
		{
			ID:          "crashloop-diagnostic",
			Name:        "CrashLoopBackOff Diagnostic",
			Description: "Diagnose pods stuck in CrashLoopBackOff and identify root cause via AI analysis.",
			Category:    "diagnostic",
			Risk:        "low",
			Steps: []string{
				"List all crashing pods across the cluster",
				"Run AI root cause analysis on the target pod",
				"Review analysis output and decide on remediation",
				"Optionally restart the deployment to apply fixes",
			},
			Actions: []string{
				"list_crashing_pods",
				"ai_analyze_pod",
				"manual",
				"restart_deployment",
			},
		},
		{
			ID:          "node-pressure-recovery",
			Name:        "Node Pressure Recovery",
			Description: "Identify nodes under memory/disk/PID pressure and review the cluster impact.",
			Category:    "recovery",
			Risk:        "low",
			Steps: []string{
				"List all nodes currently under resource pressure",
				"Review cluster health summary",
				"Investigate workloads on affected nodes (manual)",
				"Cordon + drain or scale down as needed (manual)",
			},
			Actions: []string{
				"list_pressure_nodes",
				"cluster_health_summary",
				"manual",
				"manual",
			},
		},
		{
			ID:          "image-pull-fix",
			Name:        "Image Pull Failure Triage",
			Description: "Find pods stuck on ImagePullBackOff/ErrImagePull and identify registry/credential issues.",
			Category:    "diagnostic",
			Risk:        "low",
			Steps: []string{
				"Scan cluster for image pull errors",
				"Run AI analysis on a representative failing pod",
				"Verify registry credentials and image existence (manual)",
				"Update imagePullSecrets or image tag (manual)",
			},
			Actions: []string{
				"check_image_pull_errors",
				"ai_analyze_pod",
				"manual",
				"manual",
			},
		},
		{
			ID:          "oom-investigation",
			Name:        "OOM Investigation",
			Description: "Investigate pods killed for exceeding memory limits and review resource configuration.",
			Category:    "diagnostic",
			Risk:        "low",
			Steps: []string{
				"List crashing pods (includes OOMKilled)",
				"Run AI root cause analysis on the OOM pod",
				"Review memory requests/limits (manual)",
				"Increase limits or tune application memory (manual)",
			},
			Actions: []string{
				"list_crashing_pods",
				"ai_analyze_pod",
				"manual",
				"manual",
			},
		},
		{
			ID:          "pvc-health-check",
			Name:        "PVC & Storage Health Check",
			Description: "Review persistent volume claim health and identify binding failures.",
			Category:    "health",
			Risk:        "low",
			Steps: []string{
				"List all PVCs and their phase status",
				"Run cluster health summary",
				"Review storage class provisioner logs (manual)",
				"Resolve Pending PVCs (manual)",
			},
			Actions: []string{
				"check_pvc_status",
				"cluster_health_summary",
				"manual",
				"manual",
			},
		},
		{
			ID:          "deployment-rollback",
			Name:        "Deployment Rollback",
			Description: "Roll back a failing deployment by triggering a restart with last-known-good config.",
			Category:    "rollback",
			Risk:        "medium",
			Steps: []string{
				"List crashing pods to confirm deployment is failing",
				"Run AI analysis to confirm restart is safe",
				"Restart the deployment to pick up ConfigMap/Secret changes",
				"Verify rollout completion (manual)",
			},
			Actions: []string{
				"list_crashing_pods",
				"ai_analyze_pod",
				"restart_deployment",
				"manual",
			},
		},
		{
			ID:          "cluster-health-audit",
			Name:        "Cluster Health Audit",
			Description: "End-to-end cluster health review covering nodes, pods, storage, and AI-detected anomalies.",
			Category:    "health",
			Risk:        "low",
			Steps: []string{
				"Generate cluster health summary (nodes + pods)",
				"Check for nodes under resource pressure",
				"List all crashing pods",
				"Check PVC binding status",
				"Review AI-detected anomalies in the RCA tab (manual)",
			},
			Actions: []string{
				"cluster_health_summary",
				"list_pressure_nodes",
				"list_crashing_pods",
				"check_pvc_status",
				"manual",
			},
		},
	}
}
