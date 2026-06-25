# Deployment (systemd) & Dashboard Authentication

This guide covers running KubePilot as a host service with systemd and securing
the dashboard with HTTP authentication. For in-cluster deployment use the Helm
chart (`charts/kubepilot/`); auth there is configured via `kubepilot.auth.*` in
`values.yaml` and injected from a Secret.

## How dashboard auth behaves

When auth is enabled, the `withAuth` middleware (`internal/dashboard/auth.go`)
guards **every** dashboard and API route except `/healthz` (kept open for
liveness probes) and `OPTIONS` preflight requests. Unauthenticated requests get:

```
HTTP/1.1 401 Unauthorized
WWW-Authenticate: Basic realm="KubePilot"
```

That `WWW-Authenticate: Basic` header is what makes a browser present its native
sign-in dialog over a blank page. Once the username/password match, the dashboard
loads normally; the browser replays the credentials on every subsequent request.

Credentials come from these settings (viper keys; as env vars they take the
`KUBEPILOT_` prefix, e.g. `KUBEPILOT_DASHBOARD_AUTH_PASSWORD`):

| Setting | Purpose |
|---|---|
| `dashboard_auth_enabled` | Master switch. |
| `dashboard_auth_username` + `dashboard_auth_password` | HTTP Basic credentials. |
| `dashboard_auth_token` | Optional `Authorization: Bearer <token>` for API clients. |

> If `dashboard_auth_enabled=true` but neither a token nor a username+password
> pair is set, the server **refuses to start** — this prevents accidentally
> exposing an unauthenticated dashboard.

## Rebuild the dashboard on every deploy

The Go server serves the Next.js **static export** from `./dashboard/out` (a
gitignored build artifact). The home page (`/` → the Kubernetes CoPilot) only
exists once that export is built; if `dashboard/out` is stale or missing its root
`index.html`, Go falls back to rendering a raw directory listing at `/`.

Always rebuild the dashboard as part of a deploy so `/` serves the CoPilot:

```bash
make dashboard   # next build → regenerates dashboard/out (incl. out/index.html)
make build       # rebuild the Go binary
```

## Install with systemd

Templates live in `deploy/systemd/`:

- `kubepilot.service` — the unit. Carries no secrets; reads everything from an
  `EnvironmentFile`.
- `kubepilot.env.example` — the environment file template, with auth enabled by
  default.

```bash
# 1. Build and place the binary (path must match WorkingDirectory in the unit).
make build
sudo install -D -m 755 dist/kubepilot /opt/kubepilot/kubepilot

# 2. Install the unit + seed the env file, then start the service.
sudo scripts/install-kubepilot-systemd.sh

# 3. Set the password and restart.
sudo sed -i 's/^KUBEPILOT_DASHBOARD_AUTH_PASSWORD=.*/KUBEPILOT_DASHBOARD_AUTH_PASSWORD=YOUR_STRONG_PASSWORD/' \
  /etc/kubepilot/kubepilot.env
sudo systemctl restart kubepilot
```

`install-kubepilot-systemd.sh` is idempotent: it always refreshes the unit, but
seeds `/etc/kubepilot/kubepilot.env` (mode `0600`) **only if it does not already
exist**, so re-running never overwrites credentials.

## Automated deploy (GitHub Actions → Ansible)

`.github/workflows/deploy.yml` deploys to the home-lab hosts as a systemd
service — no Kubernetes/Helm involved. After **Publish Docker Image** succeeds on
`master` (or via manual dispatch) it:

1. **build** (hosted runner) — compiles the Go binary and Next.js dashboard once
   and uploads them as the `kubepilot-dist` artifact.
2. **deploy-int** (LAN runner, label `int`) — downloads the artifact and runs the
   Ansible role (`deploy/ansible/`) over SSH against `bwalia@192.168.1.193`.
3. **deploy-test** (LAN runner, label `test`) — same for `bwalia@192.168.1.140`,
   but only if `int` succeeded, so a bad build never reaches test.

The Ansible role installs the binary to `/opt/kubepilot`, syncs `dashboard/out`,
renders `/etc/kubepilot/kubepilot.env` (mode `0600`) from secrets, installs the
systemd unit, restarts the service, and health-gates on `GET /healthz`.

Required GitHub secrets (per environment):

| Secret | Purpose |
|---|---|
| `DEPLOY_SSH_KEY` | Private key for `bwalia` on the target host. |
| `DASHBOARD_AUTH_PASSWORD` | Dashboard password (auth is on by default). |
| `DASHBOARD_AUTH_USERNAME` | Optional; defaults to `admin`. |

Host prerequisite: `bwalia` has **passwordless sudo**. To run a deploy by hand
from a LAN machine: `DEPLOY_SSH_KEY=… KUBEPILOT_AUTH_PASSWORD=… scripts/ansible-deploy.sh int`.

### Why the env file, not CLI flags

Secrets passed as `--dashboard-auth-password=…` show up in `ps` output and the
rendered unit. Keeping them in a `0600` `EnvironmentFile` avoids that and keeps
the committed unit identical across hosts.

## Verify auth is enforced

```bash
# No credentials → 401 + the auth challenge that triggers the browser dialog:
curl -s -D - -o /dev/null http://localhost:8383/ | grep -iE 'HTTP/|www-authenticate'

# Correct credentials → 200:
curl -s -o /dev/null -w '%{http_code}\n' -u admin:YOUR_STRONG_PASSWORD http://localhost:8383/

# Health check stays open (probes must not need auth) → 200:
curl -s -o /dev/null -w '%{http_code}\n' http://localhost:8383/healthz
```

Expected: `401` with `WWW-Authenticate: Basic realm="KubePilot"` for no creds,
`200` with correct creds, `200` for `/healthz`.

## Changing the password

```bash
sudoedit /etc/kubepilot/kubepilot.env     # edit KUBEPILOT_DASHBOARD_AUTH_PASSWORD
sudo systemctl restart kubepilot
```

## Rotating to a token for API clients

Add a Bearer token alongside (or instead of) Basic auth, then call the API with
`-H "Authorization: Bearer <token>"`:

```bash
echo "KUBEPILOT_DASHBOARD_AUTH_TOKEN=$(openssl rand -hex 32)" | sudo tee -a /etc/kubepilot/kubepilot.env
sudo systemctl restart kubepilot
```
