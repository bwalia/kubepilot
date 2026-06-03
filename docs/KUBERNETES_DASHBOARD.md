# Kubernetes Dashboard

KubePilot ships two complementary experiences served from the same single binary:

| Route        | Title                        | Purpose                                              |
|--------------|------------------------------|------------------------------------------------------|
| `/`          | KubePilot AI Troubleshooting | AI command bar, RCA, anomalies, crashing-pod triage  |
| `/dashboard` | Kubernetes Dashboard         | Lens/Rancher-style read-only cluster resource browser |

A slim global navigation bar (`components/GlobalNav.tsx`) links the two pages and
highlights the active one. The AI Troubleshooting page is unchanged and remains the
default landing page; the Kubernetes Dashboard is purely additive.

The differentiator versus Lens / OpenLens / Rancher: from any pod in the dashboard you
can open the **AI Analysis** tab and run KubePilot's troubleshooting engine for an
instant root-cause analysis — moving directly from cluster visibility into AI-assisted
diagnosis.

## Architecture

```
┌──────────────────────────────┐     /api/v1/*      ┌────────────────────────────┐
│ Next.js static export (SPA)  │ ─────────────────▶ │ Go dashboard server        │
│  pages/dashboard/index.tsx   │                    │  internal/dashboard/       │
│  components/dashboard/*       │ ◀───────────────── │   k8s_browser.go (handlers)│
└──────────────────────────────┘     JSON / text    │   server.go (routing)      │
                                                     └────────────┬───────────────┘
                                                                  │ client-go
                                                     ┌────────────▼───────────────┐
                                                     │ pkg/k8s/*  (typed wrappers) │
                                                     └────────────┬───────────────┘
                                                                  │
                                                     ┌────────────▼───────────────┐
                                                     │ Kubernetes API server       │
                                                     └─────────────────────────────┘
```

### Static-export constraint

`dashboard/next.config.js` uses `output: "export"`, so there is no Node.js runtime in
production — the Go binary serves pre-built static files from `dashboard/out`. This means
**no server-side dynamic routes**. The dashboard therefore uses a single
`pages/dashboard/index.tsx` page that keeps all navigation in React state
(selected namespace, active section, selected pod). The pod detail view is a client-side
slide-over **drawer**, not a `[param]` file route.

### Frontend layout

- `pages/dashboard/index.tsx` — page shell: namespace selector + section tabs (Workloads,
  Network, Config & Storage, Cluster Health, Events), plus the pod-detail and YAML drawers.
- `components/dashboard/WorkloadsSection.tsx` — Pods / Deployments / StatefulSets /
  DaemonSets / Jobs / CronJobs. Pods reuse `components/PodTable.tsx`.
- `components/dashboard/NetworkSection.tsx` — Services, Ingresses.
- `components/dashboard/ConfigSection.tsx` — ConfigMaps, Secrets, PVCs, StorageClasses.
- `components/dashboard/ClusterHealthSection.tsx` — reuses `ClusterResourceCharts` plus the
  `/troubleshooting/summary` insights, problem pods, and node health.
- `components/dashboard/EventsSection.tsx` — reuses `ClusterEventsTroubleshooting`.
- `components/dashboard/PodDetailDrawer.tsx` — tabs: Overview, Containers, Logs, Events,
  YAML, AI Analysis.
- `components/ui/*` — shadcn/ui primitives (Dialog/Drawer, Tabs, Tooltip, Badge, Button,
  Separator) themed to the existing `pilot-*` palette.

### UI stack note

shadcn/ui components are plain React source files owned in this repo (`components/ui/`),
wrapping Radix primitives and styled with Tailwind + `class-variance-authority`. They
coexist with the existing hand-rolled `pilot-*` components; shadcn's own CSS variables are
not introduced, so there is no token collision — all dashboard components use `pilot-*`
classes directly.

## API flow

All endpoints are under `/api/v1` and are **read-only GETs** (no new mutation endpoints).

| Endpoint                                          | Backend method                  |
|---------------------------------------------------|----------------------------------|
| `GET /namespaces`                                 | `ListNamespaces`                 |
| `GET /clusters/statefulsets?namespace=`           | `ListStatefulSets`               |
| `GET /clusters/daemonsets?namespace=`             | `ListDaemonSets`                 |
| `GET /clusters/jobs?namespace=`                   | `ListJobs`                       |
| `GET /clusters/cronjobs?namespace=`               | `ListCronJobs`                   |
| `GET /clusters/ingresses?namespace=`              | `ListIngresses`                  |
| `GET /clusters/services?namespace=`               | `ListServices`                   |
| `GET /clusters/configmaps?namespace=`             | `ListConfigMaps`                 |
| `GET /clusters/secrets?namespace=`                | `ListSecrets` (metadata only)    |
| `GET /clusters/pvcs?namespace=`                   | `ListPVCs`                       |
| `GET /clusters/storageclasses`                    | `ListStorageClasses`             |
| `GET /clusters/pods/{ns}/{pod}/logs?container=&tail=` | `GetPodLogs`                 |
| `GET /resource/{kind}/{namespace}/{name}/yaml`    | `GetResourceYAML` (sanitized)    |
| `GET /config`                                     | server capability flags          |

