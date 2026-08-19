# Observing KubePilot

KubePilot watches your cluster. This document is about watching KubePilot — the
OpenTelemetry traces and metrics the server emits about its own behaviour.

## What is instrumented

| Area | Spans | Metrics |
|---|---|---|
| Dashboard + REST API | `GET /api/v1/clusters/pods/{namespace}/{pod}` | `http.server.request.duration`, `http.server.active_requests`, request/response body size |
| Kubernetes API client | `k8s GET pods/log` | `kubepilot.k8s.client.duration`, `kubepilot.k8s.client.errors` |
| AI backend | `chat llama3.1:8b` | `gen_ai.client.operation.duration`, `gen_ai.client.token.usage`, `kubepilot.ai.errors` |
| Anomaly detection | `watcher.poll` | `kubepilot.anomalies.detected` |
| Root cause analysis | `rca.AnalyzePod` | `kubepilot.rca.reports`, `kubepilot.rca.duration` |
| Autopilot | `autopilot.HandleReport` | `kubepilot.autopilot.actions` |

Spans nest the way the work does, which is the point. A slow RCA shows up as
`rca.AnalyzePod` with its API-server calls and its `chat` span underneath, so
you can see immediately whether the cluster or the model was the slow part.

## Two independent outputs

**Prometheus (`/metrics`)** is on by default and needs no collector. It is served
from the dashboard port by the same process that serves the UI.

**OTLP export** is off until you set an endpoint. With none configured KubePilot
opens no outbound telemetry connection at all: tracing is a no-op and metrics
stay local. This is the normal state for a host with no collector on its network.

## Turning on OTLP export

Set an endpoint. Everything else has a working default.

```bash
# systemd host — /etc/kubepilot/kubepilot.env
KUBEPILOT_OTEL_ENDPOINT=otel-collector.lan:4317
KUBEPILOT_OTEL_ENVIRONMENT=int          # names this host in a shared backend
KUBEPILOT_OTEL_SERVICE_NAMESPACE=home-lab
```

```bash
# Helm
helm upgrade --install kubepilot charts/kubepilot \
  --set kubepilot.otel.endpoint=otel-collector.observability.svc:4317 \
  --set kubepilot.otel.environment=prod
```

```bash
# Directly
kubepilot serve --otel-endpoint=localhost:4317 --otel-environment=dev
```

The standard `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_EXPORTER_OTLP_PROTOCOL`,
`OTEL_SERVICE_NAME`, and `OTEL_RESOURCE_ATTRIBUTES` variables are honoured too,
so an operator or sidecar that injects them automatically will just work. The
`KUBEPILOT_OTEL_*` settings take precedence when both are present.

### Settings

| Setting | Default | Notes |
|---|---|---|
| `otel_endpoint` | `""` | Empty disables all OTLP export |
| `otel_protocol` | `grpc` | `grpc` (:4317) or `http/protobuf` (:4318) |
| `otel_insecure` | `true` | Plaintext OTLP; fine on a trusted LAN or in-cluster |
| `otel_headers` | `""` | `key=value,key=value`; treat as a secret |
| `otel_sample_ratio` | `1.0` | Head sampling for root spans |
| `otel_metric_interval` | `60s` | OTLP push interval; Prometheus is unaffected |
| `otel_service_name` | `kubepilot` | |
| `otel_service_namespace` | `""` | Groups several installs, e.g. `home-lab` |
| `otel_environment` | `""` | `int`, `test`, `prod` — tells instances apart |
| `metrics_enabled` | `true` | Serves `/metrics` |
| `metrics_require_auth` | `false` | See below |

## Scraping `/metrics`

`/metrics` is exempt from dashboard auth by default, alongside `/healthz`. That
matches what scrapers expect, and the endpoint is built to carry nothing
sensitive: its labels are route templates, HTTP verbs, status classes, and
resource kinds. Namespace names, pod names, and object names never appear.

If you would rather close it anyway, set `metrics_require_auth: true` and give
your scraper the dashboard credentials.

With the Prometheus Operator installed, the chart ships a ServiceMonitor:

```bash
helm upgrade --install kubepilot charts/kubepilot \
  --set metrics.serviceMonitor.enabled=true
```

It scrapes the **dashboard** port, not the operator metrics port — `kubepilot
serve` exposes `/metrics` from the same listener that serves the UI.

## Label cardinality

Every label on every metric is drawn from a bounded set. This is deliberate and
it is enforced by tests: `/api/v1/clusters/pods/prod/checkout-7d9f8b-x2k4l` is
recorded as the route template `/clusters/pods/{namespace}/{pod}`, and the
API-server path `/api/v1/namespaces/kube-system/pods/coredns-abc/log` is
recorded as `pods/log`.

Pod and object names are unbounded. Putting them in a metric label is how a
metrics backend falls over, so they are kept on spans — where high-cardinality
detail belongs — and never on metrics.

## Cost when disabled

With no OTLP endpoint, spans are created against OpenTelemetry's no-op tracer
and discarded without allocation. Metrics are still recorded into the local
registry, which is a lock-free atomic add per instrument. Neither is measurable
against the cost of the Kubernetes API call or LLM inference it wraps.

## Verifying it works

```bash
# Local metrics, no collector needed
curl -s localhost:8383/metrics | grep kubepilot_

# Full pipeline against a throwaway collector
docker run -d --name otelcol -p 4317:4317 \
  -v "$PWD/otelcol.yaml:/etc/otelcol/config.yaml" \
  otel/opentelemetry-collector-contrib:latest --config /etc/otelcol/config.yaml
kubepilot serve --otel-endpoint=127.0.0.1:4317 --otel-insecure
docker logs otelcol | grep -E "Traces|Metrics"
```

A minimal collector config that prints what it receives:

```yaml
receivers:
  otlp:
    protocols:
      grpc: { endpoint: 0.0.0.0:4317 }
exporters:
  debug: { verbosity: detailed }
service:
  pipelines:
    traces:  { receivers: [otlp], exporters: [debug] }
    metrics: { receivers: [otlp], exporters: [debug] }
```

## Failure behaviour

Telemetry never takes the server down. If the collector is unreachable at
startup, or the configuration is malformed, KubePilot logs a warning and serves
without export. Export errors during operation are logged through the same zap
logger as everything else rather than written to stderr by the SDK.
