# WOMS 實作後驗證指南

## 1. 本機靜態與單元測試

```bash
go test ./...
npm run test:web
test -z "$(gofmt -l .)"
```

期望結果：

- 所有 Go tests 通過。
- 前端 mock tests 通過。
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

檢查月曆行為：

- 用排程工程師建立排程任務。
- `GET /api/schedules/calendar?lineId=A&month=2026-05` 會回傳已保存 allocations。
- 查詢其他排程工程師的產線會回錯誤。

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

## 9. 前端 Smoke 驗證

- 在 `http://127.0.0.1:8081` 登入。
- 重新整理瀏覽器，確認 session 會恢復。
- 確認登入後會隱藏帳號密碼欄位，頁首顯示目前帳號與登出按鈕。
- 使用 `admin` / `demo` 登入，確認 Admin panel 可見，且非 admin 看不到。
- 切換客戶、產線、優先級精準篩選；確認狀態篩選是單選。
- 用 sales 建立草稿訂單 preview，確認 preview page 會高亮日曆結果，再確認放到待排程訂單。
- 用 scheduler 選取待排程訂單，先 preview，再從 preview page 確認執行。缺少 `previewId` 的直接排程 API 必須失敗。
- 刪除已選取的待排程/已排程訂單，確認被刪除訂單的月曆 allocation 也會消失。
- 使用衝突測試按鈕建立同日大量訂單，preview 後確認會顯示衝突報告。
- 確認權限不足與操作錯誤都會用彈出訊息視窗顯示。
