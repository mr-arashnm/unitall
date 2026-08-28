# Unital — Target Architecture

> Redesign of the Django monolith (see [BUSINESS_DOCUMENT.md](BUSINESS_DOCUMENT.md)) into Go microservices. Patterns: Clean Architecture per service (domain / usecase / adapter ports), database-per-service, event-driven integration, API Gateway at the edge.

## 1. System Context

```
                          ┌─────────────────────────────┐
   Residents / Managers   │        API Gateway          │  :8080
   ── HTTPS ────────────► │  authN (JWT) · routing ·    │
                          │  rate limit · feature flags │
                          └──────┬──────┬──────┬────────┘
                                 │      │      │  internal REST (mTLS later)
              ┌──────────────────┴─┐ ┌──┴──────┴──────────┐
              │ identity   :9001   │ │ property   :9002   │
              │ billing    :9003   │ │ facilities :9004   │
              │ operations :9005   │ │ communications:9006│
              └─────────┬──────────┘ └─────────┬──────────┘
                        │     PostgreSQL per service │
                        └────────────►►►►─────────┘
                                     NATS / RabbitMQ  (domain events)
                                     Redis (cache, rate limits)
                                     MinIO/S3 (files)
```

**Initial deployment reality**: one `docker-compose.yml` runs everything (gateway, 6 services, Postgres schemas via separate DBs, NATS, Redis). Services are separate deployables from day one but co-located — microservice boundaries bought with events and DB isolation, not with Kubernetes yet.

## 2. Bounded Contexts & Service Ownership

| Service | Owns (data) | Publishes | Consumes |
|---|---|---|---|
| **identity** | users, sessions/refresh tokens, building **memberships** (user↔building + role), platform roles | `user.created`, `user.updated`, `membership.granted`, `membership.revoked` | — |
| **property** | buildings, features (flags), units, parkings, warehouses, transfer history, contracts | `building.created`, `unit.created`, `asset.assigned`, `ownership.changed`, `residency.changed`, `contract.activated` | `user.created` (denormalize names for audit) |
| **billing** | charge templates, charges, transactions, invoices, payments | `charges.generated`, `charge.overdue`, `payment.completed`, `invoice.settled` | `ownership.changed` (rebill target), `contract.activated` (rental invoicing) |
| **facilities** | facilities, usage rules, images, bookings, maintenance windows | `booking.confirmed`, `booking.cancelled`, `facility.maintenance_scheduled` | `payment.completed` (mark booking paid) |
| **operations** | teams, tasks, task comments, service requests, performance snapshots | `task.created`, `task.completed`, `servicerequest.completed` | `user.updated` (staff profile), `facility.maintenance_scheduled` (auto-task) |
| **communications** | announcements, meetings, attendance, minutes, tickets, responses, notification inbox, templates | `notification.sent` | events from **all** services → notifications & reminders |

### Key decisions (fixing monolith defects)

1. **One Team concept** (operations owns teams; communications references `team_id` only — kills the duplicate Django model).
2. **Membership as first-class** (identity): user→building with role and validity range. Replaces scattered `board_members` M2M and ad-hoc `user_type in [...]` checks with an explicit authorization check at the gateway (`/buildings/{id}/…` routes require membership of that building with sufficient role).
3. **Ownership/residency kept in property**, modeled with **validity ranges** (`from_date`, `to_date`) instead of mutable FKs — history is a projection, not an after-thought log table.
4. **Contracts owned by property**; activation is a use case that changes ownership/residency and emits `contract.activated` (billing listens).
5. **Feature flags enforced**: gateway checks `building.features` via property's internal API before forwarding module routes.
6. **Jalali periods**: billing period keys are Jalali `YYYY-MM`; all dates stored as ISO; `pkg/jalali` converts.

## 3. Service Internal Structure (Clean Architecture)

```
services/identity/
  cmd/server/main.go          # wiring only
  internal/domain/            # entities, value objects, domain errors, repo PORTS
  internal/usecase/           # application services; depends only on domain ports
  internal/adapter/
    http/                     # chi handlers, request/response DTOs, openapi glue
    postgres/                 # repo implementations, migrations
    events/                   # NATS publisher/subscriber
    mailer/                   # verification email adapter (SMTP interface)
  internal/config/            # env config
```

