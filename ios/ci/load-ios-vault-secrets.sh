#!/usr/bin/env bash
#
# load-ios-vault-secrets.sh — pull KubePilot iOS signing material from wslvault.
#
# Reads the flat KV v2 secret `kv/kubepilot/ios` from wslvault
# (https://vault.workstation.co.uk; override the path with KUBEPILOT_IOS_VAULT_PATH),
# materialises the ASC API key and distribution-certificate private key as
# files, and exports identifiers for fastlane.
#
# Auth, first match wins:
#   WSLVAULT_TOKEN          a wslvault JWT
#   WSLVAULT_API_KEY        a wslvault API key, exchanged for a JWT
#   WSLVAULT_API_KEY_FILE   file holding that key (default ~/.secrets/kubepilot/wslvault-ci-key)
# CI cannot answer an MFA challenge, so the runner's key must be a machine key
# issued without mfa_required.
#
#   CI:    ./ci/load-ios-vault-secrets.sh           # appends to $GITHUB_ENV
#   local: eval "$(./ci/load-ios-vault-secrets.sh)" # exports to shell
#
set -euo pipefail

WSLVAULT_ADDR="${WSLVAULT_ADDR:-https://vault.workstation.co.uk}"
WSLVAULT_API_KEY_FILE="${WSLVAULT_API_KEY_FILE:-$HOME/.secrets/kubepilot/wslvault-ci-key}"
VAULT_SECRET_PATH="${KUBEPILOT_IOS_VAULT_PATH:-kubepilot/ios}"
WORKDIR="${IOS_SECRETS_DIR:-${RUNNER_TEMP:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/.ios-secrets}}"
mkdir -p "$WORKDIR"
chmod 700 "$WORKDIR"
export WSLVAULT_ADDR WSLVAULT_TOKEN="${WSLVAULT_TOKEN:-}" WSLVAULT_API_KEY="${WSLVAULT_API_KEY:-}" \
  WSLVAULT_API_KEY_FILE WORKDIR VAULT_SECRET_PATH

python3 - <<'PY'
import base64, json, os, sys, urllib.request, urllib.error

addr = os.environ["WSLVAULT_ADDR"].rstrip("/")

def post_json(path, body):
    req = urllib.request.Request(addr + path, data=json.dumps(body).encode(), method="POST")
    req.add_header("Content-Type", "application/json")
    try:
        with urllib.request.urlopen(req, timeout=30) as resp:
            return json.loads(resp.read())
    except urllib.error.HTTPError as exc:
        sys.exit("ERROR: wslvault POST %s -> HTTP %s" % (path, exc.code))
    except urllib.error.URLError as exc:
        sys.exit("ERROR: wslvault POST %s -> %s" % (path, exc))

def resolve_token():
    token = os.environ.get("WSLVAULT_TOKEN") or ""
    if token:
        return token
    key = os.environ.get("WSLVAULT_API_KEY") or ""
    key_file = os.environ.get("WSLVAULT_API_KEY_FILE") or ""
    if not key and key_file and os.path.isfile(key_file):
        with open(key_file) as fh:
            key = fh.read().strip()
    if not key:
        sys.exit("ERROR: wslvault auth unavailable — set WSLVAULT_TOKEN, WSLVAULT_API_KEY, "
                 "or put a machine API key in %s" % (key_file or "$WSLVAULT_API_KEY_FILE"))
    resp = post_json("/v1/auth/api-key", {"api_key": key})
    if resp.get("mfa_required"):
        sys.exit("ERROR: this wslvault API key requires MFA, which CI cannot answer — "
                 "issue a machine key without mfa_required for the runner")
    if not resp.get("token"):
        sys.exit("ERROR: wslvault returned no token for the API key")
    return resp["token"]

token = resolve_token()
workdir = os.environ["WORKDIR"]
secret_path = os.environ.get("VAULT_SECRET_PATH", "kubepilot/ios")
secret_path = "kv/data/" + secret_path.removeprefix("kv/data/").removeprefix("kv/")
url = addr + "/v1/" + secret_path

req = urllib.request.Request(url, method="GET")
req.add_header("X-Vault-Token", token)
try:
    with urllib.request.urlopen(req, timeout=30) as resp:
        raw = resp.read()
except urllib.error.HTTPError as exc:
    detail = exc.read().decode("utf-8", "replace")[:500]
    sys.exit("ERROR: wslvault GET %s -> HTTP %s: %s" % (url, exc.code, detail))
except urllib.error.URLError as exc:
    sys.exit("ERROR: wslvault GET %s -> %s" % (url, exc))

try:
    payload = json.loads(raw)
except ValueError:
    sys.exit("ERROR: wslvault response from %s is not JSON" % url)

data = (payload.get("data") or {}).get("data")
if not isinstance(data, dict):
    sys.exit("ERROR: unexpected wslvault response at %s (no .data.data map)" % url)

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

sys.stderr.write("OK: loaded iOS signing material from wslvault %s into %s\n"
                 % (secret_path, os.environ["WORKDIR"]))
PY
