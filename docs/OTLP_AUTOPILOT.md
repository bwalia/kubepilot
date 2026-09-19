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
- `POST /api/v1/otel/ingest` — push samples into the managed store (collector path)

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

Open **OTLP** in the top nav (`/otel`) for coverage, cluster vs application planes, and Metrics / Logs / Traces overview.
