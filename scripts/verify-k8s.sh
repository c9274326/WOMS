#!/usr/bin/env sh
set -eu

NAMESPACE="${NAMESPACE:-woms}"
RELEASE="${RELEASE:-woms}"
CHART="${CHART:-./deploy/helm/woms}"

helm template "$RELEASE" "$CHART" >/tmp/woms-rendered.yaml
grep -q "kind: ScaledObject" /tmp/woms-rendered.yaml
grep -q "kind: Ingress" /tmp/woms-rendered.yaml
grep -q "name: ${RELEASE}-woms-worker" /tmp/woms-rendered.yaml

kubectl get namespace "$NAMESPACE" >/dev/null
kubectl get scaledobject,hpa -n "$NAMESPACE"
kubectl get deploy -n "$NAMESPACE"

echo "Kubernetes static and resource verification passed"
