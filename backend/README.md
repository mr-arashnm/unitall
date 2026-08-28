# Unital Backend — Go Microservices

Go implementation of the Unital building-management platform. 6 bounded-context services behind an API gateway, each with its own store adapter (in-memory for dev, Postgres for production).

---

## 1. Architecture

```
                              Browser / Client
                                     │
                              ┌──────▼──────┐
                              │   Gateway    │  :8080  (jwt validation, routing)
                              │  (reverse     │
                              │   proxy)     │
                              └──┬───┬───┬───┘
                                 │   │   │  /api/v1/*
                   ┌─────────────┴┐ ┌┴───┴───┴─────────┐
           identity :9001  property :9002  billing :9003
          facilities :9004  operations :9005  notif :9006
                                 │   │
                         PostgreSQL per service
```

| Service | Port | Responsibility |
|---|---|---|
| **gateway** | 18080 | JWT validation, user-context injection (`X-User-Id`, `X-User-Role`), routing to upstream services |
| **identity** | 19001 | Auth (register/login/refresh/logout/verify), profile, building memberships & per-building RBAC |
| **property** | 19002 | Buildings, units, assets (parking/warehouse), ownership/residency with validity ranges, contracts with activation side effects |
| **billing** | 19003 | Charge templates, idempotent period generation, payments, invoices, financial reports, overdue sweeper |
| **facilities** | 19004 | Facility definitions, availability calendar, bookings with approval workflow, maintenance windows |
| **operations** | 19005 | Teams, tasks (lifecycle: pending→in_progress→completed), service requests (resident intake → auto-task) |
| **notifications** | 19006 | Announcements, meetings (RSVP+minutes), support tickets, notification templates, personal inbox |

**Technology**: Go 1.26 · stdlib `net/http` · PostgreSQL (production) · In-memory stores (dev) · HS256 JWT · PBKDF2-SHA256 passwords · Structured `slog` JSON logging · CloudEvents-shaped domain events.

---

## 2. Quick Start

### 2.1 Prerequisites

- Docker & Docker Compose
- `go` 1.26+ (for local/standalone runs, not required for docker mode)

### 2.2 Bring up the full stack

```bash
cd backend
make up              # docker compose -f ../docker-compose.yml up --build -d
make ps              # verify all containers are healthy
```

Wait ~15 seconds for all healthchecks to pass, then open:

