# KubePilot

KubePilot is a Kubernetes operations assistant with an API/dashboard server, AI engine, and MCP server.

## Build

```bash
make build
```

## Run Locally

By default, `serve` now runs the dashboard/API server on port `8383`.

```bash
KUBEPILOT_KUBECONFIG=$HOME/.kube/config ./dist/kubepilot serve --dashboard-port=8383
```

Open:

- Dashboard/API: `http://localhost:8383`
- MCP server: `:9090`

## Useful Commands

```bash
# Start with defaults (dashboard/API on 8383)
./dist/kubepilot serve

# Override dashboard/API port
./dist/kubepilot serve --dashboard-port=8390

# Query RCA list against local server
./dist/kubepilot rca list --server http://localhost:8383
```
