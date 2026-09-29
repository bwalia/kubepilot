# App Review demo API (`demo.kubepilot.org`)

Hosted fixture KubePilot API for Apple App Review, plus an in-app **Try demo**
path that falls back to offline fixtures when the public URL is unreachable.

## Architecture

```
Apple reviewer
  → https://demo.kubepilot.org  (Cloudflare CNAME, grey-cloud / DNS-only)
  → lon1.pop0.uk (wslproxy POP)
  → 193.237.176.232:8888 (k3s1 traefik-edge)
  → Ingress host demo.kubepilot.org → kubepilot-demo-api
```

Same edge pattern as fishers.cloud (`devops/wslproxy` + `traefik-edge`).

## GitOps (persistent)

| Layer | Source of truth | Apply path |
|---|---|---|
| Workload | `deploy/demo-api/` on k3s1 | `kubectl apply -k deploy/demo-api/` |
| Edge vhost + rule (POP files) | `bwalia/wslproxy` → `data/servers/prod/host:demo.kubepilot.org.json` + `data/rules/prod/kubepilot-demo-default.json` | Push + **Deploy WSLProxy Virtual Servers** (`prod` / `lon1.pop0.uk`) |
| Edge + DNS (app-side reconcile) | `deploy/wslproxy/*.json` + `scripts/upsert-demo-cname.sh` | `.github/workflows/register-demo-edge.yml` on push / `workflow_dispatch` |

Required GitHub secrets on **bwalia/kubepilot** (same values as fishers):

- `CLOUDFLARE_API_TOKEN` — Zone.DNS edit on `kubepilot.org`
- `WSLPROXY_USER` / `WSLPROXY_PASSWORD` / `WSLPROXY_GATEWAY_URL`

Keep the kubepilot `deploy/wslproxy` specs and the wslproxy `data/` copies in sync when changing backends or hostnames.

## Credentials

| | |
|---|---|
| URL | `https://demo.kubepilot.org` |
| Auth | HTTP Basic |
| User | `apple` |
| Password | `review` |

Rotate `deploy/demo-api/deployment.yaml` Secret after approval.

`/healthz` is open (no auth). Everything under `/api/v1/*` requires Basic auth.

## Deploy (k3s1)

```bash
# 1. Build & push (context = repo root; or use ko)
ko build --bare --platform=linux/amd64 --tags=latest ./cmd/demo-api
# KO_DOCKER_REPO=docker.io/bwalia/kubepilot-demo-api

# 2. Apply
KUBECONFIG=~/.kube/k3s1.yaml kubectl apply -k deploy/demo-api/

# 3. Smoke (in-cluster / via Host header to traefik)
curl -s -H 'Host: demo.kubepilot.org' http://193.237.176.232:8888/healthz
curl -s -u apple:review -H 'Host: demo.kubepilot.org' \
  http://193.237.176.232:8888/api/v1/troubleshooting/summary | head -c 200
```

## Edge + DNS (manual / local)

```bash
export CLOUDFLARE_API_TOKEN=…
./scripts/upsert-demo-cname.sh
# Prefer: gh workflow run register-demo-edge.yml
```

## In-app Try demo

Onboarding → **Try demo** prefills the hosted URL + Basic `apple`/`review`.
If the health check fails, the app seeds an offline fixture account so review
never dead-ends without network.

## Local mock

```bash
go run ./cmd/demo-api
# DEMO_API_ADDR=:8383 DEMO_USER=apple DEMO_PASSWORD=review
```
