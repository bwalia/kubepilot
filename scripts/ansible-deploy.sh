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

if [[ -z "${DEPLOY_SSH_KEY:-}" ]]; then
  echo "::error::DEPLOY_SSH_KEY secret is not set — needed to SSH into the ${ENVIRONMENT} host." >&2
  exit 1
fi

if [[ ! -x "$ROOT_DIR/artifact/kubepilot" ]] && [[ ! -f "$ROOT_DIR/artifact/kubepilot" ]]; then
  echo "::error::Missing build artifact at $ROOT_DIR/artifact/kubepilot — the build job must run first." >&2
  exit 1
fi

# ── Install the SSH key (cleaned up on exit) ─────────────────────────────────
SSH_KEY_FILE="$(mktemp)"
cleanup() { rm -f "$SSH_KEY_FILE"; }
trap cleanup EXIT
printf '%s\n' "$DEPLOY_SSH_KEY" > "$SSH_KEY_FILE"
chmod 600 "$SSH_KEY_FILE"
export ANSIBLE_PRIVATE_KEY_FILE="$SSH_KEY_FILE"

# ── Ensure a target-compatible ansible-core ──────────────────────────────────
# The home-lab hosts ship an older system Python (< 3.7). ansible-core 2.17+
# drops support for those targets (its modules use `from __future__ import
# annotations`, which errors on old interpreters). Pin to the 2.16 line, which
# still supports Python 2.7/3.6 targets.
#
# 2.16 itself needs a controller Python >= 3.10, and this runner's default
# `python3` is older (pip there only offers ansible-core <= 2.11). So pick the
# newest Python >= 3.10 on the box to install + run ansible with.
#
# The interpreter must also have a usable pip. Some runners ship a bare
# python3.13 with no pip module (`No module named pip`); selecting it purely by
# version leaves us unable to install ansible-core. So require pip too — trying
# `ensurepip` first — and otherwise fall through to the next candidate (e.g. the
# runner's python3.12, which does ship pip).
ANSIBLE_CORE_SPEC="ansible-core>=2.16,<2.17"
PYBIN=""
for c in python3.13 python3.12 python3.11 python3.10 python3; do
  command -v "$c" >/dev/null 2>&1 || continue
  v="$("$c" -c 'import sys; print("%d%02d" % sys.version_info[:2])' 2>/dev/null || echo 0)"
  [ "${v:-0}" -ge 310 ] || continue
  # Ensure pip is importable, bootstrapping it if the interpreter allows.
  "$c" -m pip --version >/dev/null 2>&1 \
    || "$c" -m ensurepip --default-pip >/dev/null 2>&1 || true
  "$c" -m pip --version >/dev/null 2>&1 || continue
  PYBIN="$c"; break
done
if [ -z "$PYBIN" ]; then
  echo "::error::No Python >= 3.10 with pip found on the runner to install ansible-core ${ANSIBLE_CORE_SPEC}." >&2
  exit 1
fi
echo "Using controller Python: $("$PYBIN" --version 2>&1) ($PYBIN)"
# Some runners ship a PEP-668 "externally managed" Python (e.g. Debian's
# python3.13) that refuses `pip install --user`. Retry with the override.
"$PYBIN" -m pip install --user --quiet "$ANSIBLE_CORE_SPEC" \
  || "$PYBIN" -m pip install --user --quiet --break-system-packages "$ANSIBLE_CORE_SPEC"
export PATH="$HOME/.local/bin:$PATH"
hash -r 2>/dev/null || true
ansible-playbook --version | head -1

# ── Run the play, limited to the requested environment ───────────────────────
cd "$ANSIBLE_DIR"
exec ansible-playbook site.yml --limit "$ENVIRONMENT"
