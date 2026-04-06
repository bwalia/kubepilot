#!/usr/bin/env bash
# =============================================================================
# setup-longhorn.sh — Automated Longhorn + Nexus setup for k3s clusters
# =============================================================================
# Lessons learned from real incidents:
#   1. BestEffort QoS pods (no resource limits) get evicted under pressure
#   2. Longhorn volume expansion during attach causes faulted engine loops
#   3. Volume attachment conflicts when pods reschedule across nodes
#   4. Missing resource limits cause unpredictable memory usage (Nexus: 6.8GB)
#   5. Stale remountRequestedAt flags cause infinite attach/detach cycles
# =============================================================================

set -euo pipefail

# ---------------------------------------------------------------------------
# Defaults (override via env or flags)
# ---------------------------------------------------------------------------
KUBECONFIG="${KUBECONFIG:-$HOME/.kube/k3s0.yaml}"
LONGHORN_VERSION="${LONGHORN_VERSION:-1.10.1}"
LONGHORN_NAMESPACE="${LONGHORN_NAMESPACE:-longhorn-system}"
NEXUS_NAMESPACE="${NEXUS_NAMESPACE:-nexus}"
NEXUS_CHART_VERSION="${NEXUS_CHART_VERSION:-64.2.0}"
NEXUS_STORAGE_SIZE="${NEXUS_STORAGE_SIZE:-500Gi}"
NEXUS_MEMORY_REQUEST="${NEXUS_MEMORY_REQUEST:-4Gi}"
NEXUS_MEMORY_LIMIT="${NEXUS_MEMORY_LIMIT:-8Gi}"
NEXUS_CPU_REQUEST="${NEXUS_CPU_REQUEST:-500m}"
NEXUS_CPU_LIMIT="${NEXUS_CPU_LIMIT:-2}"
NEXUS_JVM_HEAP="${NEXUS_JVM_HEAP:-2g}"
NEXUS_JVM_DIRECT="${NEXUS_JVM_DIRECT:-3g}"
NEXUS_DOMAIN="${NEXUS_DOMAIN:-dev-nexus.workstation.co.uk}"
REGISTRY_DOMAIN="${REGISTRY_DOMAIN:-dev-registry.workstation.co.uk}"
INGRESS_CLASS="${INGRESS_CLASS:-wslproxy}"
REPLICA_COUNT="${REPLICA_COUNT:-3}"
DRY_RUN="${DRY_RUN:-false}"
SKIP_LONGHORN="${SKIP_LONGHORN:-false}"
SKIP_NEXUS="${SKIP_NEXUS:-false}"

# ---------------------------------------------------------------------------
# Colors & helpers
# ---------------------------------------------------------------------------
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
BLUE='\033[0;34m'; NC='\033[0m'

log()  { printf "${GREEN}[OK]${NC}    %s\n" "$*"; }
warn() { printf "${YELLOW}[WARN]${NC}  %s\n" "$*"; }
err()  { printf "${RED}[ERR]${NC}   %s\n" "$*" >&2; }
info() { printf "${BLUE}[INFO]${NC}  %s\n" "$*"; }
step() { printf "\n${BLUE}==> %s${NC}\n" "$*"; }

die() { err "$@"; exit 1; }

kc() { kubectl --kubeconfig="$KUBECONFIG" "$@"; }

dry() {
  if [[ "$DRY_RUN" == "true" ]]; then
    info "[DRY-RUN] $*"
    return 0
  fi
  return 1
}

