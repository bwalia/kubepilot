# OTLP Autopilot

OTLP Autopilot is KubePilot's **automatic observability engine**. It is not an OpenTelemetry configuration UI.

> Deploy anything. Get observability automatically.

## Flow

```text
Install KubePilot
       │
       ▼
Enable OTLP Autopilot (observe | enable)
       │
       ▼
Discover applications on the K3s cluster
       │
       ▼
Automatically enable telemetry (best effort)
       ├── Logs
       ├── Metrics
       └── Traces
       │
       ▼
KubePilot UI — Metrics, Logs & Traces
  (cluster plane + application plane)
```

## Fallback ladder

`eBPF → auto-instrumentation → collector-based telemetry → minimal configuration → manual SDK (last resort)`

## Hybrid storage

| Path | Default | Purpose |
|------|---------|---------|
| KubePilot-managed short-retention store | On | Powers built-in Metrics / Logs / Traces UI with zero external deps |
| Prometheus / Thanos / Cortex `remote_write` | Off | Optional metrics dual-write |
| External OTLP endpoint | Off | Optional logs/traces (and metrics) dual-write |
| Grafana | Optional | Advanced dashboards on exported backends |

## Modes

| Mode | Behavior |
|------|----------|
| `off` | Disabled |
| `observe` | Discover workloads and score coverage without mutating |
| `enable` | Apply zero/low-code enablement for the supported runtime subset |

## API

- `GET /api/v1/otel/autopilot` — status, coverage, signal counts, export health
- `POST /api/v1/otel/autopilot/mode` — `{ "mode": "off\|observe\|enable" }`
- `GET /api/v1/otel/apps` — discovered apps + capability
- `GET /api/v1/otel/metrics|logs|traces` — overview queries (`plane`, `service`, `limit`)
- `GET /api/v1/otel/services/{id}` — service drill-down
- `POST /api/v1/otel/ingest` — push samples into the managed store (simple JSON)
- `POST /api/v1/otel/v1/{traces,metrics,logs}` — standard **OTLP/HTTP** receiver (protobuf or JSON, gzip). Point a collector's `otlphttp` exporter at `<kubepilot>/api/v1/otel`
- `GET /api/v1/otel/servicegraph?window=15m&namespace=` — trace-derived topology: nodes (services + inferred databases/queues/external hosts), edges, RED stats vs the previous window, live pod state
- `GET /api/v1/otel/trace-summaries?window=&service=&edge=&errors=true&min_ms=` — trace explorer list
- `GET /api/v1/otel/traces/{traceId}` — one trace with spans and trace-correlated logs
- `GET /api/v1/otel/insights` — AI incident insights (open first)
- `POST /api/v1/otel/insights/{id}/analyze` — re-run the AI analysis

## Proactive AI insights

Every 30s KubePilot compares the last 5 minutes of spans with the previous 30 and flags error-rate spikes, p95 latency regressions and new error signatures (messages with digits normalised, so `timed out after 1692ms` and `…1733ms` are one signature). Each signal is walked downstream along unhealthy edges to its **origin** — a failing database is one incident however many callers report it — and the upstream blast radius is computed. The incident is then explained by the configured model (`--ollama-model`) with sample failing traces, their logs and live pod state as evidence; failed analyses retry up to 3 times. New incidents are added to the anomaly timeline and sent to Slack when configured. Insights run whenever spans arrive, independent of the Autopilot mode.

## Connecting a collector

KubePilot's dashboard auth also covers ingest. Give the collector an ingest-only token instead of dashboard credentials:

```bash
kubepilot serve --otel-ingest-token "$TOKEN"     # or KUBEPILOT_OTEL_INGEST_TOKEN
```

```yaml
exporters:
  otlphttp/kubepilot:
    endpoint: http://kubepilot.kubepilot-system.svc:8080/api/v1/otel
    headers:
      Authorization: "Bearer ${env:KUBEPILOT_OTEL_INGEST_TOKEN}"
```

Add it next to existing exporters (X-Ray, Loki, Prometheus…) — nothing else changes. With Helm, set `kubepilot.auth.ingestToken` (or an `ingestToken` key in `existingSecret`) and the bundled collector picks it up.

## Install

CLI flags (see `config.example.yaml`):

```bash
./dist/kubepilot serve \
  --otel-autopilot-mode=observe \
  --otel-autopilot-managed-store \
  --otel-export-metrics-remote-write-url=http://thanos-receive:19291/api/v1/receive
```

Helm:

```yaml
otelAutopilot:
  enabled: true
  mode: observe
  managedStore:
    enabled: true
  collector:
    enabled: true
  export:
    metrics:
      remoteWrite:
        url: "http://thanos-receive.monitoring:19291/api/v1/receive"
```

Raw manifests: `manifests/otel-autopilot/`.

## UI

Open **OTLP** in the top nav (`/otel`):

- **Incidents** — AI-detected problems with failure path, before/after signals, top errors, AI root cause, next steps and copyable commands.
- **Service map** — layered topology (callers → services → databases/queues/external); node ring = health, edge width = traffic, failing edges animate; hover to focus a neighbourhood, click for RED vs baseline, operations, pods and errors.
- **Traces** — filter by service, edge, errors or duration; waterfall with span attributes and trace-correlated logs.
- **Coverage & signals** — Autopilot discovery/enablement and raw Metrics / Logs / Traces.
