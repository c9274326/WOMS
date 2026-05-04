import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";

const values = readFileSync(new URL("./values.yaml", import.meta.url), "utf8");
const scaledObject = readFileSync(new URL("./templates/keda-scaledobject.yaml", import.meta.url), "utf8");
const apiDeployment = readFileSync(new URL("./templates/api-deployment.yaml", import.meta.url), "utf8");
const workerDeployment = readFileSync(new URL("./templates/worker-deployment.yaml", import.meta.url), "utf8");

test("Helm values keep async scheduling and HPA demo defaults wired", () => {
  assert.match(values, /store:\s+postgres/);
  assert.match(values, /databaseUrl:\s+postgres:\/\/woms:woms@postgres:5432\/woms\?sslmode=disable/);
  assert.match(values, /scheduleTopic:\s+woms\.schedule\.jobs/);
  assert.match(values, /kafkaPublishEnabled:\s+"true"/);
  assert.match(values, /minJobDurationMs:\s+"0"/);
  assert.match(values, /maxRetries:\s+"3"/);
  assert.match(values, /consumerGroup:\s+woms-scheduler-workers/);
  assert.match(values, /lagThreshold:\s+"10"/);
  assert.match(values, /targetUtilization:\s+"70"/);
});

test("KEDA ScaledObject template points at scheduler worker backlog", () => {
  assert.match(scaledObject, /kind:\s+ScaledObject/);
  assert.match(scaledObject, /horizontalPodAutoscalerConfig:/);
  assert.match(scaledObject, /name:\s+\{\{ include "woms\.fullname" \. \}\}-worker-hpa/);
  assert.match(scaledObject, /scaleTargetRef:[\s\S]*name:\s+\{\{ include "woms\.fullname" \. \}\}-worker/);
  assert.match(scaledObject, /type:\s+kafka/);
  assert.match(scaledObject, /topic:\s+\{\{ \.Values\.keda\.kafka\.topic \| quote \}\}/);
  assert.match(scaledObject, /consumerGroup:\s+\{\{ \.Values\.keda\.kafka\.consumerGroup \| quote \}\}/);
  assert.match(scaledObject, /lagThreshold:\s+\{\{ \.Values\.keda\.kafka\.lagThreshold \| quote \}\}/);
  assert.match(scaledObject, /type:\s+cpu/);
  assert.match(scaledObject, /metricType:\s+Utilization/);
});

test("API and worker deployments expose PostgreSQL, Kafka, and retry env", () => {
  assert.match(apiDeployment, /name:\s+API_STORE/);
  assert.match(apiDeployment, /name:\s+DATABASE_URL/);
  assert.match(apiDeployment, /name:\s+KAFKA_SCHEDULE_TOPIC/);
  assert.match(apiDeployment, /name:\s+KAFKA_PUBLISH_ENABLED/);
  assert.match(workerDeployment, /name:\s+KAFKA_SCHEDULE_TOPIC/);
  assert.match(workerDeployment, /name:\s+KAFKA_CONSUMER_GROUP/);
  assert.match(workerDeployment, /name:\s+DATABASE_URL/);
  assert.match(workerDeployment, /name:\s+WORKER_MIN_JOB_DURATION_MS/);
  assert.match(workerDeployment, /name:\s+WORKER_MAX_RETRIES/);
});