Rules: `domain` imports nothing external; `usecase` imports `domain` only; adapters implement domain ports; tests use in-memory ports (`adapters/…/fake` or `usecase` tests with fakes). HTTP handlers do parse→usecase→respond only.

## 4. Cross-Cutting

- **Auth**: JWT access (15m, RS256… start HS256 with shared secret in compose, RS256 listed as hardening step) + opaque refresh token (rotating, stored hashed in identity). Gateway validates signature + expiry; services trust `X-User-Id`, `X-User-Roles`, `X-Building-Role` headers set by the gateway (stripped from inbound).
- **Authorization model**: platform role (user type) + per-building membership role. Permission matrix:
  - `manager`: everything in their buildings.
  - `board_member`: everything except billing PSP confirmations & user management.
  - `staff`: teams/tasks/maintenance assigned to them (read) + update task status.
  - `owner`/`resident`: own units' data, bookings, requests, tickets, inbox.
- **Idempotency**: all POST mutations accept `Idempotency-Key` header; billing generation & payments require it.
- **Money**: integer minor units (`amount` in Rial, int64). No floats. Currency field reserved, default `IRR`.
- **Time**: UTC internally; Jalali rendering at edges.
- **Errors**: RFC 9457 Problem Details envelope, machine-readable `code` (e.g. `UNIT_NOT_IN_BUILDING`), correlation `trace_id`.
- **Events**: CloudEvents 1.0 JSON envelope on NATS JetStream, at-least-once, consumers idempotent by `event_id`.
- **Observability**: structured slog JSON, OTLP traces/metrics behind `OTEL_EXPORTER_OTLP_ENDPOINT` (optional in compose), `/healthz` & `/readyz` per service.
- **Migrations**: golang-migrate SQL files per service, run on boot (`migrate on start` flag).

## 5. Data Ownership (summary of tables per service DB)

- **identity_db**: `users`, `refresh_tokens`, `memberships` (user_id, building_id, role, from, to), `verification_tokens`.
- **property_db**: `buildings`, `building_features`, `units`, `parkings`, `warehouses`, `unit_parties` (ownership/residency with validity ranges), `transfer_history` (projection), `contracts`.
- **billing_db**: `charge_templates`, `charges`, `transactions`, `payments`, `invoices`, `invoice_lines`.
- **facilities_db**: `facilities`, `usage_rules`, `images`, `bookings`, `maintenance_windows`.
- **operations_db**: `teams`, `team_members`, `tasks`, `task_comments`, `service_requests`, `performance_snapshots`.
- **communications_db**: `announcements`, `meetings`, `attendance`, `minutes`, `tickets`, `ticket_responses`, `notification_inbox`, `notification_campaigns`.

No cross-DB foreign keys. Cross-service reads (e.g. billing needs unit owner) go through internal REST with a short-TTL Redis cache; names are denormalized into events where needed.

## 6. Sagas & Workflows

- **Contract activation (saga, orchestrated by property)**: validate draft → apply ownership/residency (local tx + history) → emit `contract.activated` → billing creates/updates billing target (eventual). Compensation not needed (single writer for ownership).
- **Charge generation (saga, billing)**: for each unit of building → create or skip (`unit+template+period` unique) — idempotent by key, safe to rerun.
- **Payment**: transaction created → PSP/manual confirm → `payment.completed` → charge/invoice updated locally → `facilities` marks booking paid if `ref_type=booking`.
- **Overdue sweeper** (cron in billing): due_date < today & status=pending → `overdue` + emit `charge.overdue` → communications fans out reminders per channel policy.

## 7. Security

- Argon2id password hashing; lockout after N failed attempts (identity).
- Gateway rate limits (token bucket per IP/user), body size caps.
- All files (avatars, contract PDFs, ticket attachments) via presigned S3/MinIO URLs — no direct proxy.
- Secrets via env in compose; production assumes secret manager (documented, not built now).
- Audit: every state-changing use case emits an audit row (who/what/when/prev→next) in the owning service.
