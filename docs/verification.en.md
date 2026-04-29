# WOMS Post-Implementation Verification Guide

## 1. Local Static And Unit Tests

```bash
go test ./...
npm run test:web
test -z "$(gofmt -l .)"
```

Expected:

- All Go tests pass.
- Frontend mock tests pass.
- `gofmt` produces no output.

## 2. API/JWT/RBAC Verification

Start the API:

```bash
JWT_SECRET=local-dev-secret go run ./cmd/api
```

Log in as sales:

```bash
curl -s http://localhost:8080/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"sales","password":"demo"}'
```

Check missing token:

```bash
curl -i http://localhost:8080/internal/auth/verify
```

Expected: `401 Unauthorized`.

Check that sales cannot create schedule jobs:

```bash
curl -i http://localhost:8080/api/schedules/jobs \
  -H "Authorization: Bearer <sales-token>" \
  -H 'Content-Type: application/json' \
  -d '{"lineId":"A","startDate":"2026-05-01"}'
```

Expected: `403 Forbidden`.

Check scheduler line isolation:

- Create a line B job as `scheduler-b`.
- Query that job as `scheduler-a`.
- Expected: `403 Forbidden`.

Check calendar behavior:

- Create a schedule job as a scheduler user.
- `GET /api/schedules/calendar?lineId=A&month=2026-05` returns persisted allocations.
- Querying another scheduler's line returns an error.

## 3. Docker Verification

```bash
docker build -f Dockerfile.api -t woms-api:local .
docker build -f Dockerfile.worker -t woms-scheduler-worker:local .
docker build -f Dockerfile.web -t woms-web:local .
docker compose up --build
```

Expected:

- API health: `curl http://localhost:8080/healthz`
- Web: `http://localhost:8081`

## 4. Helm Render Verification

```bash
helm template woms ./deploy/helm/woms
```

Expected output includes:

- `Deployment`: api, worker, web.
- `Ingress`: public, api-secure.
- `ScaledObject`: worker Kafka/CPU triggers.

## 5. Ingress / Gateway Verification

After deployment:

```bash
curl -i https://woms.local/api/orders
curl -i https://woms.local/api/orders -H "Authorization: Bearer <valid-token>"
```

Expected:

- Missing token returns `401`.
- Valid token passes Ingress auth.
- API still performs its own JWT/RBAC checks.
- HTTP redirects to HTTPS.

## 6. KEDA / HPA Verification

Check resources:

```bash
kubectl get scaledobject,hpa -n woms
kubectl describe scaledobject -n woms
```

After sending many Kafka scheduling messages:

```bash
kubectl get deploy -n woms -w
kubectl get hpa -n woms -w
```

Expected:

- Kafka lag increases.
- Worker replicas rise above `minReplicaCount`.
- Replicas scale down after lag drains and cooldown passes.
- If CPU trigger does not work, verify metrics-server and pod resource requests first.

## 7. Redis Lock Verification

Submit two concurrent schedule jobs for the same line:

- Expected: no overlapping schedule version is created.
- One job should wait, retry, or fail cleanly.

Submit jobs for different lines:

- Expected: processing can run in parallel.

## 8. Feature Completion Standard

- Tests pass.
- README zh-TW/en is updated.
- `.gitignore` covers generated/local files.
- Docker/Helm/CI settings are synced.
- `git add`, commit, and push are completed.

## 9. Frontend Smoke Verification

- Login at `http://127.0.0.1:8081`.
- Refresh the browser and confirm the session is restored.
- Toggle exact filters for customer, line, status, and priority.
- Use schedule preview and confirm allocation cards render.
- Create a schedule job and confirm the monthly calendar shows actual scheduled-date allocations.
