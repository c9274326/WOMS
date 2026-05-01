<p align="center">
  <strong>WOMS</strong>
</p>

<p align="center">
  ?嗅?閮蝞∠???蝔頂蝯?
</p>

<p align="center">
  <a href="README.md">English</a> |
  <a href="README.zh-TW.md">蝜?銝剜?</a>
</p>

<p align="center">
  <img alt="Go" src="https://img.shields.io/badge/Go-1.22-00ADD8?style=flat-square">
  <img alt="Docker" src="https://img.shields.io/badge/Docker-ready-2496ED?style=flat-square">
  <img alt="Kubernetes" src="https://img.shields.io/badge/Kubernetes-Helm-326CE5?style=flat-square">
  <img alt="KEDA" src="https://img.shields.io/badge/KEDA-autoscaling-4B32C3?style=flat-square">
</p>

---

WOMS ?臭誑?蝯蝵脣????潛??嗅?閮蝞∠???蝔頂蝯晞璅霈平?犖?∪遣蝡?餈質馱閮嚗???撌亦?撣思??Ｙ?蝞∠???????蝒?憛急??亦??銝阡? Kafka?edis?EDA ??Kubernetes ?舀???甇仿????游???

## 蝟餌絞?嗆?

```mermaid
flowchart LR
  User[雿輻? --> Ingress[NGINX Ingress / HTTPS]
  Ingress --> Web[Static Web / NGINX]
  Ingress --> API[Go API]
  API --> Auth[JWT + RBAC]
  API --> DB[(PostgreSQL)]
  API --> Redis[(Redis Locks)]
  API --> Kafka[(Apache Kafka)]
  Kafka --> Worker[Go Scheduler Worker]
  Worker --> Redis
  Worker --> DB
  KEDA[KEDA ScaledObject] --> Worker
```

### ?函蔡?桀?

- `web`: ?? HTML/CSS/JS ?垢嚗 NGINX serve??
- `api`: Go REST API嚗?鞎?JWT?BAC???柴岫????蝔遙??憛怨? audit log??
- `scheduler-worker`: Go worker嚗靘 Kafka consumer ????甇交?蝔?
- `deploy/helm/woms`: Kubernetes Helm chart嚗???API?orker?eb?ngress ??KEDA??

## Prerequirements

?祆??桀??閬?摰?銝?撌亙嚗?

- Git
- Go 1.22+
- Docker ??Docker Desktop
- Docker Compose
- kubectl
- Helm 3
- Kubernetes cluster嚗?憒?Docker Desktop Kubernetes?ind?inikube ?蝡?K8s
- NGINX Ingress Controller
- KEDA
- metrics-server嚗閬?霅?CPU autoscaling

?舐?誘蝣箄?嚗?

```bash
go version
docker --version
docker compose version
kubectl version --client=true
helm version
```

## 撠?閮剖?

銴ˊ?啣?霈蝭?嚗?

```bash
cp .env.example .env
```

??閮剖?嚗?

- `JWT_SECRET`: JWT 蝪賜?撖嚗迤撘憓????
- `DEMO_SEED_DATA`: ?身 `true`嚗閬???demo 閮?航身??`false`??
- `DATABASE_URL`: PostgreSQL ???摮葡??
- `REDIS_ADDR`: Redis 雿???
- `KAFKA_BROKERS`: Kafka broker 皜??
- `KAFKA_SCHEDULE_TOPIC`: ??隞餃? topic??
- `DOCKERHUB_NAMESPACE`: Docker Hub namespace??
- `WOMS_IMAGE_TAG`: Docker Compose 雿輻??image tag??閮剔 `latest`嚗? Compose build ?璈???雿輻??tag ??Docker Hub `latest` 靽?銝?氬?

GitHub Actions Docker Hub 閮剖?嚗?

- Repository secret `DOCKERHUB_TOKEN`: Docker Hub Personal Access Token嚗??? Read & Write??
- Repository variable `DOCKERHUB_USERNAME`: Docker Hub username??
- Repository variable `DOCKERHUB_NAMESPACE`: Docker Hub username ??organization namespace??
- 雿輻 repository-level Actions 閮剖??喳???workflows 瘝?摰?? `environment:`嚗??閬?environment-level settings??

Demo 撣唾?嚗?

- 蝞∠??∴?`admin` / `demo`
- 璆剖?嚗sales` / `demo`
- A 蝺?蝔?`scheduler-a` / `demo`
- B 蝺?蝔?`scheduler-b` / `demo`
- C 蝺?蝔?`scheduler-c` / `demo`
- D 蝺?蝔?`scheduler-d` / `demo`

## ?祆??

?瑁?皜祈岫嚗?

```bash
go test ./...
```

?? API嚗?

```bash
JWT_SECRET=local-dev-secret go run ./cmd/api
```

雿輻 Docker Compose ??嚗?

```bash
docker compose up --build
```

?身??嚗?

- API: `http://localhost:8080`
- Web: `http://localhost:8081`
- PostgreSQL: `localhost:5432`
- Redis: `localhost:6379`
- Kafka: `localhost:9092`

?垢銵嚗?

