# WOMS 實作計畫

## 摘要

WOMS 直接以最終部署型態開發：Go API、Go scheduler worker、原生 HTML/CSS/JS 前端、PostgreSQL、Redis、Apache Kafka、Docker Hub、Helm、Kubernetes、NGINX Ingress、KEDA 與 GitHub Actions。

## 核心實作

- 分支使用 `feat/xxxx-xxxx`，禁止直接在 `main` 開發。
- README 分成 `README.zh-TW.md` 與 `README.en.md`。
- `api` 負責 JWT、RBAC、訂單、試排程、排程任務、回填與 audit log。
- `scheduler-worker` 作為 Kafka scheduling job consumer 的部署單元。
- `web` 是原生 HTML/CSS/JS 工作台。
- Helm chart 控制 image tag、Ingress、KEDA 與資源設定。

## 排程規則

- 產線 `A/B/C/D` 各自每日預設 `10,000` 片。
- 排程工程師只能看到自己的產線。
- 訂單可跨多日排程。
- 狀態只保留 `待排程`、`已排程`、`生產中`、`已完成`。
- 自動排程以最早完成時間為目標。
- 高優先級已排程訂單不可被自動移動。
- 人工強制介入必須記錄 reason、conflict report 與 audit log。

## 驗證要求

- 單元測試覆蓋排程、JWT、RBAC、產線隔離與回填拆單。
- CI 必須通過 `go test ./...`、`gofmt`、Docker build、Helm render。
- K8s 驗證必須確認 Ingress auth、HTTPS、KEDA/HPA scale up/down。

## 假設

- PostgreSQL、Redis、Kafka 的正式 persistence wiring 會在後續 feature slice 完成。
- 目前 foundation 版本先提供可測試的 Go in-memory API 與部署骨架。
