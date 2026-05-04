#!/usr/bin/env sh
set -eu

NAMESPACE="${NAMESPACE:-woms}"
RELEASE="${RELEASE:-woms}"
CHART="${CHART:-./deploy/helm/woms}"
INGRESS_ENABLED="${INGRESS_ENABLED:-false}"

helm template "$RELEASE" "$CHART" --set "ingress.enabled=${INGRESS_ENABLED}" >/tmp/woms-rendered.yaml
grep -q "kind: ScaledObject" /tmp/woms-rendered.yaml
if [ "$INGRESS_ENABLED" = "true" ]; then
  grep -q "kind: Ingress" /tmp/woms-rendered.yaml
fi
grep -q "kind: PodDisruptionBudget" /tmp/woms-rendered.yaml
grep -q "name: ${RELEASE}-woms-worker" /tmp/woms-rendered.yaml
grep -q "name: ${RELEASE}-woms-worker-hpa" /tmp/woms-rendered.yaml

kubectl get namespace "$NAMESPACE" >/dev/null
kubectl get scaledobject "$RELEASE-woms-worker" -n "$NAMESPACE"
kubectl get hpa "$RELEASE-woms-worker-hpa" -n "$NAMESPACE"
kubectl get poddisruptionbudget "$RELEASE-woms-api" "$RELEASE-woms-web" -n "$NAMESPACE"
kubectl get deploy -n "$NAMESPACE"
kubectl describe scaledobject "$RELEASE-woms-worker" -n "$NAMESPACE"

echo "Kubernetes static and resource verification passed"