- ?芰?交????脣?函??餃???? session 摮???＊蝷箏?券??Ｕ?
- ?餃???摮?汗??`localStorage`嚗??唳??????session嚗??JWT ???◤ API ????
- admin ?臬 Admin panel ?晷撣唾?閫??scheduler ?撅祉蝺???admin ?澆?? `403`??
- ?桀??Ｙ??豢??典? sales/admin ?身?箏??詨??雿??Ｙ?嚗cheduler ??摰?撅祉蝺?
- 蝎暹?蝭拚?舀摰Ｘ????恥?嗆?粹?桀?蝭拚嚗??桃??撌血???踵?嗚?
- 閮??摮蝯梯??桀??詨??Ｙ???
- 月曆會顯示完整六週頁面範圍內正式保存的排程產能，包含相鄰月份日期；水位主要顯示當日剩餘可排片數。試排 allocation 只會出現在試排確認頁，不會混入主月曆。
- sales 只能加入客戶訂單到待排程；草稿可行性會與既有已排程配置檢查，不會把其他待排程訂單一起試算。Order notes are write-on-create only; rejected-order resubmission can adjust due date and quantity but cannot rewrite the original note。
- scheduler 可以先預覽已選取的待排程訂單，也可以把待排程訂單拖到任何可見且非過去的月曆日期。拖曳排程會以當日作為最快起排日並優先填入最早可用產能，因此未來交期訂單會先吃掉今天剩餘產能，再使用交期日產能。發生衝突時，preview 頁會顯示分配計畫、建議提前開始日、單筆訂單交期修改重試，以及允許時的人工強制介入重新試排。人工介入必須填寫原因並逐項確認衝突清單後才會接受任務；缺少 `previewId` 的直接排程 API 會被拒絕。
- scheduler workflow history 是從 backend audit data 載入，透過 `GET /api/schedules/history` 顯示 scheduler 所屬產線的 schedule jobs、manual force、rejected orders 與 production events。
- 已排程訂單可以轉入生產中。開始生產會鎖住該訂單所有 allocation。生產中訂單可以回報全部完成或部分完成；部分完成會把已生產數量結案，並將未生產 allocation 轉給剩餘數量的已排程子訂單。
- `scheduler-a` demo 閮 `ORD-2` 撌脰?銝?demo allocation嚗?甇斗?憿舐內?冽???
- 銵?皜祈岫???遣蝡?撘萄??亙之???殷??嫣噶??preview ?銵??勗???

鞈????牧??

- Docker Compose ??PostgreSQL 雿輻 `postgres-data` named volume嚗?甇斗璈?DB 鞈?? container ??敺???
- ?桀? foundation API 隞蝙??in-memory store?ostgreSQL migration ??seed files 撌脣??剁?雿?API 撖怠 PostgreSQL ??persistence wiring ?敺? feature slice 摰???
- Helm chart ?桀?雿輻 `DATABASE_URL` ??鞈?摨恬?撠?批遣 PostgreSQL StatefulSet/PVC??

## Docker Build

```bash
docker build -f Dockerfile.api -t woms-api:local .
docker build -f Dockerfile.worker -t woms-scheduler-worker:local .
docker build -f Dockerfile.web -t woms-web:local .
```

## Kubernetes ?函蔡

?Ⅱ隤?cluster 撌脣?鋆?NGINX Ingress?EDA ??metrics-server??

Render Helm嚗?

```bash
helm template woms ./deploy/helm/woms \
  --set api.image.repository=docker.io/<namespace>/woms-api \
  --set worker.image.repository=docker.io/<namespace>/woms-scheduler-worker \
  --set web.image.repository=docker.io/<namespace>/woms-web \
  --set api.image.tag=<tag> \
  --set worker.image.tag=<tag> \
  --set web.image.tag=<tag>
```

?函蔡嚗?

```bash
helm upgrade --install woms ./deploy/helm/woms \
  --namespace woms --create-namespace \
  --set ingress.host=woms.local \
  --set api.jwtSecret=<strong-secret> \
  --set api.image.repository=docker.io/<namespace>/woms-api \
  --set worker.image.repository=docker.io/<namespace>/woms-scheduler-worker \
  --set web.image.repository=docker.io/<namespace>/woms-web \
  --set api.image.tag=<tag> \
  --set worker.image.tag=<tag> \
  --set web.image.tag=<tag>
```

## CI/CD

GitHub Actions ?銵?

- `go test ./...`
- `npm run test:web`
- `gofmt` 瑼Ｘ
- API?orker?eb Docker build
- Helm render
- `main`?release/**` ???銵??? Docker Hub push ??tag
- `main` publish ??敺???Helm image tag
- 瘥活?? publish ??`main` ??遣蝡?Git tag嚗?閮剜撘 `v0.1.<run-number>`

GitHub repository ?閮剖?嚗?

- Secret: `DOCKERHUB_TOKEN`
- Variable: `DOCKERHUB_USERNAME`
- Variable: `DOCKERHUB_NAMESPACE`

Image tags ????release tag嚗誑??protected main/release publish flow ??`latest`?docker-publish` workflow ?? release tag ?神??`deploy/helm/woms/values.yaml` 銝衣 `[skip ci]` commit嚗?敺遣蝡???Git tag??

?瘚?嚗?

- `main` 敹?摮銝血??其?霅瑯?
- ??賢 `feat/xxxx-xxxx` ??脰???
- 敺?`feat/...` ??PR ??`main` 隞亥孛??CI bot??
- `docker-publish` ?芸蝔??脣 `main`?release/**` ???孛?潭??瑁???
- 銝???feature branch push ????Docker Hub publishing??

## 撖虫?敺?霅?

摰撽?甇仿?隢?嚗?

- [撽??? zh-TW](docs/verification.zh-TW.md)
- [Verification Guide en](docs/verification.en.md)

頛?單嚗?

```bash
BASE_URL=http://localhost:8080 ./scripts/smoke-api.sh
NAMESPACE=woms ./scripts/verify-k8s.sh
```

?雿???皞?

- API ??token ??`401`??
- sales ?澆 scheduler API ??`403`??
- scheduler A 銝霈撖?scheduler B ?蝺???
- `helm template` ?舐??Ingress ??KEDA `ScaledObject`??
- Kafka lag 銝???worker replicas scale up嚗ag 皜征敺?scale down??
- README?葫閰艾ommit?ush 敹??冽????賢???
