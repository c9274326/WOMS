# WOMS 實作後驗證指南

## 1. 本機靜態與單元測試

```bash
go test ./...
test -z "$(gofmt -l .)"
```

期望結果：

- 所有 Go tests 通過。
- `gofmt` 沒有輸出。

## 2. API/JWT/RBAC 驗證

啟動 API：

```bash
JWT_SECRET=local-dev-secret go run ./cmd/api
```

登入 sales：

```bash
curl -s http://localhost:8080/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"sales","password":"demo"}'
```

檢查無 token：

```bash
curl -i http://localhost:8080/internal/auth/verify
```

期望：`401 Unauthorized`。

檢查 sales 禁止建立排程任務：

```bash
curl -i http://localhost:8080/api/schedules/jobs \
  -H "Authorization: Bearer <sales-token>" \
  -H 'Content-Type: application/json' \
  -d '{"lineId":"A","startDate":"2026-05-01"}'
```

期望：`403 Forbidden`。

檢查排程工程師產線隔離：

- 用 `scheduler-b` 建立 B 線 job。
- 用 `scheduler-a` 查詢該 job。
- 期望：`403 Forbidden`。

## 3. Docker 驗證

```bash
docker build -f Dockerfile.api -t woms-api:local .
docker build -f Dockerfile.worker -t woms-scheduler-worker:local .
docker build -f Dockerfile.web -t woms-web:local .
docker compose up --build
```

期望：

- API health: `curl http://localhost:8080/healthz`
- Web: `http://localhost:8081`

## 4. Helm Render 驗證

```bash
helm template woms ./deploy/helm/woms
```

期望輸出包含：

- `Deployment`：api、worker、web。
- `Ingress`：public、api-secure。
- `ScaledObject`：worker Kafka/CPU triggers。

## 5. Ingress / Gateway 驗證

部署後執行：

```bash
curl -i https://woms.local/api/orders
curl -i https://woms.local/api/orders -H "Authorization: Bearer <valid-token>"
```

期望：

- 無 token 回 `401`。
- 有效 token 通過 Ingress auth。
- API 仍會執行自身 JWT/RBAC 檢查。
- HTTP 會 redirect HTTPS。

## 6. KEDA / HPA 驗證

確認資源：

```bash
kubectl get scaledobject,hpa -n woms
kubectl describe scaledobject -n woms
```

送入大量 Kafka scheduling messages 後：

```bash
kubectl get deploy -n woms -w
kubectl get hpa -n woms -w
```

期望：

- Kafka lag 上升。
- worker replicas 超過 `minReplicaCount`。
- lag 清空並等待 cooldown 後 replicas scale down。
- 若 CPU trigger 未生效，先確認 metrics-server 與 pod resource requests。

## 7. Redis Lock 驗證

同產線同時送兩個排程 job：

- 期望不產生重疊 schedule version。
- 其中一個 job 應等待、重試或乾淨失敗。

不同產線同時送 job：

- 期望可並行處理。

## 8. 完成功能標準

- 測試通過。
- README zh-TW/en 更新。
- `.gitignore` 已涵蓋新增 generated/local files。
- Docker/Helm/CI 設定同步。
- `git add`、commit、push 完成。
