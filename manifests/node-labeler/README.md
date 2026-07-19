# node-labeler

Stamps each node with its real, non-WireGuard **LAN IP** as the label
`kubepilot.io/lan-ip`.

On WireGuard-meshed k3s clusters, Kubernetes only knows the WireGuard address as
a node's internal IP, so the physical LAN IP is absent from the node object and
the dashboard would otherwise show the WireGuard address mislabeled as LAN. With
this label present, KubePilot trusts it: the LAN badge shows the real LAN IP and
the WireGuard address is correctly shown under **Tunnel**.

## How it works

A DaemonSet runs one tiny pod per node with `hostNetwork: true`. Each pod:

1. Reads the source address of the node's default route
   (`ip route get 1.1.1.1`) — the physical LAN IP, not the WireGuard tunnel.
2. Patches its own node's `kubepilot.io/lan-ip` label via the API server using
   its ServiceAccount token (RBAC: `nodes` `get,list,patch` only).
3. Re-applies every `INTERVAL` seconds (default 300) so the label self-heals.

## Install

```sh
kubectl apply -f manifests/node-labeler/
```

Verify:

```sh
kubectl get nodes -L kubepilot.io/lan-ip
kubectl -n kubepilot-system logs -l app.kubernetes.io/name=node-labeler
```

## Notes

- The image is stock `alpine:3.20` plus `iproute2` + `curl` installed at start;
  swap it for a prebuilt image that already bundles both if your nodes lack
  outbound internet.
- Detection assumes the node's default route is via the LAN, not the WireGuard
  tunnel (true for typical homelab setups).
