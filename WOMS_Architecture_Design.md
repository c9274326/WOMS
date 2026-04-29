# Architecture Design - Wafer Order Management System
## 1. Assumptions

1. 初期以單一晶圓廠為主，日產能上限為 10,000 片。
2. 單筆訂單量介於 25 到 2,500 片。
3. 訂單進度由現場生產系統定期同步到 WOMS。
4. 系統預設部署在雲端。

## 2. Target Users and Main Scenarios

| 角色    | 主要目標                   | 主要操作                   |
| ----- | ---------------------- | ---------------------- |
| 業務人員  | 快速確認能否接單、回覆客戶交期、掌握訂單狀態 | 建立訂單、修改訂單、查詢進度         |
| 排程工程師 | 最大化產能利用率、降低重排衝擊        | 查看日曆、分析利用率、回填實際產量、觸發重排 |
主要情境：

1. 業務建立新訂單前先試排程。
2. 訂單異動後系統重新排程。
3. 排程工程師查看日曆與產能利用率。
4. 使用者查詢訂單進度與預估完成時間。
5. 排程工程師每日回填實際生產數量，若未完成則將剩餘數量拆出並重新排程。

## 3. Business Rules and Constraints

| 規則      | 說明                                         |
| ------- | ------------------------------------------ |
| 日產能限制   | 每日總排程量不得超過 10,000 片                        |
| 單筆訂單量   | 每筆訂單量必須介於 25 到 2,500 片                     |
| 訂單狀態    | 待排程、已排程、生產中、已取消                            |
| 高優先級訂單  | 觸發重排時，高優先級訂單只能被同為高優先級的訂單影響與重排              |
| 生產回填與拆單 | 排程工程師每日回填實際生產數量；若訂單未完成，剩餘數量需沿用原訂單結構拆成新排程項目 |

## 4. Functional Requirements

1. 支援 Wafer 訂單新增、修改、取消。
2. 支援試排程、自動排程與手動重排。
3. 提供日曆視圖與列表視圖查詢訂單與產能。
4. 顯示訂單進度與預估完成時間。
5. 支援高優先級訂單標記、備註與通知。
6. 觸發重排時，高優先級訂單只能被同為高優先級的訂單影響。
7. 排程工程師可回填每日實際生產數量。
8. 若訂單實際生產數量少於原排程數量，系統需自動建立剩餘數量的排程項目並重新排程。

## 5. Non-Functional Requirements

| 類別            | 設計要求                                |
| ------------- | ----------------------------------- |
| Correctness   | 排程結果必須遵守產能限制與訂單規則                   |
| Consistency   | 訂單異動、排程版本與查詢結果必須可追蹤                 |
| Availability  | 系統需在查詢與重排下持續可用                      |
| Scalability   | 架構需能擴展到多廠區與高流量                      |
| Performance   | 查詢低延遲，試排程快速回應，重排可非同步                |
| Security      | 必須具備認證、授權、資料隔離與審計                   |
| Observability | 必須提供 metrics、logs、traces 與 alerting |

## 6. Cloud Architecture

預期完成的雲端架構包含：

1. CDN + Load Balancer + API Gateway: 入口層，負責加速、分流、統一 API 管理
	1. CDN: 負責把靜態內容放到邊緣節點，降低延遲
		- 解決: 靜態資源離使用者太遠、首頁 / JS / CSS 載入慢、原站壓力大。
	2. Load Balancer: 負責將前端或 API 請求分流到多個後端 Application Service
		- 解決: 單一後端掛掉就整個服務中斷、無法做多實例分流
	3. API Gateway: 負責統一 API 入口，做認證、路由、限流、版本管理
		- 解決: 認證、限流、路由散在各服務裡會很亂，後期 API 多了很難管。
2. Application Services: 介於 API Gateway 和資料庫之間的應用層，負責把「使用者想做的事」轉成「系統真正要執行的流程」，包含、排程、回填、拆單、通知等流程
3. Database (PostgreSQL): 負責存放高一致性、強關聯的資料，如訂單、排程版本、生產回填紀錄
4. Cache (Redis): 負責做快取、暫存、計數器、分散式鎖
	- 解決: 重複查詢太多、主資料庫壓力大、某些流程(例如同時修改訂單)需要鎖來避免重複執行
5. Managed Queue (Kafka): 負責把耗時工作或事件(EX: 排程)異步傳遞出去，讓後續 worker/service 消化重排、拆單、通知等流程
	- 解決: 同步處理會讓 API 變慢甚至 timeout
6. Kubernetes: 負責管理多個 service 和 worker，包含部署、服務發現、健康檢查、滾動更新、自我修復與擴縮
7. Monitor: 監測系統健康狀況，例如 CPU、memory、latency、error rate、queue depth，提供系統健康指標

![](assets/WOMS%20Architecture%20Design/file-20260420162950322.png)

## 7. Shared Design Principles

1. 訂單異動先寫入交易資料庫，再觸發排程事件。
2. 排程結果以版本化方式保存，避免讀到半完成資料。
3. 客戶查詢需做資料隔離，所有修改操作需保留 audit log。
4. 監控 API latency、queue backlog、schedule failure rate 與重排耗時。
5. 服務部署需支援健康檢查、滾動更新與自動擴展，避免高流量時整體服務不穩。

## 8. Conclusion

WOMS 採雲端部署，並以同一套架構逐步演進：Initial Stage 先建立可運作的 modular monolith 基礎，Growth Stage 在原本架構上拆出關鍵服務並加入 Read Replica 與 Read Model，High-Traffic Stage 再在既有服務化架構上加入 Event-Driven Architecture、Partitioned PostgreSQL、Read Store、Distributed Redis、多區域部署與自動擴展能力。這樣的設計更符合實際專案最終 Demo 需要涵蓋所有能力、同時保留可擴展性的需求。
