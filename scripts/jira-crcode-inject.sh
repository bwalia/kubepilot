#!/usr/bin/env bash
# ─────────────────────────────────────────────────────────────────────────────
# KubePilot – Jira CR Code Injection Script
#
# Use this script (or adapt it for your CI/CD pipeline) to register a
# production CR code into Kubernetes Secrets when a Jira change request
# is approved. Called from Jira Automation, GitHub Actions, or Argo Workflows.
#
# Usage:
#   export JIRA_ISSUE_KEY="INFRA-1234"
#   export CR_CODE="$(openssl rand -hex 16)"  # or use your change management system
#   export CR_EXPIRES_IN_HOURS=4
#   ./scripts/jira-crcode-inject.sh
# ─────────────────────────────────────────────────────────────────────────────
set -euo pipefail

CHANGE_ID="${JIRA_ISSUE_KEY:?JIRA_ISSUE_KEY must be set}"
CR_CODE="${CR_CODE:?CR_CODE must be set}"
NAMESPACE="kubepilot-security"
SECRET_NAME="kubepilot-crcode-${CHANGE_ID}"
EXPIRES_IN_HOURS="${CR_EXPIRES_IN_HOURS:-4}"

echo "▸ Registering CR code for change: ${CHANGE_ID}"
echo "▸ Secret name: ${SECRET_NAME}"
echo "▸ Expires in: ${EXPIRES_IN_HOURS}h"

# Calculate expiry timestamp.
EXPIRES_AT=$(date -u -d "+${EXPIRES_IN_HOURS} hours" '+%Y-%m-%dT%H:%M:%SZ' 2>/dev/null || \
             date -u -v "+${EXPIRES_IN_HOURS}H" '+%Y-%m-%dT%H:%M:%SZ')

# Create or update the Kubernetes secret.
# The CR code value is injected as the 'cr-code' key.
kubectl create secret generic "${SECRET_NAME}" \
  --namespace="${NAMESPACE}" \
  --from-literal="cr-code=${CR_CODE}" \
  --from-literal="expires-at=${EXPIRES_AT}" \
  --dry-run=client -o yaml | kubectl apply -f -

# Label the secret for auditing and KubePilot discovery.
kubectl label secret "${SECRET_NAME}" \
  --namespace="${NAMESPACE}" \
  "app.kubernetes.io/managed-by=kubepilot" \
  "kubepilot.io/change-id=${CHANGE_ID}" \
  --overwrite

echo "✓ CR code registered successfully for ${CHANGE_ID} (expires: ${EXPIRES_AT})"
echo ""
echo "  To authorize a production job from the dashboard:"
echo "  1. Open http://localhost:8080"
echo "  2. Create a new job with target_environment=production"
echo "  3. Enter Change ID: ${CHANGE_ID}"
echo "  4. Enter the CR code when prompted"
