#!/usr/bin/env bash
# Upsert Cloudflare CNAME demo.kubepilot.org → lon1.pop0.uk (DNS-only / grey-cloud).
#
# Requires: CLOUDFLARE_API_TOKEN with Zone.DNS edit on kubepilot.org
# Optional: CLOUDFLARE_ZONE_ID (looked up from CLOUDFLARE_ZONE if unset)
set -euo pipefail

ZONE="${CLOUDFLARE_ZONE:-kubepilot.org}"
NAME="${DEMO_DNS_NAME:-demo}"
FQDN="${NAME}.${ZONE}"
TARGET="${CNAME_TARGET:-lon1.pop0.uk}"
API="https://api.cloudflare.com/client/v4"

if [[ -z "${CLOUDFLARE_API_TOKEN:-}" ]]; then
  echo "CLOUDFLARE_API_TOKEN is required" >&2
  exit 1
fi

auth=(-H "Authorization: Bearer ${CLOUDFLARE_API_TOKEN}" -H "Content-Type: application/json")

ZONE_ID="${CLOUDFLARE_ZONE_ID:-}"
if [[ -z "$ZONE_ID" ]]; then
  ZONE_ID=$(curl -fsS "${auth[@]}" \
    "${API}/zones?name=${ZONE}" | python3 -c 'import json,sys; d=json.load(sys.stdin); print(d["result"][0]["id"])')
fi

EXISTING=$(curl -fsS "${auth[@]}" \
  "${API}/zones/${ZONE_ID}/dns_records?type=CNAME&name=${FQDN}" \
  | python3 -c 'import json,sys; d=json.load(sys.stdin); print(d["result"][0]["id"] if d["result"] else "")')

BODY=$(python3 - <<PY
import json
print(json.dumps({
  "type": "CNAME",
  "name": "${NAME}",
  "content": "${TARGET}",
  "ttl": 1,
  "proxied": False,
}))
PY
)

if [[ -n "$EXISTING" ]]; then
  curl -fsS -X PUT "${auth[@]}" \
    --data "$BODY" \
    "${API}/zones/${ZONE_ID}/dns_records/${EXISTING}" >/dev/null
  echo "Updated CNAME ${FQDN} → ${TARGET} (DNS-only)"
else
  curl -fsS -X POST "${auth[@]}" \
    --data "$BODY" \
    "${API}/zones/${ZONE_ID}/dns_records" >/dev/null
  echo "Created CNAME ${FQDN} → ${TARGET} (DNS-only)"
fi
