#!/usr/bin/env sh
set -eu

RELEASE="${RELEASE:-woms}"
CHART="${CHART:-./deploy/helm/woms}"

rendered="$(mktemp)"
trap 'rm -f "$rendered"' EXIT

helm template "$RELEASE" "$CHART" >"$rendered"

grep -q "kind: ScaledObject" "$rendered"
grep -q "name: ${RELEASE}-woms-worker-hpa" "$rendered"
grep -q "horizontalPodAutoscalerConfig:" "$rendered"
grep -q "scaleTargetRef:" "$rendered"
grep -q "name: ${RELEASE}-woms-worker" "$rendered"
grep -q "minReplicaCount: 1" "$rendered"
grep -q "maxReplicaCount: 10" "$rendered"
grep -q "type: kafka" "$rendered"
grep -q 'topic: "woms.schedule.jobs"' "$rendered"
grep -q 'consumerGroup: "woms-scheduler-workers"' "$rendered"
grep -q 'lagThreshold: "10"' "$rendered"
grep -q "type: cpu" "$rendered"
grep -q "metricType: Utilization" "$rendered"
grep -q 'value: "70"' "$rendered"
grep -q "scaleUp:" "$rendered"
grep -q "stabilizationWindowSeconds: 0" "$rendered"
grep -q "scaleDown:" "$rendered"
grep -q "stabilizationWindowSeconds: 120" "$rendered"
grep -q "kind: PodDisruptionBudget" "$rendered"
grep -q "name: ${RELEASE}-woms-api" "$rendered"
grep -q "name: ${RELEASE}-woms-web" "$rendered"
grep -q "minAvailable: 1" "$rendered"

echo "HPA/KEDA render verification passed"
