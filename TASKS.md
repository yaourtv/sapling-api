# TASKS — job-tracker-api

Scope: this directory is the `job-tracker-api` repo from the spec (Go backend).
The `job-tracker-web` and `job-tracker-deploy` repos are separate and out of scope
for this working directory.

## Foundation
- [x] git init + .gitignore
- [x] go.mod (chi v5, pgx/v5, sqlc runtime, goose v3, golang-jwt/v5, bcrypt, go-chi/cors)
- [x] internal/config/config.go — env var loading + validation
- [x] internal/db/migrations/00001_init.sql — full schema (goose)
- [x] internal/db/db.go — pgxpool setup + embedded goose migration runner

## Data layer (sqlc)
- [x] sqlc.yaml
- [x] sqlc schema source: sqlc.yaml points at internal/db/migrations directly (no separate schema.sql needed)
- [x] internal/db/queries/*.sql — users, refresh_tokens, statuses, applications, history, calendar_events, availability_windows, invitations, archive
- [x] Generate internal/db/query/ via `sqlc generate`

## Core packages
- [x] internal/model/model.go — domain types (API-facing)
- [x] internal/token/token.go — JWT issue/validate (access, refresh opaque+hash, reset)
- [x] internal/token/token_test.go
- [x] internal/middleware/auth.go — JWT validation middleware
- [x] internal/middleware/ratelimit.go — per-IP rate limit
- [x] internal/middleware/cors.go — CORS from config

## Services
- [x] internal/service/errors.go — typed service errors
- [x] internal/service/auth.go — register (seed statuses, calendar token), login, refresh rotation, logout, password reset (SMTP optional, stdout fallback)
- [x] internal/service/status.go — CRUD + delete rules (403 system / 409 conflict / confirm → move to Lost)
- [x] internal/service/application.go — CRUD, list (+include_lost), status change + history
- [x] internal/service/calendar.go — events CRUD, availability windows, invitations accept/decline, public slot computation (busy/tentative/available, 30-min granules)
- [x] internal/service/calendar_test.go — slot computation tests
- [x] internal/service/archive.go — auto-archive goroutine (6h ticker) + history rows

## HTTP layer
- [x] internal/handler/respond.go — JSON helpers, error mapping
- [x] internal/handler/auth.go — /api/auth/*
- [x] internal/handler/user.go — /api/user/*
- [x] internal/handler/application.go — /api/applications*
- [x] internal/handler/status.go — /api/statuses*
- [x] internal/handler/calendar.go — /api/calendar/*
- [x] internal/handler/public.go — /api/c/:token*
- [x] internal/handler/router.go — chi router wiring
- [x] cmd/api/main.go — bootstrap: config, db, migrations, archive goroutine, graceful shutdown

## Infrastructure
- [x] .env.example
- [x] Dockerfile (multi-stage: golang → alpine)
- [x] .github/workflows/ci.yml — test, vet, golangci-lint, buildx, GHCR push on tag, trivy
- [x] README.md

## Verification
- [x] BUILD: `go build ./...`
- [x] LINT: `go vet ./...`
- [x] VERIFY: `go test ./...`
- [x] VERIFY (runtime): boot server against dockerized Postgres, exercise register → statuses → application → calendar → public invite flow
- [x] git add -A && git commit