Reused existing endpoints: `/clusters/pods`, `/clusters/deployments`,
`/clusters/pods/{ns}/{pod}/diagnostics`, `/events`, `/troubleshooting/summary`,
`/ai/troubleshoot/{ns}/{pod}`.

## Security model

### Secret redaction (two layers)

1. **List layer** — `pkg/k8s/config_secrets.go` `ListSecrets` returns a `SecretSummary`
   with only name, namespace, type, and key count. It never reads `.Data`/`.StringData`,
   and the struct has no field that could carry a value.
2. **YAML layer** — `pkg/k8s/yaml_view.go` `GetResourceYAML` strips the `data` and
   `stringData` blocks for `kind == Secret`, and removes `metadata.managedFields` for every
   resource. Covered by `TestGetResourceYAML_RedactsSecretData` and
   `TestListSecrets_NeverExposesData`.

ServiceAccount tokens and private keys live inside Secrets and are therefore never exposed.

### Mutation gating

The dashboard is read-only by default. The `GET /api/v1/config` endpoint reports
`mutations_enabled` from the server's `EnableActionMutationEndpoints` flag. The AI Analysis
tab's action buttons are disabled (with a tooltip) unless mutations are enabled; when
enabled, actions that require a change-record code route through the existing
`CRCodeApproval` modal and the gated `/ai/execute-action` endpoint.

### RBAC

The service account KubePilot runs as needs read access to the browsed resources:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: kubepilot-dashboard-reader
rules:
  - apiGroups: [""]
    resources: [namespaces, pods, pods/log, services, configmaps, secrets,
                persistentvolumeclaims, nodes, events]
    verbs: [get, list, watch]
  - apiGroups: ["apps"]
    resources: [deployments, statefulsets, daemonsets, replicasets]
    verbs: [get, list, watch]
  - apiGroups: ["batch"]
    resources: [jobs, cronjobs]
    verbs: [get, list, watch]
  - apiGroups: ["networking.k8s.io"]
    resources: [ingresses]
    verbs: [get, list, watch]
  - apiGroups: ["storage.k8s.io"]
    resources: [storageclasses]
    verbs: [get, list, watch]
```

`secrets` is included so secret *metadata* and redacted YAML can be listed. Restrict or
remove it if even metadata visibility is undesirable; the dashboard degrades gracefully
(the Secrets tab simply shows an error).

The dashboard never bypasses Kubernetes RBAC — every call uses the same client credentials
as the rest of KubePilot.

## Port forwarding

The dashboard can start `kubectl`-style port-forwards against the target cluster for Pods and
Services (auto-resolving service → backing pod and named target ports). Endpoints live under
`/api/v1/portforward` and are gated by `EnableActionMutationEndpoints` (list is read-only/always
on; start/stop are gated).

Because KubePilot often runs in a container where the auto-assigned local port is **not** published
to the host, each session also exposes an HTTP reverse-proxy path through the already-published
dashboard port:

```
GET /api/v1/forward/{session_id}/...  →  127.0.0.1:{local_port}  →  pod
```

The UI surfaces this as the primary **Access URL** (e.g.
`http://localhost:8383/api/v1/forward/<id>/`). It is **HTTP-only** — raw TCP protocols (databases,
DNS) can't be tunnelled through a browser; for those, use the direct `localhost:<local_port>`
address from a host that can reach the server (i.e. run KubePilot natively or publish the port).

## Performance

Resource lists and logs use TanStack Query polling (15s interval, inherited from
`_app.tsx`). The pod-detail Logs tab polls the same way and offers manual refresh and a
configurable tail size (200/500/1000). WebSocket / watch-based streaming is a planned
follow-up and is intentionally out of scope for this version.

## Screenshots

- `[Screenshot: Workloads section — pods table with a crashing pod]`
- `[Screenshot: Pod detail drawer — AI Analysis tab with root cause]`
- `[Screenshot: Config & Storage — Secrets tab showing the redaction banner]`
- `[Screenshot: Cluster Health — resource gauges, insights, node health]`

## Build & deploy

```bash
# Build the static dashboard (emits dashboard/out)
make dashboard

# Build the Go binary (embeds/serves dashboard/out)
make build

# Run locally — dashboard on :8383, MCP on :9090
./dist/kubepilot serve

# Open the two experiences
open http://localhost:8383/            # AI Troubleshooting
open http://localhost:8383/dashboard/  # Kubernetes Dashboard
```

### Development workflow

```bash
# Terminal 1 — Go API server on :8383
make run-server

# Terminal 2 — Next.js dev server on :3000 (proxies /api/* to :8383)
make dashboard-dev
```

To enable write actions (scale/restart/execute) in the dashboard, start the server with
action mutations enabled (`enable_action_mutations: true` in config or
`KUBEPILOT_ENABLE_ACTION_MUTATIONS=true`). Actions still require change-record code
authorization where flagged.
