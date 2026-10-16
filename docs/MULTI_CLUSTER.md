# Multiple clusters

One KubePilot server can manage any number of clusters, and every browser tab
or iOS session chooses which one it is looking at. Switching changes only your
own view: other tabs, other people and the iOS app keep their own cluster.

## How it works

The server keeps a list of saved kubeconfig files. Each context in each file is
one selectable cluster, with a stable ID derived from the file path and context
name.

A client selects a cluster per request with a header:

```
X-KubePilot-Cluster: 7ba430b969fe
```

`?cluster=<id>` works too, for links that cannot set headers. A request with
neither uses the **server default** cluster.

The default cluster is also the one background work watches: Autopilot, RCA,
anomaly detection, OTLP Autopilot and scheduled jobs. Remediating an RCA report
therefore always acts on the default cluster, whatever the session selected.

## API

| Method & path | Purpose |
|---|---|
| `GET /api/v1/clusters` | List clusters. Add `?probe=true` to check each API server (4s timeout each, run in parallel). |
| `POST /api/v1/clusters/kubeconfigs` | Save a kubeconfig already on the server: `{"path": "...", "activate": false}` |
| `POST /api/v1/clusters/kubeconfigs/upload` | Upload a kubeconfig file (multipart field `kubeconfig`, optional `activate=false`) |
| `POST /api/v1/clusters/kubeconfigs/base64` | Upload base64 content: `{"name": "...", "content_base64": "...", "activate": false}` |
| `DELETE /api/v1/clusters/kubeconfigs?path=...` | Forget a saved kubeconfig (the file is kept; the default's file cannot be removed) |
| `POST /api/v1/clusters/default` | Make a cluster the server default: `{"id": "..."}` |

Adding responses include `added_ids`, the clusters the new file contributed, so
a client can select one straight away. `activate` defaults to `true` for older
clients, which makes the upload the server default as before.

The add, remove and default endpoints are mutations: they need
`--enable-kubeconfig-mutations` (`KUBEPILOT_ENABLE_KUBECONFIG_MUTATIONS=true`).
Selecting a cluster with the header does not.

An ID that is no longer on the server answers `409 Conflict`; the web dashboard
and the iOS app then fall back to the default cluster.

## Web dashboard

The cluster picker sits in the top bar on every page. The choice is kept per
tab (`sessionStorage`); a new tab starts on the cluster picked most recently.
Switching clears cached data, resets the namespace filter to all namespaces
and remounts the page so nothing from the previous cluster lingers.

**Manage clusters…** adds a kubeconfig by server path, file upload or paste
(YAML or base64), shows which clusters are reachable, makes a cluster the
server default, and forgets kubeconfigs that are stale or unreadable.

## iOS app

Tap the cluster banner on the Dashboard, or **Settings → Switch or add
cluster…**, to:

- switch cluster on the current server (remembered per server across launches);
- add a cluster by importing a kubeconfig file from Files;
- long-press a cluster to make it the server default;
- switch between KubePilot servers, add another, or swipe to remove one.

Servers that predate the cluster list still work; the app shows their single
active context.