| URL | Description |
|---|---|
| [http://localhost:18000](http://localhost:18000) | Resident app (nginx + gateway) |
| [http://localhost:18080/api/docs](http://localhost:18080/api/docs) | Swagger UI |
| [http://localhost:18080/api/schema](http://localhost:18080/api/schema) | OpenAPI 3.1 YAML |

> **Note**: On this machine MinIO occupies `:8080/:9001`, so the dev stack uses ports `18080` (gateway) and `19001–19006` (services).

### 2.3 First steps (Swagger or curl)

```bash
# Register a user
curl -s -X POST http://localhost:18080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"ali@unital.app","password":"Test1234","full_name":"Ali Rezai"}' \
  | python3 -m json.tool

# Login (DEV_AUTOVERIFY=1 auto-verifies accounts — no email needed)
curl -s -X POST http://localhost:18080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"ali@unital.app","password":"Test1234"}' \
  | python3 -m json.tool
```

Copy the `access_token` from the login response, click **Authorize** in Swagger UI, paste it — you now have a live session.

---

## 3. Run Commands

```bash
make up          # docker compose up --build -d   (full stack)
make down        # docker compose down
make logs        # docker compose logs -f --tail=50
make ps          # docker compose ps

go run ./gateway                          # gateway only (port :8080)
go run ./services/identity/cmd/server      # identity only (port :9001)
go run ./services/property/cmd/server     # property only (port :9002)
# ... and so on for billing, facilities, operations, notifications
```

**Resetting Postgres** (clears all data):

```bash
docker stop unital_backend-postgres-1 && \
  docker rm unital_backend-postgres-1 && \
  docker volume rm -f unital_backend_pgdata && \
  make up
```

---

## 4. Tests

### 4.1 Unit tests (no external dependencies)

```bash
make test              # go test ./... -count=1
make race             # go test ./... -race -count=1   (race detector)
make vet              # go vet ./...
make fmt              # gofmt -w .
```

Every service HTTP handler is tested with **in-memory stores** — no Postgres, no Docker, no network required. 7 packages with tests:

| Package | What is tested |
|---|---|
| `gateway` | Route→service mapping (60+ paths), CORS middleware |
| `pkg/httpx` | Request-ID middleware (generate, propagate, error responses) |
| `identity/httpapi` | Register/login/refresh/logout/verify, platform roles, membership RBAC |
| `property/httpapi` | Buildings/units/assets/contracts, ownership transfer, RBAC |
| `billing/httpapi` | Charge templates, idempotent generation, payments, invoices, overdue |
| `facilities/httpapi` | Bookings (overlap, capacity, hours), maintenance windows |
| `notifications/httpapi` | Announcements, meetings (RSVP+minutes), tickets, inbox |
| `operations/httpapi` | Teams, tasks lifecycle, service requests |

### 4.2 Integration smoke test (requires live stack)

```bash
# Stack must be running (make up)
UNITAL_INTEGRATION_BASE_URL=http://localhost:18080 go test ./integration/... -v
```

Covers end-to-end: register → login → buildings → units → assets → billing → facilities → operations → communications → gateway routing. Uses a fresh test user (`e2e-{timestamp}@unital.app`) per run.

### 4.3 All tests (CI pattern)

```bash
make test && UNITAL_INTEGRATION_BASE_URL=http://localhost:18080 go test ./integration/... -count=1
```

---

## 5. Authentication Flow

```
Browser                          Gateway                       Identity
  │                                │                             │
  ├─ POST /auth/register ────────► │                             │
  │                                ├─ forward ──────────────────► │
  │                                │                             │
  │◄────── user JSON ─────────────┤ ◄─── user JSON ─────────────┤
  │                                │                             │
  ├─ POST /auth/login ────────────► │                             │
  │                                ├─ forward ──────────────────► │
  │                                │    (DEV_AUTOVERIFY=1: account │
  │                                │     is auto-verified)        │
  │◄─── { access_token, ─────────┤ ◄─── token pair ────────────┤
  │        refresh_token }        │                             │
  │                                │                             │
  ├─ GET /buildings ─────────────► │                             │
  │  Authorization: Bearer <at>   │                             │
  │                                ├─ verify signature ─────────► │
  │                                │  (same JWT_SECRET)          │
  │                                ├─ inject X-User-Id ─────────► │
  │◄────── buildings JSON ────────┤ ◄─── buildings JSON ─────────┤
```

- **Access token**: HS256 JWT, **24-hour TTL** (changed from 15 min to avoid silent expiry during normal use). Signed with `JWT_SECRET`.
- **Refresh token**: Opaque, rotating, stored hashed in Postgres (or in-memory). Used via `POST /auth/refresh`.
- **Gateway role**: Validates JWT signature + expiry, then injects `X-User-Id` + `X-User-Role` headers for upstream services.
- **Dev mode flags**:
  - `DEV_AUTOVERIFY=1` — identity auto-verifies accounts at registration (no email needed in dev).
  - `DEV_TRUST_ALL=1` — property/billing/notifications bypass RBAC (dev/demo only).

---

## 6. Internal (Service-to-Service) Routes

Gateway restricts `/internal/*` paths to requests with a valid `X-Internal-Token` header (set via `INTERNAL_TOKEN` env var). These routes are for cross-service calls only:

| Endpoint | Service | Purpose |
|---|---|---|
| `GET /internal/users?email=` | identity | Find user by email prefix |
| `POST /internal/users/invite` | identity | Create stub user + send invite email |
| `GET /internal/users/{id}` | identity | Get user by ID |
| `PATCH /internal/users/{id}/platform-role` | identity | Assign/revoke platform role |
| `POST /internal/buildings/{id}/bootstrap-manager` | identity | Auto-grant manager role |

---

## 7. Key Gotchas

| Issue | Symptom | Fix |
|---|---|---|
| **Postgres volume persists** | `make down && make up` loses nothing; old data may cause FK/UUID mismatches | `docker volume rm unital_backend_pgdata` between major resets |
| **Stale gateway IP in nginx** | 502 Bad Gateway after `make up` | `docker restart unital_backend-frontend-1` to clear nginx's DNS cache |
| **JWT signature mismatch** | 401 UNAUTHORIZED on all requests | Ensure `JWT_SECRET` env var is identical in gateway + identity (defaults: `dev-secret-change-me`) |
| **15-min token expiry** | Silent 401s after 15 min of inactivity | Fixed: TTL is now 24 h; refresh via `POST /auth/refresh` |
| **Missing `pgcrypto` extension** | Bootstrap errors on fresh Postgres | Migrations auto-create it; run against existing DB: `CREATE EXTENSION pgcrypto;` |
| **Gateway `pickService` gaps** | 404 at gateway layer for new route groups | Rule: new top-level `/api/v1/<group>` needs a `case` in `gateway/main.go pickService()` |
| **Diagnostic rule** | Handler returns 500 with no trace | Any handler calling `a.fail(err)` must `slog.Error` first so the real error reaches logs |

---

## 8. Project Status

| Phase | Status | Notes |
|---|---|---|
| P1 Identity + Gateway | ✅ Complete | JWT auth, memberships, RBAC, all CRUD |
| P2 Property | ✅ Complete | Buildings, units, assets, ownership/residency, contracts |
| P3 Billing | ✅ Complete | Templates, charges, payments, invoices, financial report |
| P4 Facilities + Operations | ✅ Complete | Bookings, maintenance, teams, tasks, service requests |
| P5 Communications + Notifications | ✅ Complete | Announcements, meetings, tickets, templates, inbox |
| P6 Frontend | ✅ Complete | Resident app served from docker; live against gateway |
| P7 Hardening | 🔄 Partial | Unit + integration tests green; CI/CD not yet set up |

**Remaining**: NATS event bus, Redis caching, production-grade secrets management, load tests, Django ETL migration.

---

## 9. File Layout

```
backend/
├── Makefile                 # make up/test/race/vet/fmt/down/logs/ps
├── go.mod / go.sum          # single Go module: module unital/backend
├── gateway/
│   └── main.go              # API gateway (JWT validation, routing, CORS)
├── pkg/
│   ├── config/              # environment variable loader
│   ├── events/              # CloudEvents-shaped log publisher
│   ├── httpx/               # middleware (RequestID, CORS, Recover, AccessLog, Chain)
│   ├── ids/                 # sortable UUIDv7 generator
│   ├── jwtx/                # HS256 JWT signer/parser
│   └── password/             # PBKDF2-SHA256 hasher
├── services/
│   ├── identity/            # auth, users, memberships
│   ├── property/            # buildings, units, assets, contracts
│   ├── billing/             # charges, invoices, payments
│   ├── facilities/          # bookings, maintenance windows
│   ├── operations/          # teams, tasks, service requests
│   └── notifications/       # announcements, meetings, tickets, inbox
├── integration/
│   └── smoke_test.go        # end-to-end smoke against live gateway
├── openapi/
│   └── unital-v1.yaml       # OpenAPI 3.1 spec (served at /api/schema)
├── migrations/              # SQL init scripts (mounted into Postgres)
└── deploy/
    └── Dockerfile           # multi-stage: build → minimal image
```

For design rationale, API conventions, and bounded-context details, see `../docs/ARCHITECTURE.md` and `../docs/API_DESIGN.md`.
