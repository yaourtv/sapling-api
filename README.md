# job-tracker-api

Go backend for **job-tracker**, a personal, single-tenant job application tracker:

- **Applications** — cards with custom, user-defined statuses and a full status-change history.
- **Calendar** — personal events, recurring/one-time availability windows, and an inbox of pending interview invitations.
- **Public calendar** — a 6-character personal link (`/{token}`) where interviewers see only Busy/Tentative/Available slots and can submit an interview invitation. No accounts, no titles exposed.

Companion repos: `job-tracker-web` (React + Vite frontend) and `job-tracker-deploy` (Docker Compose stack).

## Tech stack

- Go 1.25, [chi v5](https://github.com/go-chi/chi) router
- PostgreSQL 16 via [pgx/v5](https://github.com/jackc/pgx) + [sqlc](https://sqlc.dev) (typed queries, no ORM)
- [goose v3](https://github.com/pressly/goose) migrations, embedded and applied automatically on startup
- JWT auth (access + rotating refresh tokens) with `golang-jwt/jwt/v5`, bcrypt passwords
- Background auto-archive job (moves stale applications to the system "Lost" status every 6 h)

## Prerequisites

- Go 1.25+
- PostgreSQL 16 (or Docker to run one)
- [sqlc](https://docs.sqlc.dev/en/latest/overview/install.html) — only if you change SQL queries (`internal/db/queries/`); generated code is committed under `internal/db/query/`

## Setup

```sh
cp .env.example .env
# then edit .env:
#   DATABASE_URL          postgres connection string
#   ACCESS_TOKEN_SECRET   openssl rand -hex 32
#   REFRESH_TOKEN_SECRET  openssl rand -hex 32
#   CORS_ORIGIN           frontend origin, e.g. http://localhost:5173
```

SMTP variables are optional. Without `SMTP_HOST`, password-reset links are printed to stdout instead of emailed.

## Run locally

```sh
# disposable database
docker run -d --name jt-pg -p 5432:5432 \
  -e POSTGRES_USER=jobtracker -e POSTGRES_PASSWORD=jobtracker -e POSTGRES_DB=jobtracker \
  postgres:16-alpine

# run the API (migrations apply automatically)
set -a; source .env; set +a
go run ./cmd/api
```

Health check: `curl http://localhost:8080/healthz`

## Develop

```sh
go build ./...   # build
go vet ./...     # lint
go test ./...    # tests (token + slot-computation logic)
sqlc generate    # regenerate internal/db/query after editing queries
```

## API overview

| Area | Base path | Auth |
|------|-----------|------|
| Auth (register/login/refresh/logout/password reset) | `/api/auth/*` | — |
| Profile + calendar link | `/api/user/*` | Bearer |
| Applications + status history | `/api/applications*` | Bearer |
| Custom statuses | `/api/statuses*` | Bearer |
| Events, availability, invitations | `/api/calendar/*` | Bearer |
| Public calendar + invite submission | `/api/c/{token}*` | none (rate-limited) |

Notable behaviors:

- Registration seeds five statuses (Applied, Interview, Accepted, Rejected, and the undeletable system status **Lost**) and generates the 6-char public calendar token.
- Refresh tokens rotate on every `/api/auth/refresh`; they are stored hashed and also set as an `HttpOnly` cookie.
- Deleting a status that applications still use returns `409` with `{affected_count, lost_status_id}`; retry with `?confirm=true` to move those applications to Lost.
- The public calendar returns the owner's `display_name` and `timezone` alongside the slots, so whoever follows the link can see they are booking with the right person. Nothing else about the owner is exposed — slots carry only times and a type (`busy` / `tentative` / `available`), never titles. Available slots are 30-minute granules computed from availability windows minus busy/tentative overlap.
- Published slots are 30-minute granules, but a **submitted invitation** only has to fit one open availability window and clash with nothing already booked — it may last any positive multiple of 5 minutes and need not align to a granule boundary. A proposal spanning two adjacent windows is refused.
- Availability windows support `PATCH /api/calendar/availability/{id}`: every field is optional and an omitted one keeps its current value, so editing a window's hours preserves its identity. Flipping `is_recurring` clears whichever of `day_of_week` / `specific_date` no longer applies.

## Deploy

```sh
docker build -t job-tracker-api .
docker run --env-file .env -p 8080:8080 job-tracker-api
```

The full deployment stack (Traefik, Postgres, frontend) lives in `job-tracker-deploy`. CI (`.github/workflows/ci.yml`) runs tests/vet/golangci-lint, builds a multi-arch image, scans it with Trivy, and pushes to GHCR on version tags.
