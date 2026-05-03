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
./scripts/verify-hpa-render.sh
```

期望輸出包含：

- `Deployment`：api、worker、web。
- `Ingress`：public、api-secure。
- `ScaledObject`：worker Kafka/CPU triggers。
- `ScaledObject.spec.advanced.horizontalPodAutoscalerConfig.name`：`woms-woms-worker-hpa`。
- `PodDisruptionBudget`：api 與 web，且 `minAvailable: 1`。

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

用 admin 登入 web，按「建立多產線排程尖峰」。確認畫面顯示 200 條產線、1,000 張訂單與 200 個 queued jobs，並顯示 Kafka topic、consumer group、HPA 與 deployment 名稱。接著觀察：

```bash
kubectl get deploy -n woms -w
kubectl get hpa -n woms -w
NAMESPACE=woms ./scripts/verify-k8s.sh
```

期望：

- Kafka lag 上升。
- worker replicas 超過 `minReplicaCount`。
- lag 清空並等待 cooldown 後 replicas scale down。
- 若 CPU trigger 未生效，先確認 metrics-server 與 pod resource requests。
- demo 後按「清除排程尖峰資料」，確認 `L001-L200` 訂單與 jobs 清空。

## 7. API/Web High Availability 驗證

```bash
kubectl get deploy,pdb -n woms
kubectl describe pdb woms-woms-api -n woms
kubectl describe pdb woms-woms-web -n woms
```

期望：

- API 與 web 預設各有兩個 replicas。
- API 與 web PDB 都要求 `minAvailable: 1`。
- 在多節點 cluster 發生 voluntary disruption 時，至少保留一個 API pod 與一個 web pod 可用。

## 8. Redis Lock 驗證

同產線同時送兩個排程 job：

- 期望不產生重疊 schedule version。
- 其中一個 job 應等待、重試或乾淨失敗。

不同產線同時送 job：

- 期望可並行處理。

## 9. 完成功能標準

- 測試通過。
- README zh-TW/en 更新。
- `.gitignore` 已涵蓋新增 generated/local files。
- Docker/Helm/CI 設定同步。
- `git add`、commit、push 完成。

## 10. 前端 Smoke 驗證

- 在 `http://127.0.0.1:8081` 登入。
- 重新整理瀏覽器，確認 session 會恢復。
- 確認登入後會隱藏帳號密碼欄位，頁首顯示目前帳號與登出按鈕。
- 使用 `admin` / `demo` 登入，確認 Admin panel 可見，且非 admin 看不到。
- 切換客戶、產線、優先級精準篩選；確認狀態篩選是單選，且客戶選單只列出目前狀態/優先級範圍內的客戶。
- 用 scheduler 把較晚交期的待排程訂單拖到指定的非過去月曆日期，接受 preview 後確認正式 allocation 保留在拖放日期。
- 用 scheduler 建立衝突，在衝突面板選取衝突訂單與可移動的低優先級已排程訂單，預覽最早完成解法，接受後確認被移動訂單的舊未鎖定 allocation 已被替換。
- 用 scheduler 點擊月曆內的已排程訂單，確認可轉為生產中；再點擊生產中訂單，確認可開啟回報生產。
- 輸入部分完成數量後送出，確認同一張訂單編號會以剩餘數量回到待排程。
- 用 sales 建立草稿訂單 preview，確認 preview page 會高亮日曆結果，再確認放到待排程訂單。
- 用 scheduler 選取待排程訂單，先 preview，再從 preview page 確認執行。缺少 `previewId` 的直接排程 API 必須失敗。
- 刪除已選取的待排程/已排程訂單，確認被刪除訂單的月曆 allocation 也會消失。
- 使用衝突測試按鈕建立同日大量訂單，preview 後確認衝突面板佔滿預覽視窗右側，且解法控制項不會被裁切。
- 確認權限不足與操作錯誤都會用彈出訊息視窗顯示。
