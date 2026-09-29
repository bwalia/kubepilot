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
| Edge + DNS (reconcile with secrets) | `bwalia/fishers` → `devops/wslproxy/host-demo.kubepilot.org.json` + rule (mirror of kubepilot specs) | `register-kubepilot-demo-edge.yml` (push / hourly / `workflow_dispatch`) |
| App-side mirror | `deploy/wslproxy/*.json` + `scripts/upsert-demo-cname.sh` | `.github/workflows/register-demo-edge.yml` (needs CF/wslproxy secrets on **kubepilot**) |

Keep the three JSON copies aligned when changing backends or hostnames:

1. `kubepilot` `deploy/wslproxy/`
2. `fishers` `devops/wslproxy/` (drives the live CF + import reconcile)
3. `wslproxy` `data/{servers,rules}/prod/` (lon1 POP file plane)

Required secrets live on **bwalia/fishers** today (`CLOUDFLARE_API_TOKEN`, `WSLPROXY_*`). Copy the same four onto **bwalia/kubepilot** if you want `register-demo-edge.yml` to run from this repo.

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
