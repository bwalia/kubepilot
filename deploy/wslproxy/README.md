# wslproxy edge specs for demo.kubepilot.org

App-repo mirror of the fishers.cloud pattern. **Authoritative live copies** also
live in the wslproxy GitOps plane:

- `wslproxy` repo: `data/servers/prod/host:demo.kubepilot.org.json`
- `wslproxy` repo: `data/rules/prod/kubepilot-demo-default.json`

Pushing those to `wslproxy` and running **Deploy WSLProxy Virtual Servers**
(`TARGET_ENV=prod`, `TARGET_HOST=lon1.pop0.uk`) applies them on lon1.

This directory is the kubepilot-side source consumed by
`.github/workflows/register-demo-edge.yml`, which:

1. Upserts Cloudflare CNAME `demo.kubepilot.org` → `lon1.pop0.uk` (DNS-only)
2. Imports the rule + vhost via wslproxy `/api/projects/import`

Backend is `193.237.176.232:8888` (k3s1 traefik-edge). Traefik matches
`Host: demo.kubepilot.org` to namespace `kubepilot-demo`.
