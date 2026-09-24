#!/bin/bash
# Mac Studio launchd entrypoint — versioned in git (deploy/macstudio/start-server.sh).
# Every Deploy → Mac Studio workflow copies this file to ~/.kubepilot/start-server.sh.
#
# Config layering (later wins):
#   1. kubepilot.env     — versioned defaults (model pin, CORS, telemetry)
#   2. secrets.env       — local-only password / kubeconfig (never in git)
#   3. launchd EnvironmentVariables — optional operator overrides
set -euo pipefail

DEST="$(cd "$(dirname "$0")" && pwd)"
cd "$DEST"

set -a
# shellcheck disable=SC1091
[ -f "$DEST/kubepilot.env" ] && . "$DEST/kubepilot.env"
# shellcheck disable=SC1091
[ -f "$DEST/secrets.env" ] && . "$DEST/secrets.env"
set +a

# Hard defaults if somehow neither file set the interactive model.
: "${KUBEPILOT_OLLAMA_BASE_URL:=http://127.0.0.1:11434/v1}"
: "${KUBEPILOT_OLLAMA_MODEL:=llama3.2:3b-dash}"
: "${KUBEPILOT_DASHBOARD_AUTH_ENABLED:=true}"
: "${KUBEPILOT_DASHBOARD_AUTH_USERNAME:=admin}"

if [ -z "${KUBEPILOT_DASHBOARD_AUTH_PASSWORD:-}" ] && [ "${KUBEPILOT_DASHBOARD_AUTH_ENABLED}" = "true" ]; then
  echo "kubepilot: KUBEPILOT_DASHBOARD_AUTH_PASSWORD is unset — create $DEST/secrets.env" >&2
  exit 1
fi

# Pass model/URL on the CLI. Empty cobra flag defaults otherwise win over env
# via viper BindPFlag, which previously left the process on a stale model.
exec "$DEST/bin/kubepilot" serve \
  --ollama-base-url="${KUBEPILOT_OLLAMA_BASE_URL}" \
  --ollama-model="${KUBEPILOT_OLLAMA_MODEL}" \
  --dashboard-port=8383 \
  --mcp-port=9090 \
  --runbooks-dir="$DEST/runbooks" \
  --state-dir="$DEST/state" \
  --state-retention=720h
