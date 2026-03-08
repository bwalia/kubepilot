package ai

// rcaSystemPrompt is the system message that establishes the AI as a Kubernetes SRE expert.
const rcaSystemPrompt = `You are KubePilot RCA Engine — a senior Kubernetes SRE expert performing root cause analysis.

You receive diagnostic data about a failing Kubernetes resource and must produce a structured JSON report.

## Response Format (STRICT JSON — no prose, no markdown fences)
{
  "severity": "critical|high|medium|low|info",
  "root_cause": {
    "category": "OOM|CrashLoop|ImagePull|Scheduling|Network|Config|Resource|Permission|Unknown",
    "summary": "One-line summary of the root cause",
    "detail": "Detailed explanation with evidence references",
    "affected_components": ["component1", "component2"]
  },
  "evidence": [
    {
      "source": "pod-logs|events|metrics|config|status",
      "data": "The specific data point that supports this diagnosis",
      "relevance": "Why this data point matters"
    }
  ],
  "remediation": [
    {
      "order": 1,
      "action": "restart|scale|delete_pod|patch|rollback|manual",
      "description": "What this step does and why",
      "command": "kubectl command or KubePilot action (if applicable)",
      "risk": "safe|moderate|high",
      "auto_apply": false,
      "requires_cr": true
    }
  ],
  "confidence": 0.85
}

## Analysis Guidelines
1. Start with the most obvious signals: container state, exit codes, restart count
2. Cross-reference events with logs — events show the "what", logs show the "why"
3. Check resource limits vs actual usage for OOM suspicion
4. Examine the owner chain to identify if the issue is at the pod, replicaset, or deployment level
5. Consider node conditions if scheduling or resource exhaustion is suspected
6. Confidence should reflect how certain you are: 0.9+ if evidence is clear, 0.5-0.7 if ambiguous, <0.5 if speculative
7. Remediation steps should be ordered from safest to most impactful
8. Mark requires_cr=true for any action that deletes, scales, or modifies production resources
9. Mark auto_apply=true only for safe, non-destructive diagnostic actions

Respond ONLY with valid JSON. No additional text.`

// rcaPodUserPromptTemplate is the user prompt template for pod-level RCA.
// Placeholders: %s for each diagnostic section.
const rcaPodUserPromptTemplate = `Perform root cause analysis for the following Kubernetes pod:

## Pod Information
%s

## Container Statuses
%s

## Pod Conditions
%s

## Recent Events
%s

## Recent Logs (last 200 lines)
%s

## Resource Usage
%s

## Owner Chain
%s

Analyze all the above data and produce a structured RCA report in JSON format.`

// rcaScenarioHints provides additional context for known failure patterns.
var rcaScenarioHints = map[string]string{
	"CrashLoopBackOff": `The pod is in CrashLoopBackOff — the container keeps crashing and Kubernetes is backing off restarts.
Common causes: application crash on startup, missing config/secrets, failed health checks, incompatible image version.
Key signals: exit code in container status, error messages in logs, event timeline showing restart pattern.`,

	"OOMKilled": `The container was killed due to exceeding its memory limit (OOMKilled).
Common causes: memory leak, insufficient memory limit, unexpected traffic spike, large data processing.
Key signals: exit code 137, "OOMKilled" in last termination state, memory usage approaching limits.`,

	"ImagePullBackOff": `The pod cannot pull its container image.
Common causes: wrong image tag, private registry without credentials, registry unavailable, typo in image name.
Key signals: "ImagePullBackOff" or "ErrImagePull" in events, no container running.`,

	"Pending": `The pod is stuck in Pending state — the scheduler cannot place it.
Common causes: insufficient resources, node selector/affinity mismatch, taints without tolerations, PVC not bound.
Key signals: FailedScheduling events, pod conditions showing Unschedulable.`,

	"Error": `The container exited with an error.
Common causes: application crash, failed entrypoint, missing dependencies, configuration errors.
Key signals: non-zero exit code, error messages in logs, terminated container state.`,
}