# ---------------------------------------------------------------------------
# Usage
# ---------------------------------------------------------------------------
usage() {
  cat <<'EOF'
Usage: setup-longhorn.sh [OPTIONS]

Options:
  --kubeconfig PATH        Path to kubeconfig (default: ~/.kube/k3s0.yaml)
  --longhorn-version VER   Longhorn chart version (default: 1.10.1)
  --nexus-storage SIZE     Nexus PVC size (default: 500Gi)
  --nexus-domain DOMAIN    Nexus UI domain (default: dev-nexus.workstation.co.uk)
  --registry-domain DOMAIN Docker registry domain (default: dev-registry.workstation.co.uk)
  --ingress-class CLASS    Ingress class name (default: wslproxy)
  --replica-count N        Longhorn replica count (default: 3)
  --skip-longhorn          Skip Longhorn install/upgrade
  --skip-nexus             Skip Nexus install/upgrade
  --dry-run                Show what would be done without making changes
  -h, --help               Show this help

Environment variables:
  KUBECONFIG, LONGHORN_VERSION, NEXUS_NAMESPACE, NEXUS_STORAGE_SIZE,
  NEXUS_MEMORY_REQUEST, NEXUS_MEMORY_LIMIT, NEXUS_CPU_REQUEST, NEXUS_CPU_LIMIT,
  NEXUS_DOMAIN, REGISTRY_DOMAIN, INGRESS_CLASS, REPLICA_COUNT, DRY_RUN
EOF
  exit 0
}

# ---------------------------------------------------------------------------
# Parse flags
# ---------------------------------------------------------------------------
while [[ $# -gt 0 ]]; do
  case "$1" in
    --kubeconfig)        KUBECONFIG="$2"; shift 2 ;;
    --longhorn-version)  LONGHORN_VERSION="$2"; shift 2 ;;
    --nexus-storage)     NEXUS_STORAGE_SIZE="$2"; shift 2 ;;
    --nexus-domain)      NEXUS_DOMAIN="$2"; shift 2 ;;
    --registry-domain)   REGISTRY_DOMAIN="$2"; shift 2 ;;
    --ingress-class)     INGRESS_CLASS="$2"; shift 2 ;;
    --replica-count)     REPLICA_COUNT="$2"; shift 2 ;;
    --skip-longhorn)     SKIP_LONGHORN=true; shift ;;
    --skip-nexus)        SKIP_NEXUS=true; shift ;;
    --dry-run)           DRY_RUN=true; shift ;;
    -h|--help)           usage ;;
    *) die "Unknown flag: $1" ;;
  esac
done

# ---------------------------------------------------------------------------
# Pre-flight checks
# ---------------------------------------------------------------------------
step "Pre-flight checks"

[[ -f "$KUBECONFIG" ]] || die "Kubeconfig not found: $KUBECONFIG"
command -v kubectl >/dev/null || die "kubectl not found"
command -v helm >/dev/null    || die "helm not found"

# Test cluster connectivity
if ! kc cluster-info >/dev/null 2>&1; then
  die "Cannot connect to cluster with kubeconfig: $KUBECONFIG"
fi
log "Cluster reachable"

NODE_COUNT=$(kc get nodes --no-headers 2>/dev/null | wc -l | tr -d ' ')
info "Cluster has $NODE_COUNT nodes"

if [[ "$DRY_RUN" == "true" ]]; then
  warn "DRY-RUN mode — no changes will be made"
fi

# ---------------------------------------------------------------------------
# Install / Upgrade Longhorn
# ---------------------------------------------------------------------------
install_longhorn() {
  step "Installing/Upgrading Longhorn v${LONGHORN_VERSION}"

  if dry "helm upgrade --install longhorn ..."; then return; fi

  # Add Longhorn Helm repo
  helm repo add longhorn https://charts.longhorn.io 2>/dev/null || true
  helm repo update longhorn

  kc create namespace "$LONGHORN_NAMESPACE" --dry-run=client -o yaml | kc apply -f -

  helm upgrade --install longhorn longhorn/longhorn \
    --namespace "$LONGHORN_NAMESPACE" \
    --version "$LONGHORN_VERSION" \
    --set defaultSettings.defaultReplicaCount="$REPLICA_COUNT" \
    --set defaultSettings.storageOverProvisioningPercentage=200 \
    --set defaultSettings.storageMinimalAvailablePercentage=15 \
    --set defaultSettings.defaultDataLocality=best-effort \
    --set defaultSettings.replicaAutoBalance=best-effort \
    --set defaultSettings.nodeDownPodDeletionPolicy=delete-both-statefulset-and-deployment-pod \
    --set defaultSettings.volumeAttachmentRecoveryPolicy=wait \
    --set persistence.defaultClassReplicaCount="$REPLICA_COUNT" \
    --set csi.kubeletRootDir=/var/lib/kubelet \
    --wait \
    --timeout 10m

  log "Longhorn installed/upgraded"
}

