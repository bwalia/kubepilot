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
# 1. Build & push (context = repo root)
docker build -f deploy/demo-api/Dockerfile -t docker.io/bwalia/kubepilot-demo-api:latest .
docker push docker.io/bwalia/kubepilot-demo-api:latest

# 2. Apply
KUBECONFIG=~/.kube/k3s1.yaml kubectl apply -k deploy/demo-api/

# 3. Smoke (in-cluster / via Host header to traefik)
curl -s -H 'Host: demo.kubepilot.org' http://193.237.176.232:8888/healthz
curl -s -u apple:review -H 'Host: demo.kubepilot.org' \
  http://193.237.176.232:8888/api/v1/troubleshooting/summary | head -c 200
```

## Edge + DNS

1. **wslproxy** — apply `deploy/wslproxy/host-demo.kubepilot.org.json` and
   `deploy/wslproxy/rule-kubepilot-demo-default.json` (same flow as fishers
   `register-edge-vhost`).
2. **Cloudflare** — CNAME `demo` → `lon1.pop0.uk`, **proxied: false** (grey-cloud):

```bash
export CLOUDFLARE_API_TOKEN=…
./scripts/upsert-demo-cname.sh
```

`kubepilot.org` is not in the cluster ExternalDNS `--domain-filter` (only
`diytaxreturn.co.uk`), so the CNAME must be upserted explicitly.

## In-app Try demo

Onboarding → **Try demo** prefills the hosted URL + Basic `apple`/`review`.
If the health check fails, the app seeds an offline fixture account so review
never dead-ends without network.

## Local mock

```bash
go run ./cmd/demo-api
# DEMO_API_ADDR=:8383 DEMO_USER=apple DEMO_PASSWORD=review
```
