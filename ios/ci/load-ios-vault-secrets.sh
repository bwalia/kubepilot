#!/usr/bin/env bash
#
# load-ios-vault-secrets.sh — pull KubePilot iOS signing material from Vault.
#
# Reads the flat KV v2 secret `secret/kubepilot/ios` (override with
# KUBEPILOT_IOS_VAULT_PATH), materialises the ASC API key and distribution-
# certificate private key as files, and exports identifiers for fastlane.
#
#   CI:    ./ci/load-ios-vault-secrets.sh           # appends to $GITHUB_ENV
#   local: eval "$(./ci/load-ios-vault-secrets.sh)" # exports to shell
#
set -euo pipefail

VAULT_TOKEN_FILE="${VAULT_TOKEN_FILE:-$HOME/.secrets/vault/token.json}"
VAULT_SECRET_PATH="${KUBEPILOT_IOS_VAULT_PATH:-secret/kubepilot/ios}"
WORKDIR="${IOS_SECRETS_DIR:-${RUNNER_TEMP:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/.ios-secrets}}"
mkdir -p "$WORKDIR"
chmod 700 "$WORKDIR"
export VAULT_ADDR="${VAULT_ADDR:-}" VAULT_TOKEN="${VAULT_TOKEN:-}" VAULT_TOKEN_FILE WORKDIR VAULT_SECRET_PATH

python3 - <<'PY'
import base64, json, os, sys, urllib.request, urllib.error

def resolve_auth():
    addr = os.environ.get("VAULT_ADDR") or ""
    token = os.environ.get("VAULT_TOKEN") or ""
    path = os.environ.get("VAULT_TOKEN_FILE") or ""
    if (not addr or not token) and path and os.path.isfile(path):
        with open(path) as fh:
            data = json.load(fh)
        if not isinstance(data, dict):
            sys.exit("ERROR: %s is not a JSON object" % path)

        def pick(keys):
            for key in keys:
                if data.get(key):
                    return data[key]
            return ""

        addr = addr or pick(("VAULT_ADDR", "VAULT_URI", "vault_addr", "addr", "url"))
        token = token or pick(("VAULT_TOKEN", "vault_token", "token", "client_token"))
        if not token and isinstance(data.get("auth"), dict):
            token = data["auth"].get("client_token") or ""
    if not addr or not token:
        sys.exit("ERROR: Vault auth unavailable — set VAULT_ADDR/VAULT_TOKEN or %s"
                 % (path or "$VAULT_TOKEN_FILE"))
    return addr.rstrip("/"), token

addr, token = resolve_auth()
workdir = os.environ["WORKDIR"]
secret_path = os.environ.get("VAULT_SECRET_PATH", "secret/kubepilot/ios")
if not secret_path.startswith("secret/data/"):
    secret_path = "secret/data/" + secret_path.removeprefix("secret/")
url = addr + "/v1/" + secret_path

req = urllib.request.Request(url, method="GET")
req.add_header("X-Vault-Token", token)
try:
    with urllib.request.urlopen(req, timeout=30) as resp:
        raw = resp.read()
except urllib.error.HTTPError as exc:
    detail = exc.read().decode("utf-8", "replace")[:500]
    sys.exit("ERROR: Vault GET %s -> HTTP %s: %s" % (url, exc.code, detail))
except urllib.error.URLError as exc:
    sys.exit("ERROR: Vault GET %s -> %s" % (url, exc))

try:
    payload = json.loads(raw)
except ValueError:
    sys.exit("ERROR: Vault response from %s is not JSON" % url)

data = (payload.get("data") or {}).get("data")
if not isinstance(data, dict):
    sys.exit("ERROR: unexpected Vault response at %s (no .data.data map)" % url)

# fastlane provisions the distribution certificate from the ASC API key, so only
# the ASC key is required. CERT_PRIVATE_KEY_B64 is accepted but optional.
required = ["ASC_KEY_ID", "ASC_ISSUER_ID", "ASC_PRIVATE_KEY_B64"]
missing = [k for k in required if not data.get(k)]
if missing:
    sys.exit("ERROR: %s is missing required key(s): %s" % (secret_path, ", ".join(missing)))

def write_secret_file(name, b64_value):
    path = os.path.join(os.environ["WORKDIR"], name)
    raw = base64.b64decode(b64_value, validate=True)
    with open(path, "wb") as fh:
        fh.write(raw)
    os.chmod(path, 0o600)
    return path

asc_key_path = write_secret_file("asc_api_key.p8", data["ASC_PRIVATE_KEY_B64"])

env_lines = [
    ("ASC_KEY_ID", data["ASC_KEY_ID"]),
    ("ASC_ISSUER_ID", data["ASC_ISSUER_ID"]),
    ("ASC_KEY_FILEPATH", asc_key_path),
]
if data.get("CERT_PRIVATE_KEY_B64"):
    env_lines.append(("DIST_CERT_KEY_FILEPATH",
                      write_secret_file("dist_cert_key.pem", data["CERT_PRIVATE_KEY_B64"])))
if data.get("APPLE_TEAM_ID"):
    env_lines.append(("APPLE_TEAM_ID", data["APPLE_TEAM_ID"]))
if data.get("APP_STORE_APP_ID"):
    env_lines.append(("APP_STORE_APP_ID", data["APP_STORE_APP_ID"]))

github_env = os.environ.get("GITHUB_ENV")
if github_env:
    with open(github_env, "a") as fh:
        for key, value in env_lines:
            fh.write("%s=%s\n" % (key, value))
else:
    for key, value in env_lines:
        sys.stdout.write("export %s=%s\n" % (key, value))

sys.stderr.write("OK: loaded iOS signing material from %s into %s\n"
                 % (secret_path, os.environ["WORKDIR"]))
PY
