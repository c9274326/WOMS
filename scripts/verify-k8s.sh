#!/usr/bin/env sh
set -eu

NAMESPACE="${NAMESPACE:-woms}"
RELEASE="${RELEASE:-woms}"
CHART="${CHART:-./deploy/helm/woms}"
INGRESS_ENABLED="${INGRESS_ENABLED:-false}"

require_rendered_pdb() {
  name="$1"
  awk -v name="$name" '
    /^kind: PodDisruptionBudget$/ { in_pdb = 1; next }
    /^---$/ { in_pdb = 0 }
    in_pdb && $0 ~ "name: " name "$" { found = 1 }
    END { exit found ? 0 : 1 }
  ' /tmp/woms-rendered.yaml
}

helm template "$RELEASE" "$CHART" --dependency-update --set "ingress.enabled=${INGRESS_ENABLED}" >/tmp/woms-rendered.yaml
grep -q "kind: ScaledObject" /tmp/woms-rendered.yaml
if [ "$INGRESS_ENABLED" = "true" ]; then
  grep -q "kind: Ingress" /tmp/woms-rendered.yaml
fi
grep -q "name: ${RELEASE}-woms-worker" /tmp/woms-rendered.yaml
grep -q "name: ${RELEASE}-woms-worker-hpa" /tmp/woms-rendered.yaml
require_rendered_pdb "${RELEASE}-woms-api"
require_rendered_pdb "${RELEASE}-woms-web"

kubectl get namespace "$NAMESPACE" >/dev/null
kubectl get scaledobject "$RELEASE-woms-worker" -n "$NAMESPACE"
kubectl get hpa "$RELEASE-woms-worker-hpa" -n "$NAMESPACE"
kubectl get poddisruptionbudget "$RELEASE-woms-api" "$RELEASE-woms-web" -n "$NAMESPACE"
kubectl get deploy -n "$NAMESPACE"
kubectl describe scaledobject "$RELEASE-woms-worker" -n "$NAMESPACE"

echo "Kubernetes static and resource verification passed"
