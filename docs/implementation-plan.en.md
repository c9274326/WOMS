# WOMS Implementation Plan

## Summary

WOMS is implemented directly in its final deployment shape: Go API, Go scheduler worker, vanilla HTML/CSS/JS frontend, PostgreSQL, Redis, Apache Kafka, Docker Hub, Helm, Kubernetes, NGINX Ingress, KEDA, and GitHub Actions.

## Core Implementation

- Branches use `feat/xxxx-xxxx`; direct development on `main` is forbidden.
- README is split into `README.zh-TW.md` and `README.en.md`.
- `api` owns JWT, RBAC, orders, schedule preview, schedule jobs, production confirmation, and audit logs.
- `scheduler-worker` is the deployable unit for Kafka scheduling job consumption.
- `web` is a vanilla HTML/CSS/JS operator console.
- The Helm chart controls image tags, Ingress, KEDA, and resource settings.

## Scheduling Rules

- Lines `A/B/C/D` each default to `10,000` wafers per day.
- Scheduler users only see their assigned line.
- Orders may be scheduled across multiple days.
- Statuses are only `待排程`, `已排程`, `生產中`, and `已完成`.
- Automatic scheduling optimizes for earliest finish time.
- Scheduled high-priority orders cannot be moved automatically.
- Manual force intervention must record a reason, conflict report, and audit log.

## Verification Requirements

- Unit tests cover scheduling, JWT, RBAC, line isolation, and production confirmation splitting.
- CI must pass `go test ./...`, `gofmt`, Docker build, and Helm render.
- Kubernetes verification must prove Ingress auth, HTTPS, and KEDA/HPA scale up/down.

## Assumptions

- Production persistence wiring for PostgreSQL, Redis, and Kafka will be completed in later feature slices.
- The foundation version provides a testable Go in-memory API and deployment skeleton.
