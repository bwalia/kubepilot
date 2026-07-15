#!/usr/bin/env bash
# ─────────────────────────────────────────────────────────────────────────────
# Deploy KubePilot to a home-lab host as a systemd service via Ansible.
#
#   scripts/ansible-deploy.sh <env>      # env = int | test
#
# Run from the GitHub Actions deploy jobs (see .github/workflows/deploy.yml), but
# also runnable by hand from a machine on the LAN. Expects the built artifact in
# ./artifact (kubepilot + dashboard-out) and these environment variables:
#
#   DEPLOY_SSH_KEY            private key for the inventory's ansible_user (required)
#   KUBEPILOT_AUTH_PASSWORD   dashboard password — auth is on by default (required)
#   KUBEPILOT_AUTH_USERNAME   optional, defaults to "admin"
#
# The host `bwalia` must have passwordless sudo (the playbook uses become).
# ─────────────────────────────────────────────────────────────────────────────
set -euo pipefail

ENVIRONMENT="${1:-}"
case "$ENVIRONMENT" in
  int|test) ;;
  *) echo "usage: $0 <int|test>" >&2; exit 2 ;;
esac

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ANSIBLE_DIR="$ROOT_DIR/deploy/ansible"

if [[ -z "${KUBEPILOT_AUTH_PASSWORD:-}" ]]; then
  echo "::error::DASHBOARD_AUTH_PASSWORD secret is not set — dashboard auth is enabled by default and the service will not start without a credential." >&2
  exit 1
fi

if [[ ! -x "$ROOT_DIR/artifact/kubepilot" ]] && [[ ! -f "$ROOT_DIR/artifact/kubepilot" ]]; then
  echo "::error::Missing build artifact at $ROOT_DIR/artifact/kubepilot — the build job must run first." >&2
  exit 1
fi

# ── SSH key ──────────────────────────────────────────────────────────────────
# DEPLOY_SSH_KEY is optional. If provided, write it to a temp file and use it.
# Otherwise fall back to the runner's own SSH setup (~/.ssh + agent) — the Mac
# Studio runner already has a key authorized for bwalia on the target hosts, so
# no private key needs to travel through CI secrets.
if [[ -n "${DEPLOY_SSH_KEY:-}" ]]; then
  SSH_KEY_FILE="$(mktemp)"
  cleanup() { rm -f "$SSH_KEY_FILE"; }
  trap cleanup EXIT
  printf '%s\n' "$DEPLOY_SSH_KEY" > "$SSH_KEY_FILE"
  chmod 600 "$SSH_KEY_FILE"
  export ANSIBLE_PRIVATE_KEY_FILE="$SSH_KEY_FILE"
  echo "Using DEPLOY_SSH_KEY for SSH auth."
else
  for k in "$HOME/.ssh/id_ed25519" "$HOME/.ssh/id_rsa"; do
    if [[ -f "$k" ]]; then
      export ANSIBLE_PRIVATE_KEY_FILE="$k"
      echo "DEPLOY_SSH_KEY not set — using the runner's SSH key: $k"
      break
    fi
  done
fi

# ── Ensure ansible is available on the runner ────────────────────────────────
# Prefer a pre-installed ansible (`sudo apt-get install -y ansible` on the
# runner) — the most robust option. Debian strips `ensurepip` and often ships no
# pip at all, so the pip bootstrap can't even run; a pre-installed ansible
# sidesteps that. Fall back to a pip bootstrap only when no system ansible.
ANSIBLE_CORE_SPEC="ansible-core>=2.16,<2.17"

# pip-based bootstrap, used only when no system ansible is present.
# The home-lab TARGET hosts ship an older system Python (< 3.7). ansible-core
# 2.17+ drops support for those targets (its modules use `from __future__ import
# annotations`, which errors on old interpreters), so pin to the 2.16 line.
#
# 2.16 needs a CONTROLLER Python >= 3.10 with a usable pip. Pick the newest such
# interpreter — requiring an importable pip (trying `ensurepip` first, since
# some runners ship a bare python3.13 with `No module named pip`) and otherwise
# falling through to the next candidate (e.g. python3.12).
bootstrap_ansible() {
  local c v PYBIN=""
  for c in python3.13 python3.12 python3.11 python3.10 python3; do
    command -v "$c" >/dev/null 2>&1 || continue
    v="$("$c" -c 'import sys; print("%d%02d" % sys.version_info[:2])' 2>/dev/null || echo 0)"
    [ "${v:-0}" -ge 310 ] || continue
    "$c" -m pip --version >/dev/null 2>&1 \
      || "$c" -m ensurepip --default-pip >/dev/null 2>&1 || true
    "$c" -m pip --version >/dev/null 2>&1 || continue
    PYBIN="$c"; break
  done
  if [ -z "$PYBIN" ]; then
    echo "::error::No system ansible, and no Python >= 3.10 with pip on the runner to install ${ANSIBLE_CORE_SPEC}. Fix: install ansible on the runner host — 'sudo apt-get install -y ansible'." >&2
    exit 1
  fi
  echo "Using controller Python: $("$PYBIN" --version 2>&1) ($PYBIN)"
  # Some runners ship a PEP-668 "externally managed" Python that refuses
  # `pip install --user`. Retry with the override.
  "$PYBIN" -m pip install --user --quiet "$ANSIBLE_CORE_SPEC" \
    || "$PYBIN" -m pip install --user --quiet --break-system-packages "$ANSIBLE_CORE_SPEC"
  export PATH="$HOME/.local/bin:$PATH"
  hash -r 2>/dev/null || true
}

if command -v ansible-playbook >/dev/null 2>&1; then
  echo "Using pre-installed ansible: $(ansible-playbook --version 2>/dev/null | head -1)"
else
  echo "No system ansible found — attempting a pip bootstrap of ${ANSIBLE_CORE_SPEC}."
  bootstrap_ansible
fi
ansible-playbook --version | head -1

# ── Run the play, limited to the requested environment ───────────────────────
cd "$ANSIBLE_DIR"
exec ansible-playbook site.yml --limit "$ENVIRONMENT"