# ---------------------------------------------------------------------------
# Wait for Longhorn readiness
# ---------------------------------------------------------------------------
wait_longhorn_ready() {
  step "Waiting for Longhorn components"

  if dry "wait for longhorn pods"; then return; fi

  info "Waiting for Longhorn manager pods..."
  kc -n "$LONGHORN_NAMESPACE" rollout status daemonset/longhorn-manager --timeout=300s

  info "Waiting for Longhorn driver deployer..."
  kc -n "$LONGHORN_NAMESPACE" rollout status deployment/longhorn-driver-deployer --timeout=300s

  # Wait for CSI pods
  local retries=30
  while [[ $retries -gt 0 ]]; do
    local not_ready
    not_ready=$(kc -n "$LONGHORN_NAMESPACE" get pods --no-headers 2>/dev/null \
      | grep -v "Running\|Completed" | wc -l | tr -d ' ')
    if [[ "$not_ready" -eq 0 ]]; then
      log "All Longhorn pods are running"
      return 0
    fi
    info "Waiting for $not_ready pods... ($retries retries left)"
    sleep 10
    retries=$((retries - 1))
  done

  warn "Some Longhorn pods may not be ready yet"
}

# ---------------------------------------------------------------------------
# Create optimised StorageClass
# ---------------------------------------------------------------------------
create_storage_class() {
  step "Creating StorageClass"

  if dry "apply storageclass longhorn"; then return; fi

  # StorageClass with volume expansion enabled and best-effort data locality
  # WHY best-effort dataLocality: reduces cross-node I/O, prevents attachment
  # conflicts seen when engine runs on a different node than replicas
  # WHY allowVolumeExpansion: avoids needing to recreate PVCs for more space
  cat <<YAML | kc apply -f -
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: longhorn
  annotations:
    storageclass.kubernetes.io/is-default-class: "true"
provisioner: driver.longhorn.io
allowVolumeExpansion: true
reclaimPolicy: Delete
volumeBindingMode: Immediate
parameters:
  numberOfReplicas: "${REPLICA_COUNT}"
  staleReplicaTimeout: "30"
  dataLocality: "best-effort"
  fromBackup: ""
  fsType: "ext4"
YAML

  log "StorageClass configured"
}

