#!/usr/bin/env bash
# ─────────────────────────────────────────────────────────────────────────────
# Install / refresh the KubePilot systemd service from the repo templates.
#
#   sudo scripts/install-kubepilot-systemd.sh
#
# Idempotent and safe to re-run:
#   - Always refreshes the unit file from deploy/systemd/kubepilot.service.
#   - Seeds /etc/kubepilot/kubepilot.env from the example ONLY if it is missing,
#     so existing credentials are never clobbered. Edit that file to set the
#     dashboard password, then re-run (or `systemctl restart kubepilot`).
# ─────────────────────────────────────────────────────────────────────────────
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
UNIT_SRC="$ROOT_DIR/deploy/systemd/kubepilot.service"
ENV_SRC="$ROOT_DIR/deploy/systemd/kubepilot.env.example"
UNIT_DST="/etc/systemd/system/kubepilot.service"
ENV_DST="/etc/kubepilot/kubepilot.env"

if [[ "${EUID}" -ne 0 ]]; then
  echo "error: must run as root — re-run with: sudo $0" >&2
  exit 1
fi

[[ -f "$UNIT_SRC" ]] || { echo "error: missing $UNIT_SRC" >&2; exit 1; }
[[ -f "$ENV_SRC" ]]  || { echo "error: missing $ENV_SRC" >&2; exit 1; }

echo "Installing unit -> $UNIT_DST"
install -D -m 644 "$UNIT_SRC" "$UNIT_DST"

if [[ -f "$ENV_DST" ]]; then
  echo "Keeping existing env file -> $ENV_DST (not overwriting secrets)"
else
  echo "Seeding env file -> $ENV_DST (mode 0600) — edit it to set the password"
  install -D -m 600 "$ENV_SRC" "$ENV_DST"
fi

echo "Reloading systemd and (re)starting kubepilot"
systemctl daemon-reload
systemctl enable kubepilot >/dev/null 2>&1 || true
systemctl restart kubepilot

echo
echo "Done. Verify auth is enforced:"
echo "  curl -s -o /dev/null -w 'no-creds:   %{http_code}\\n' http://localhost:8383/"
echo "  curl -s -o /dev/null -w 'with-creds: %{http_code}\\n' -u admin:PASSWORD http://localhost:8383/"
echo "Expected: no-creds 401, with-creds 200."
