# KubePilot Autopilot — AI-Driven Self-Healing

Autopilot closes KubePilot's loop. Until now the platform could **detect** pod
problems and **explain** them with AI root-cause analysis (RCA), but a human had
to click "remediate". Autopilot adds the missing final step: it consumes each
finished RCA report and — under a strict, auditable safety policy — **resolves
safe issues automatically**.

```
                 ┌──────────────┐   anomaly    ┌─────────────┐   RCA report   ┌────────────────┐
   cluster  ───▶ │ ClusterWatch │ ───────────▶ │  RCA Engine │ ─────────────▶ │   Autopilot    │
   snapshot      │ (anomaly     │              │ (Ollama LLM)│   (ReportHook) │  Controller    │
   every 30s     │  rules)      │              └─────────────┘                └───────┬────────┘
                 └──────────────┘                                                     │ policy gate
                                                                                      ▼
                                              ┌───────────────────────────────────────────────┐
                                              │ decide → (execute | dry-run | skip | escalate) │
                                              │   via ai.RemediationExecutor → k8s write ops    │
                                              └───────────────────────────────────────────────┘
```

It deliberately **reuses** existing building blocks rather than adding a parallel
path: `observability.ClusterWatcher` for detection, `ai.RCAEngine` for diagnosis,
and `ai.RemediationExecutor` for the actual cluster mutations (restart / delete
pod / scale). The only new code is the *decision layer* in `pkg/autopilot`.

## Safety model (defence in depth)

Autopilot is **off by default**. Nothing changes until an operator opts in.

| Guardrail | Default | What it prevents |
|-----------|---------|------------------|
| **Mode** | `off` | `off` does nothing; `dry-run` decides + logs but never mutates; `active` mutates |
| **Confidence floor** | `0.80` | Acting on a low-confidence / guessed diagnosis |
| **Action allow-list** | `delete_pod, restart` | Running an action type you didn't sanction |
| **Risk ceiling** | `safe` | Auto-applying `moderate`/`high`-risk steps |
| **Namespace guardrails** | block `kube-system`, `kube-public`, `kube-node-lease`, `kubepilot-system` | Touching control-plane / system workloads |
| **Per-resource cooldown** | `10m` | Fighting a reconciling controller in a tight loop |
| **Global rate limit** | `10 / hour` | A large blast radius during a cluster-wide incident |
| **CR-code boundary** | always on | Bypassing change control — CR-gated steps are **escalated**, never auto-run |

Every evaluation — `executed`, `dry-run`, `skipped`, `escalated`, or `failed` —
is recorded as a `Decision` in an in-memory ledger for full auditability.

## How a step is chosen

For each RCA report, autopilot scans the LLM's remediation steps (lowest `order`
first) and picks the first one that is **all** of: not CR-gated and not manual,
`auto_apply: true`, in the action allow-list, within the risk ceiling, and with a
resolvable target. For `delete_pod` the target is the failing pod from the RCA;
for `restart`/`scale` an explicit `namespace/name` target must be supplied,
otherwise the step is **escalated** rather than guessed.

If the only available fixes need a human (CR code, manual, or an unresolvable
target), the report is **escalated** — surfaced for an operator instead of
silently dropped.

## Configuration

CLI flags (all have `KUBEPILOT_`-prefixed env + `config.yaml` equivalents):

```bash
kubepilot serve \
  --autopilot-mode=dry-run \
  --autopilot-min-confidence=0.85 \
  --autopilot-allowed-actions=delete_pod,restart \
  --autopilot-max-risk=safe \
  --autopilot-blocked-namespaces=kube-system,kube-public,kube-node-lease \
  --autopilot-cooldown=10m \
  --autopilot-max-actions-per-hour=10
```

See `config.example.yaml` for the YAML form.

### Recommended rollout

1. **`dry-run`** for a few days. Watch `GET /api/v1/autopilot` and logs to see
   exactly what autopilot *would* have done.
2. Tighten `allowed_namespaces` to a low-risk namespace (e.g. `staging`).
3. Promote to **`active`** for that namespace; widen gradually.

## Dashboard

An **Autopilot** page (top nav) shows the live cockpit: current mode badge,
aggregate stats, the active policy, and the decision ledger — plus a global
**pause / resume kill switch**.

- **Pause** (`POST /api/v1/autopilot/pause`) is the kill switch: it immediately
  switches to `off`, halting all self-healing. It is always available,
  regardless of mutation-endpoint settings, because stopping automation is
  always safe. It remembers the previous mode.
- **Resume** (`POST /api/v1/autopilot/resume`) restores the pre-pause mode (or
  `dry-run` if autopilot was never running). The UI requires a confirm click.

## Observability

`GET /api/v1/autopilot?limit=50` returns:

```json
{
  "enabled": true,
  "policy": { "mode": "active", "min_confidence": 0.8, "...": "..." },
  "stats": { "executed": 4, "dry-run": 0, "skipped": 11, "escalated": 2, "actions_last_hour": 4 },
  "decisions": [
    {
      "time": "2026-06-19T09:30:01Z",
      "report_id": "rca-web-7c9f",
      "resource": { "kind": "Pod", "namespace": "shop", "name": "web-7c9f" },
      "severity": "high", "confidence": 0.93, "root_cause": "CrashLoop",
      "action": "delete_pod", "verdict": "executed",
      "reason": "auto-remediated shop/web-7c9f via delete_pod",
      "output": "Pod shop/web-7c9f deleted (will be recreated by controller)"
    }
  ]
}
```

## Roadmap — next high-impact ideas

The controller is intentionally a clean seam to build on. Prioritised follow-ups:

1. **Post-action verification loop.** After acting, re-poll the resource for N
   minutes; if it recovers, mark *resolved*; if not, *escalate* and avoid
   re-acting. Turns "fire and forget" into a real control loop.
2. **Outcome-aware confidence.** Feed verified outcomes back into the knowledge
   base (`pkg/ai/knowledge.go`) so repeated, proven fixes earn higher trust and
   novel ones stay conservative.
3. **Rollback / undo.** Capture pre-action state (replica count, image, revision)
   so an action can be reverted; implement real `rollback` via Deployment
   revision history (today it escalates).
4. **Workload-target resolution.** Resolve a pod's owning Deployment/StatefulSet
   from its owner chain so `restart`/`scale` work without an explicit target.
5. **Slack/PagerDuty approval flow.** For `escalated` or `moderate`-risk steps,
   post an interactive "Approve / Deny" message and execute on approval.
6. **Policy as CRD.** Express the policy as a `KubePilotAutopilotPolicy` CRD with
   per-namespace overrides, wired through the existing operator controllers.
7. **Cascading correlation.** Use `pkg/ai/correlation.go` so fixing a root
   service suppresses redundant actions on its dependents.
8. **MCP fan-out.** Execute remediation in spoke clusters via the MCP agents
   (`pkg/mcp/client` action executor is currently a TODO).
9. **Cost/SLO awareness.** Gate `scale` actions on namespace quotas and error-
   budget burn so self-healing respects budgets.

> ✅ **Done:** live dashboard panel (decision ledger + policy + stats) and the
> global pause/resume kill switch (`/api/v1/autopilot/pause` · `/resume`).
```