# ---------------------------------------------------------------------------
# Install / Upgrade Nexus
# ---------------------------------------------------------------------------
install_nexus() {
  step "Installing/Upgrading Nexus Repository Manager"

  if dry "helm upgrade --install nexus ..."; then return; fi

  # Add Sonatype Helm repo
  helm repo add sonatype https://sonatype.github.io/helm3-charts/ 2>/dev/null || true
  helm repo update sonatype

  kc create namespace "$NEXUS_NAMESPACE" --dry-run=client -o yaml | kc apply -f -

  # WHY explicit resource limits:
  #   Without limits, Nexus runs as BestEffort QoS and gets evicted first
  #   under memory pressure. Nexus JVM uses 2g heap + 3g direct memory,
  #   plus ~2g for OS/container overhead = ~7GB total. Setting request=4Gi
  #   and limit=8Gi ensures Burstable QoS with room to breathe.
  #
  # WHY Recreate strategy:
  #   Nexus uses a single RWO PVC — rolling update would fail because the
  #   new pod can't attach the volume while the old pod holds it.
  helm upgrade --install nexus sonatype/nexus-repository-manager \
    --namespace "$NEXUS_NAMESPACE" \
    --version "$NEXUS_CHART_VERSION" \
    --set nexus.env[0].name=NEXUS_SECURITY_RANDOMPASSWORD \
    --set nexus.env[0].value="false" \
    --set nexus.env[1].name=INSTALL4J_ADD_VM_PARAMS \
    --set nexus.env[1].value="-Xms${NEXUS_JVM_HEAP} -Xmx${NEXUS_JVM_HEAP} -XX:MaxDirectMemorySize=${NEXUS_JVM_DIRECT}" \
    --set nexus.resources.requests.cpu="${NEXUS_CPU_REQUEST}" \
    --set nexus.resources.requests.memory="${NEXUS_MEMORY_REQUEST}" \
    --set nexus.resources.limits.cpu="${NEXUS_CPU_LIMIT}" \
    --set nexus.resources.limits.memory="${NEXUS_MEMORY_LIMIT}" \
    --set persistence.enabled=true \
    --set persistence.storageClass=longhorn \
    --set persistence.accessMode=ReadWriteOnce \
    --set persistence.size="${NEXUS_STORAGE_SIZE}" \
    --set ingress.enabled=true \
    --set ingress.ingressClassName="${INGRESS_CLASS}" \
    --set ingress.hostRepo="${REGISTRY_DOMAIN}" \
    --set ingress.hostPath="${NEXUS_DOMAIN}" \
    --set service.type=ClusterIP \
    --timeout 10m

  log "Nexus installed/upgraded"
}

# ---------------------------------------------------------------------------
# Configure Nexus Ingress (UI + Docker registry)
# ---------------------------------------------------------------------------
configure_nexus_ingress() {
  step "Configuring Nexus Ingress"

  if dry "apply nexus ingress resources"; then return; fi

  # Nexus UI ingress
  cat <<YAML | kc apply -f -
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: nexus-nexus-repository-manager
  namespace: ${NEXUS_NAMESPACE}
  annotations:
    nginx.ingress.kubernetes.io/proxy-body-size: "0"
spec:
  ingressClassName: ${INGRESS_CLASS}
  rules:
  - host: ${NEXUS_DOMAIN}
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: nexus-nexus-repository-manager
            port:
              number: 8081
  tls:
  - hosts:
    - ${NEXUS_DOMAIN}
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: nexus-nexus-repository-manager-docker-5000
  namespace: ${NEXUS_NAMESPACE}
  annotations:
    nginx.ingress.kubernetes.io/proxy-body-size: "0"
spec:
  ingressClassName: ${INGRESS_CLASS}
  rules:
  - host: ${REGISTRY_DOMAIN}
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: nexus-nexus-repository-manager-docker-5000
            port:
              number: 5000
  tls:
  - hosts:
    - ${REGISTRY_DOMAIN}
YAML

  log "Ingress configured for ${NEXUS_DOMAIN} and ${REGISTRY_DOMAIN}"
}

# ---------------------------------------------------------------------------
# Wait for Nexus readiness
# ---------------------------------------------------------------------------
wait_nexus_ready() {
  step "Waiting for Nexus to become ready"

  if dry "wait for nexus pod"; then return; fi

  local retries=30
  while [[ $retries -gt 0 ]]; do
    local ready
    ready=$(kc -n "$NEXUS_NAMESPACE" get pods -l app.kubernetes.io/name=nexus-repository-manager \
      -o jsonpath='{.items[0].status.conditions[?(@.type=="Ready")].status}' 2>/dev/null || echo "False")
    if [[ "$ready" == "True" ]]; then
      log "Nexus is ready"
      return 0
    fi
    info "Nexus not ready yet... ($retries retries left)"
    sleep 15
    retries=$((retries - 1))
  done

  warn "Nexus did not become ready in time — check pod logs"
  kc -n "$NEXUS_NAMESPACE" get pods -o wide
  return 1
}

# ---------------------------------------------------------------------------
# Health validation
# ---------------------------------------------------------------------------
validate() {
  step "Validation"

  local errors=0

  # Check Longhorn volumes
  if [[ "$SKIP_LONGHORN" != "true" ]]; then
    local faulted
    faulted=$(kc get volumes.longhorn.io -n "$LONGHORN_NAMESPACE" --no-headers 2>/dev/null \
      | grep -c "faulted" || true)
    if [[ "$faulted" -gt 0 ]]; then
      warn "$faulted Longhorn volume(s) in faulted state"
      errors=$((errors + 1))
    else
      log "No faulted Longhorn volumes"
    fi
  fi

  # Check Nexus pod
  if [[ "$SKIP_NEXUS" != "true" ]]; then
    local nexus_ready
    nexus_ready=$(kc -n "$NEXUS_NAMESPACE" get pods -l app.kubernetes.io/name=nexus-repository-manager \
      --no-headers 2>/dev/null | grep -c "1/1" || true)
    if [[ "$nexus_ready" -eq 0 ]]; then
      warn "Nexus pod is not ready"
      errors=$((errors + 1))
    else
      log "Nexus pod is ready"
    fi

    # Check QoS class — must NOT be BestEffort
    local qos
    qos=$(kc -n "$NEXUS_NAMESPACE" get pods -l app.kubernetes.io/name=nexus-repository-manager \
      -o jsonpath='{.items[0].status.qosClass}' 2>/dev/null || echo "Unknown")
    if [[ "$qos" == "BestEffort" ]]; then
      err "Nexus QoS is BestEffort — resource limits are missing!"
      errors=$((errors + 1))
    else
      log "Nexus QoS: $qos"
    fi

    # Check PVC
    local pvc_status
    pvc_status=$(kc -n "$NEXUS_NAMESPACE" get pvc -l app.kubernetes.io/name=nexus-repository-manager \
      -o jsonpath='{.items[0].status.phase}' 2>/dev/null || echo "Unknown")
    if [[ "$pvc_status" != "Bound" ]]; then
      warn "Nexus PVC status: $pvc_status (expected Bound)"
      errors=$((errors + 1))
    else
      log "Nexus PVC is Bound"
    fi
  fi

  if [[ "$errors" -gt 0 ]]; then
    warn "$errors validation issue(s) found"
    return 1
  fi

  log "All validations passed"
}

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
summary() {
  step "Summary"
  echo ""
  info "Longhorn:  v${LONGHORN_VERSION} (${REPLICA_COUNT} replicas)"
  info "Nexus:     ${NEXUS_NAMESPACE} namespace"
  info "Storage:   ${NEXUS_STORAGE_SIZE} (longhorn StorageClass)"
  info "Resources: ${NEXUS_CPU_REQUEST}-${NEXUS_CPU_LIMIT} CPU, ${NEXUS_MEMORY_REQUEST}-${NEXUS_MEMORY_LIMIT} memory"
  info "JVM:       -Xmx${NEXUS_JVM_HEAP} -XX:MaxDirectMemorySize=${NEXUS_JVM_DIRECT}"
  info "Ingress:   ${NEXUS_DOMAIN} (UI), ${REGISTRY_DOMAIN} (Docker)"
  echo ""
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
main() {
  step "Longhorn + Nexus Setup"
  info "KUBECONFIG: $KUBECONFIG"

  if [[ "$SKIP_LONGHORN" != "true" ]]; then
    install_longhorn
    wait_longhorn_ready
    create_storage_class
  else
    info "Skipping Longhorn install (--skip-longhorn)"
  fi

  if [[ "$SKIP_NEXUS" != "true" ]]; then
    install_nexus
    configure_nexus_ingress
    wait_nexus_ready
  else
    info "Skipping Nexus install (--skip-nexus)"
  fi

  validate
  summary
  log "Setup complete"
}

main
